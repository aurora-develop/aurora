package chatgpt

import (
	"aurora/httpclient"
	"aurora/internal/tokens"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

type UploadedFile struct {
	ID            string `json:"id,omitempty"`
	FileID        string `json:"file_id,omitempty"`
	Object        string `json:"object,omitempty"`
	Bytes         int64  `json:"bytes,omitempty"`
	CreatedAt     int64  `json:"created_at,omitempty"`
	Filename      string `json:"filename,omitempty"`
	Purpose       string `json:"purpose,omitempty"`
	MimeType      string `json:"mime_type,omitempty"`
	LibraryFileID string `json:"library_file_id,omitempty"`
}

type uploadMetaResponse struct {
	FileID        string `json:"file_id"`
	UploadURL     string `json:"upload_url"`
	LibraryFileID string `json:"library_file_id"`
}

var uploadedFiles sync.Map

func RegisterUploadedFile(file UploadedFile) {
	if file.FileID == "" {
		file.FileID = file.ID
	}
	if file.ID == "" {
		file.ID = file.FileID
	}
	if file.FileID == "" {
		return
	}
	uploadedFiles.Store(file.FileID, file)
}

func LookupUploadedFile(fileID string) (UploadedFile, bool) {
	value, ok := uploadedFiles.Load(fileID)
	if !ok {
		return UploadedFile{}, false
	}
	file, ok := value.(UploadedFile)
	return file, ok
}

func UploadFile(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy, filename, contentType, purpose string, data []byte) (UploadedFile, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	if secret == nil || secret.Token == "" || secret.IsFree {
		return UploadedFile{}, http.StatusBadRequest, fmt.Errorf("file upload requires a logged-in ChatGPT access token")
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "upload.bin"
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if purpose == "" {
		purpose = "assistants"
	}

	meta, status, err := createUpload(client, secret, filename, len(data))
	if err != nil {
		return UploadedFile{}, status, err
	}
	if status, err := putUpload(client, meta.UploadURL, contentType, data); err != nil {
		return UploadedFile{}, status, err
	}
	if status, err := confirmUpload(client, secret, meta.FileID); err != nil {
		return UploadedFile{}, status, err
	}

	result := UploadedFile{
		ID:            meta.FileID,
		FileID:        meta.FileID,
		Object:        "file",
		Bytes:         int64(len(data)),
		Filename:      filename,
		Purpose:       purpose,
		MimeType:      contentType,
		LibraryFileID: meta.LibraryFileID,
	}
	RegisterUploadedFile(result)
	return result, http.StatusOK, nil
}

func createUpload(client httpclient.AuroraHttpClient, secret *tokens.Secret, filename string, size int) (uploadMetaResponse, int, error) {
	payload := map[string]interface{}{
		"file_name":                filename,
		"file_size":                size,
		"use_case":                 "multimodal",
		"timezone_offset_min":      -480,
		"reset_rate_limits":        false,
		"store_in_library":         true,
		"library_persistence_mode": "opportunistic",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return uploadMetaResponse{}, http.StatusInternalServerError, err
	}
	header := createBaseHeader()
	header.Set("accept", "application/json")
	header.Set("content-type", "application/json")
	if secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	setTeamAccountHeader(header, secret)
	response, err := client.Request(http.MethodPost, BaseURL+"/files", header, nil, bytes.NewReader(body))
	if err != nil {
		return uploadMetaResponse{}, http.StatusInternalServerError, err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return uploadMetaResponse{}, response.StatusCode, fmt.Errorf("create file upload failed: %s", string(responseBody))
	}
	var meta uploadMetaResponse
	if err := json.Unmarshal(responseBody, &meta); err != nil {
		return uploadMetaResponse{}, response.StatusCode, err
	}
	if meta.FileID == "" || meta.UploadURL == "" {
		return uploadMetaResponse{}, response.StatusCode, fmt.Errorf("invalid file upload response: %s", string(responseBody))
	}
	return meta, response.StatusCode, nil
}

func putUpload(client httpclient.AuroraHttpClient, uploadURL, contentType string, data []byte) (int, error) {
	header := make(httpclient.AuroraHeaders)
	header.Set("Content-Type", contentType)
	header.Set("x-ms-blob-type", "BlockBlob")
	header.Set("x-ms-version", "2020-04-08")
	header.Set("Origin", "https://chatgpt.com")
	header.Set("Referer", "https://chatgpt.com/")
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "application/json, text/plain, */*")
	header.Set("Accept-Language", "en-US,en;q=0.8")
	response, err := client.Request(http.MethodPut, uploadURL, header, nil, bytes.NewReader(data))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, fmt.Errorf("upload file data failed: %s", string(responseBody))
	}
	return response.StatusCode, nil
}

func confirmUpload(client httpclient.AuroraHttpClient, secret *tokens.Secret, fileID string) (int, error) {
	header := createBaseHeader()
	header.Set("accept", "application/json")
	header.Set("content-type", "application/json")
	if secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	setTeamAccountHeader(header, secret)
	response, err := client.Request(http.MethodPost, BaseURL+"/files/"+fileID+"/uploaded", header, nil, strings.NewReader("{}"))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, fmt.Errorf("confirm file upload failed: %s", string(responseBody))
	}
	return response.StatusCode, nil
}

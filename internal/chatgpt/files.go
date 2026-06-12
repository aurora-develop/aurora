package chatgpt

import (
	"aurora/httpclient"
	"aurora/internal/tokens"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
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
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
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

// UploadFile 执行完整三步上传,对齐 chatgpttoapi/files.go UploadFile。
func UploadFile(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy, filename, mimeHint string, data []byte) (UploadedFile, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	if secret == nil || secret.Token == "" || secret.IsFree {
		return UploadedFile{}, http.StatusBadRequest, fmt.Errorf("file upload requires a logged-in ChatGPT access token")
	}
	if len(data) == 0 {
		return UploadedFile{}, http.StatusBadRequest, fmt.Errorf("empty file data")
	}

	mime, ext := resolveMime(data, mimeHint)
	useCase := "multimodal"
	if !strings.HasPrefix(mime, "image/") {
		useCase = "my_files"
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = fmt.Sprintf("file-%d%s", len(data), ext)
	}

	var width, height int
	if strings.HasPrefix(mime, "image/") {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			width = cfg.Width
			height = cfg.Height
		}
	}

	// ---- Step 1: POST /backend-api/files ----
	step1Payload := map[string]interface{}{
		"file_name":  filename,
		"file_size":  len(data),
		"use_case":   useCase,
		"mime_type":  mime,
		"store_in_library":         true,
		"library_persistence_mode": "opportunistic",
	}
	if width > 0 && height > 0 {
		step1Payload["width"] = width
		step1Payload["height"] = height
	}

	meta, status, err := createUpload(client, secret, step1Payload)
	if err != nil {
		return UploadedFile{}, status, err
	}

	// ---- Step 2: PUT upload_url (Azure Blob) ----
	if status, err := putUpload(client, meta.UploadURL, mime, data); err != nil {
		return UploadedFile{}, status, err
	}

	// ---- Step 3: POST /backend-api/files/{file_id}/uploaded ----
	if status, err := confirmUpload(client, secret, meta.FileID); err != nil {
		return UploadedFile{}, status, err
	}

	result := UploadedFile{
		ID:            meta.FileID,
		FileID:        meta.FileID,
		Object:        "file",
		Bytes:         int64(len(data)),
		Filename:      filename,
		MimeType:      mime,
		Width:         width,
		Height:        height,
		LibraryFileID: meta.LibraryFileID,
	}
	RegisterUploadedFile(result)
	return result, http.StatusOK, nil
}

func createUpload(client httpclient.AuroraHttpClient, secret *tokens.Secret, payload map[string]interface{}) (uploadMetaResponse, int, error) {
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
	header.Set("User-Agent", defaultUserAgent())
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

// ── MIME 检测(对齐 chatgpttoapi/files.go) ──

func resolveMime(data []byte, mimeHint string) (mime, ext string) {
	sniffed, sniffExt := sniffMime(data)
	hint := normalizeMime(mimeHint)
	if hint != "" && hint != "application/octet-stream" {
		ext = extFromMime(hint)
		if ext == "" {
			ext = sniffExt
		}
		return hint, ext
	}
	return sniffed, sniffExt
}

func normalizeMime(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ";"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func sniffMime(data []byte) (mime, ext string) {
	n := 512
	if len(data) < n {
		n = len(data)
	}
	mime = http.DetectContentType(data[:n])
	ext = extFromMime(mime)
	return mime, ext
}

func extFromMime(m string) string {
	switch strings.ToLower(normalizeMime(m)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	case "text/csv":
		return ".csv"
	case "application/json":
		return ".json"
	case "text/markdown":
		return ".md"
	default:
		return ""
	}
}

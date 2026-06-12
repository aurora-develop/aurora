package chatgpt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	"aurora/httpclient"
	"aurora/internal/tokens"
)

func materializeArtifactEvents(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID string, events []StreamEvent, cfg ArtifactStreamConfig) []map[string]interface{} {
	cfg = cfg.normalized()
	var out []map[string]interface{}
	for _, ev := range events {
		out = append(out, materializeArtifactEvent(client, secret, conversationID, ev, cfg)...)
	}
	return out
}

func materializeArtifactEvent(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID string, ev StreamEvent, cfg ArtifactStreamConfig) []map[string]interface{} {
	if ev.Event != StreamEventArtifact {
		return []map[string]interface{}{streamEventToMap(ev)}
	}
	switch ev.Kind {
	case "generated_image":
		return materializeGeneratedImageEvent(client, secret, conversationID, ev, cfg)
	case "sandbox_file", "pdf":
		return materializeSandboxEvent(client, secret, conversationID, ev, cfg)
	default:
		return []map[string]interface{}{streamEventToMap(ev)}
	}
}

func materializeGeneratedImageEvent(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID string, ev StreamEvent, cfg ArtifactStreamConfig) []map[string]interface{} {
	if ev.FileID == "" {
		return []map[string]interface{}{streamEventToMap(ev)}
	}
	if cfg.Delivery == ArtifactDeliveryURL {
		if url, err := ResolveGeneratedImageURL(client, secret, conversationID, ev.FileID); err == nil {
			ev.URL = url
		} else {
			ev.Error = err.Error()
		}
		return []map[string]interface{}{streamEventToMap(ev)}
	}
	url, err := ResolveGeneratedImageURL(client, secret, conversationID, ev.FileID)
	if err != nil {
		ev.Error = err.Error()
		return []map[string]interface{}{streamEventToMap(ev)}
	}
	data, err := DownloadURLBytes(client, url, secret, "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if err != nil {
		ev.Error = err.Error()
		return []map[string]interface{}{streamEventToMap(ev)}
	}
	ev.URL = url
	if ev.MimeType == "" {
		ev.MimeType = "image/png"
	}
	return materializeBytes(ev, data, cfg)
}

func materializeSandboxEvent(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID string, ev StreamEvent, cfg ArtifactStreamConfig) []map[string]interface{} {
	if conversationID == "" || ev.MessageID == "" || ev.SandboxPath == "" {
		ev.Error = "sandbox artifact requires conversation_id, message_id and sandbox_path"
		return []map[string]interface{}{streamEventToMap(ev)}
	}
	if ev.Name == "" {
		ev.Name = path.Base(ev.SandboxPath)
	}
	if ev.MimeType == "" {
		ev.MimeType = mimeTypeForName(ev.Name)
	}
	if cfg.Delivery == ArtifactDeliveryURL {
		if url, err := ResolveSandboxDownloadURL(client, secret, conversationID, ev.MessageID, ev.SandboxPath); err == nil {
			ev.URL = url
		} else {
			ev.Error = err.Error()
		}
		return []map[string]interface{}{streamEventToMap(ev)}
	}
	data, mimeType, err := DownloadSandboxFile(client, secret, conversationID, ev.MessageID, ev.SandboxPath)
	if err != nil {
		ev.Error = err.Error()
		return []map[string]interface{}{streamEventToMap(ev)}
	}
	if mimeType != "" {
		ev.MimeType = mimeType
	}
	return materializeBytes(ev, data, cfg)
}

func materializeBytes(ev StreamEvent, data []byte, cfg ArtifactStreamConfig) []map[string]interface{} {
	ev.SizeBytes = len(data)
	if cfg.Delivery == ArtifactDeliveryBase64 {
		ev.Data = base64.StdEncoding.EncodeToString(data)
		return []map[string]interface{}{
			streamEventToMap(ev),
			streamEventToMap(StreamEvent{
				Event:       StreamEventArtifactDone,
				Kind:        ev.Kind,
				Index:       ev.Index,
				SlotIndex:   ev.SlotIndex,
				Revision:    ev.Revision,
				FileID:      ev.FileID,
				MessageID:   ev.MessageID,
				SandboxPath: ev.SandboxPath,
				SizeBytes:   len(data),
			}),
		}
	}

	meta := ev
	meta.Data = ""
	events := []map[string]interface{}{streamEventToMap(meta)}
	chunkSize := cfg.ChunkSize
	total := (len(data) + chunkSize - 1) / chunkSize
	if total == 0 {
		total = 1
	}
	for i := 0; i < total; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := StreamEvent{
			Event:       StreamEventArtifactChunk,
			Kind:        ev.Kind,
			Index:       ev.Index,
			SlotIndex:   ev.SlotIndex,
			Revision:    ev.Revision,
			FileID:      ev.FileID,
			MessageID:   ev.MessageID,
			SandboxPath: ev.SandboxPath,
			ChunkIndex:  i + 1,
			ChunkTotal:  total,
			Data:        base64.StdEncoding.EncodeToString(data[start:end]),
		}
		events = append(events, streamEventToMap(chunk))
	}
	events = append(events, streamEventToMap(StreamEvent{
		Event:       StreamEventArtifactDone,
		Kind:        ev.Kind,
		Index:       ev.Index,
		SlotIndex:   ev.SlotIndex,
		Revision:    ev.Revision,
		FileID:      ev.FileID,
		MessageID:   ev.MessageID,
		SandboxPath: ev.SandboxPath,
		SizeBytes:   len(data),
	}))
	return events
}

func ResolveGeneratedImageURL(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID, fileID string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("artifact download client is unavailable")
	}
	if fileID == "" {
		return "", fmt.Errorf("missing file_id")
	}
	return GetImageDownloadURL(client, fileDownloadBaseURL()+fileID+"/download", secret)
}

func ResolveSandboxDownloadURL(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID, messageID, sandboxPath string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("artifact download client is unavailable")
	}
	if conversationID == "" || messageID == "" || sandboxPath == "" {
		return "", fmt.Errorf("missing sandbox download identifiers")
	}
	apiURL, targetPath := conversationURL(secret, "/conversation/"+conversationID+"/interpreter/download")
	bodyJSON, err := json.Marshal(map[string]interface{}{
		"message_id":   messageID,
		"sandbox_path": sandboxPath,
	})
	if err != nil {
		return "", err
	}
	header := conversationHeaders(secret, nil, "*/*", targetPath, "", "")
	response, err := client.Request(http.MethodPost, apiURL, header, nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("interpreter/download failed: %s", string(body))
	}
	var result struct {
		DownloadURL string `json:"download_url"`
		URL         string `json:"url"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.DownloadURL != "" {
		return result.DownloadURL, nil
	}
	if result.URL != "" {
		return result.URL, nil
	}
	return "", fmt.Errorf("interpreter/download missing download_url")
}

func DownloadSandboxFile(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID, messageID, sandboxPath string) ([]byte, string, error) {
	downloadURL, err := ResolveSandboxDownloadURL(client, secret, conversationID, messageID, sandboxPath)
	if err != nil {
		return nil, "", err
	}
	data, err := DownloadURLBytes(client, downloadURL, secret, "*/*")
	if err != nil {
		return nil, "", err
	}
	mimeType := mimeTypeForName(path.Base(sandboxPath))
	if mimeType == "" && len(data) > 0 {
		mimeType = http.DetectContentType(data)
	}
	return data, mimeType, nil
}

func DownloadURLBytes(client httpclient.AuroraHttpClient, url string, secret *tokens.Secret, accept string) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("download client is unavailable")
	}
	header := make(httpclient.AuroraHeaders)
	if secret != nil && secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	header.Set("User-Agent", defaultUserAgent())
	if accept == "" {
		accept = "*/*"
	}
	header.Set("Accept", accept)
	if secret != nil && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	setTeamAccountHeader(header, secret)
	response, err := client.Request(http.MethodGet, url, header, nil, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s", string(body))
	}
	return body, nil
}

func mimeTypeForName(name string) string {
	if name == "" {
		return ""
	}
	if mimeType := mime.TypeByExtension(path.Ext(name)); mimeType != "" {
		return mimeType
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return "text/csv"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".md"):
		return "text/markdown"
	default:
		return ""
	}
}

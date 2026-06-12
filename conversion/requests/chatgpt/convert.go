package chatgpt

import (
	backendchatgpt "aurora/internal/chatgpt"
	"aurora/internal/tokens"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
	"aurora/httpclient"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
)

func ConvertAPIRequest(api_request official_types.APIRequest, secret *tokens.Secret, proxy string, client httpclient.AuroraHttpClient) chatgpt_types.ChatGPTRequest {
	chatgpt_request := chatgpt_types.NewChatGPTRequest()

	// Model is passed directly to upstream; default to "auto" if not provided
	model := api_request.Model
	if model == "" {
		model = "auto"
	}
	chatgpt_request.Model = model

	for _, apiMessage := range api_request.Messages {
		if apiMessage.Role == "system" {
			apiMessage.Role = "critic"
		}
		parts, metadata := buildMessageParts(apiMessage, client, secret, proxy)
		if len(metadata) > 0 {
			chatgpt_request.AddMultimodalMessage(apiMessage.Role, parts, metadata)
			continue
		}
		chatgpt_request.AddMessage(apiMessage.Role, apiMessage.Text())
	}
	return chatgpt_request
}

func ConvertTTSAPIRequest(input string) chatgpt_types.ChatGPTRequest {
	chatgpt_request := chatgpt_types.NewChatGPTRequest()
	chatgpt_request.HistoryAndTrainingDisabled = false
	chatgpt_request.AddAssistantMessage(input)
	return chatgpt_request
}

func buildMessageParts(message official_types.APIMessage, client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string) ([]interface{}, map[string]interface{}) {
	text := message.Text()
	files := enrichFiles(message.Files(), client, secret, proxy)
	if len(files) == 0 {
		return []interface{}{text}, nil
	}

	parts := make([]interface{}, 0, len(files)+1)
	attachments := make([]interface{}, 0, len(files))
	for _, file := range files {
		fileID := fileID(file)
		if fileID == "" {
			continue
		}
		if isImageFile(file) {
			part := map[string]interface{}{
				"content_type":  "image_asset_pointer",
				"asset_pointer": "file-service://" + fileID,
			}
			if file.Size > 0 {
				part["size_bytes"] = file.Size
			}
			if file.Width > 0 {
				part["width"] = file.Width
			}
			if file.Height > 0 {
				part["height"] = file.Height
			}
			parts = append(parts, part)
		}

		attachment := map[string]interface{}{
			"id":           fileID,
			"size":         file.Size,
			"name":         fileName(file),
			"mime_type":    fileMime(file),
			"mimeType":     fileMime(file),
			"source":       "library",
			"is_big_paste": false,
		}
		if file.Width > 0 {
			attachment["width"] = file.Width
		}
		if file.Height > 0 {
			attachment["height"] = file.Height
		}
		if file.LibraryFileID != "" {
			attachment["library_file_id"] = file.LibraryFileID
		}
		attachments = append(attachments, attachment)
	}
	if text != "" {
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		parts = append(parts, text)
	}
	return parts, map[string]interface{}{
		"attachments":                  attachments,
		"developer_mode_connector_ids": []interface{}{},
		"selected_sources":             []interface{}{},
		"selected_github_repos":        []interface{}{},
		"selected_all_github_repos":    false,
		"serialization_metadata":       map[string]interface{}{"custom_symbol_offsets": []interface{}{}},
	}
}

func enrichFiles(files []official_types.FileAttachment, client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string) []official_types.FileAttachment {
	enriched := make([]official_types.FileAttachment, 0, len(files))
	seen := make(map[string]bool)
	for _, file := range files {
		id := fileID(file)
		if id == "" || seen[id] {
			continue
		}

		// 处理 image_url 的 inline 数据(data: URL 或 http URL)
		if file.Source != "" && client != nil && secret != nil {
			if uploaded, ok := uploadInlineImage(file, client, secret, proxy); ok {
				file = uploaded
			} else {
				// 免费账号或上传失败:丢弃图片,只保留文本
				continue
			}
		}

		if uploaded, ok := backendchatgpt.LookupUploadedFile(fileID(file)); ok {
			if file.ID == "" {
				file.ID = uploaded.ID
			}
			if file.FileID == "" {
				file.FileID = uploaded.FileID
			}
			if file.Name == "" && file.FileName == "" && file.Filename == "" {
				file.Name = uploaded.Filename
				file.FileName = uploaded.Filename
				file.Filename = uploaded.Filename
			}
			if file.MimeType == "" && file.MIMEType == "" {
				file.MimeType = uploaded.MimeType
				file.MIMEType = uploaded.MimeType
			}
			if file.Size == 0 {
				file.Size = uploaded.Bytes
			}
			if file.LibraryFileID == "" {
				file.LibraryFileID = uploaded.LibraryFileID
			}
		}
		seen[fileID(file)] = true
		enriched = append(enriched, file)
	}
	return enriched
}

// uploadInlineImage 将 data: URL 或 http URL 图片上传到 ChatGPT 文件服务。
func uploadInlineImage(file official_types.FileAttachment, client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string) (official_types.FileAttachment, bool) {
	src := file.Source
	var data []byte
	var filename string
	var contentType string

	if strings.HasPrefix(src, "data:") {
		// data:image/png;base64,iVBOR...
		commaIdx := strings.Index(src, ",")
		if commaIdx < 0 {
			return file, false
		}
		meta := src[:commaIdx]
		b64data := src[commaIdx+1:]
		// 提取 mime type
		if semiIdx := strings.Index(meta, ";"); semiIdx > 5 {
			contentType = meta[5:semiIdx]
		}
		var err error
		data, err = base64.StdEncoding.DecodeString(b64data)
		if err != nil {
			// 尝试 raw base64
			data, err = base64.RawStdEncoding.DecodeString(b64data)
			if err != nil {
				return file, false
			}
		}
		filename = "image.png"
		if contentType == "" {
			contentType = "image/png"
		}
	} else if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		// 下载远程图片
		resp, err := http.Get(src)
		if err != nil {
			return file, false
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return file, false
		}
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return file, false
		}
		contentType = resp.Header.Get("Content-Type")
		filename = guessFilenameFromURL(src)
	} else {
		return file, false
	}

	if len(data) == 0 {
		return file, false
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if filename == "" {
		filename = "image.png"
	}

	uploaded, _, err := backendchatgpt.UploadFile(client, secret, proxy, filename, contentType, data)
	if err != nil {
		// 免费 token 无法上传文件,回退:把 data URL 原样传递
		return file, false
	}

	return official_types.FileAttachment{
		ID:            uploaded.FileID,
		FileID:        uploaded.FileID,
		Name:          uploaded.Filename,
		FileName:      uploaded.Filename,
		Filename:      uploaded.Filename,
		MimeType:      uploaded.MimeType,
		MIMEType:      uploaded.MimeType,
		Size:          uploaded.Bytes,
		Width:         uploaded.Width,
		Height:        uploaded.Height,
		LibraryFileID: uploaded.LibraryFileID,
	}, true
}

func guessFilenameFromURL(url string) string {
	idx := strings.LastIndex(url, "/")
	if idx >= 0 && idx < len(url)-1 {
		name := url[idx+1:]
		if q := strings.Index(name, "?"); q >= 0 {
			name = name[:q]
		}
		if name != "" {
			return name
		}
	}
	return "image.png"
}

func fileID(file official_types.FileAttachment) string {
	if strings.TrimSpace(file.FileID) != "" {
		return strings.TrimSpace(file.FileID)
	}
	return strings.TrimSpace(file.ID)
}

func fileName(file official_types.FileAttachment) string {
	for _, value := range []string{file.Name, file.FileName, file.Filename} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return fileID(file)
}

func fileMime(file official_types.FileAttachment) string {
	if strings.TrimSpace(file.MimeType) != "" {
		return strings.TrimSpace(file.MimeType)
	}
	return strings.TrimSpace(file.MIMEType)
}

func isImageFile(file official_types.FileAttachment) bool {
	if strings.HasPrefix(strings.ToLower(fileMime(file)), "image/") {
		return true
	}
	name := strings.ToLower(fileName(file))
	return strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".webp") || strings.HasSuffix(name, ".gif")
}

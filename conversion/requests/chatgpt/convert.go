package chatgpt

import (
	backendchatgpt "aurora/internal/chatgpt"
	"aurora/internal/tokens"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
	"strings"
)

func ConvertAPIRequest(api_request official_types.APIRequest, secret *tokens.Secret, proxy string) chatgpt_types.ChatGPTRequest {
	chatgpt_request := chatgpt_types.NewChatGPTRequest()

	// Model is passed directly to upstream; default to "auto" if not provided
	model := api_request.Model
	if model == "" {
		model = "auto"
	}
	chatgpt_request.Model = model

	if api_request.PluginIDs != nil {
		chatgpt_request.PluginIDs = api_request.PluginIDs
		chatgpt_request.Model = "gpt-4-plugins"
	}
	for _, apiMessage := range api_request.Messages {
		if apiMessage.Role == "system" {
			apiMessage.Role = "critic"
		}
		parts, metadata := buildMessageParts(apiMessage)
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

func buildMessageParts(message official_types.APIMessage) ([]interface{}, map[string]interface{}) {
	text := message.Text()
	files := enrichFiles(message.Files())
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

func enrichFiles(files []official_types.FileAttachment) []official_types.FileAttachment {
	enriched := make([]official_types.FileAttachment, 0, len(files))
	seen := make(map[string]bool)
	for _, file := range files {
		id := fileID(file)
		if id == "" || seen[id] {
			continue
		}
		if uploaded, ok := backendchatgpt.LookupUploadedFile(id); ok {
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
		seen[id] = true
		enriched = append(enriched, file)
	}
	return enriched
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

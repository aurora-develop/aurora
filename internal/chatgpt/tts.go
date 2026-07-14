package chatgpt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"aurora/httpclient"
	"aurora/internal/accounts"
	"aurora/internal/sseparser"
)

// HandlerTTS 处理 TTS 响应的 SSE 流，返回 (message_id, conversation_id)。
func HandlerTTS(response *http.Response, input string) (string, string) {
	reader := bufio.NewReader(response.Body)

	var convId string
	var fallbackMsgID string
	var patchState sseparser.PatchState

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				break
			}
			if err != io.EOF {
				return "", ""
			}
		}
		for _, payload := range sseparser.DataPayloads(line) {
			if strings.HasPrefix(payload, "[DONE]") {
				break
			}
			streamEvent, ok := parseConversationEvent(payload, &patchState, "auto")
			if !ok {
				var raw map[string]interface{}
				if json.Unmarshal([]byte(payload), &raw) == nil {
					if cid := firstConversationID(raw); cid != "" && convId == "" {
						convId = cid
					}
					if msgID := lastAssistantMessageID(raw); msgID != "" && fallbackMsgID == "" {
						fallbackMsgID = msgID
					}
				}
				continue
			}
			if streamEvent.response.Error != nil {
				return "", ""
			}
			originalResponse := streamEvent.response
			if streamEvent.conversationID != "" && convId == "" {
				convId = streamEvent.conversationID
			}
			if originalResponse.ConversationID != convId {
				if convId == "" {
					convId = originalResponse.ConversationID
				} else {
					continue
				}
			}
			if originalResponse.Message.ID == "" {
				continue
			}
			if originalResponse.Message.Author.Role != "assistant" {
				continue
			}

			// Newer upstream responses are not always an exact single-part echo of the
			// requested TTS input. Prefer an exact match, then fall back to the first
			// assistant message in the same conversation so synthesize still works.
			if fallbackMsgID == "" {
				fallbackMsgID = originalResponse.Message.ID
			}
			if len(originalResponse.Message.Content.Parts) == 0 {
				continue
			}
			for _, rawPart := range originalResponse.Message.Content.Parts {
				part, ok := rawPart.(string)
				if !ok {
					continue
				}
				if part == input || strings.Contains(part, input) || strings.Contains(input, part) {
					return originalResponse.Message.ID, convId
				}
			}
		}
		if err == io.EOF {
			break
		}
	}
	if fallbackMsgID != "" && convId != "" {
		return fallbackMsgID, convId
	}
	return "", ""
}

func getTTSBlobFromURL(client httpclient.AuroraHttpClient, account *accounts.Account, reqURL string) ([]byte, int, error) {
	header := createBaseHeader()
	header.Set("Accept", "audio/*,*/*")
	if !(account.Type == accounts.TypeNoAuth) && account.Token != "" {
		header.Set("Authorization", "Bearer "+account.Token)
	}
	if account.Type == accounts.TypeNoAuth {
		header.Set("Oai-Device-Id", account.Token)
	}
	if account.PUID != "" {
		header.Set("Cookie", "_puid="+account.PUID+";")
	}
	setTeamAccountHeader(header, account)
	response, err := client.Request(http.MethodGet, reqURL, header, nil, nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	defer response.Body.Close()
	blob, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("tts download failed: %s", string(blob))
	}
	return blob, response.StatusCode, nil
}

func parseTTSDownloadURL(blob []byte) string {
	var info fileInfo
	if err := json.Unmarshal(blob, &info); err != nil {
		return ""
	}
	if info.DownloadURL != "" {
		return info.DownloadURL
	}
	return info.URL
}

// GetTTS 获取 TTS 音频数据。
func GetTTS(client httpclient.AuroraHttpClient, account *accounts.Account, msgId, convId, voice, format, proxy string) ([]byte, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	params := url.Values{}
	params.Set("message_id", msgId)
	params.Set("conversation_id", convId)
	params.Set("voice", voice)
	params.Set("format", format)
	var reqUrl string
	if account.Type == accounts.TypeNoAuth {
		reqUrl = strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + "/synthesize?" + params.Encode()
	} else {
		reqUrl = BaseURL + "/synthesize?" + params.Encode()
	}

	blob, status, err := getTTSBlobFromURL(client, account, reqUrl)
	if err == nil {
		if downloadURL := parseTTSDownloadURL(blob); downloadURL != "" {
			return getTTSBlobFromURL(client, account, downloadURL)
		}
		return blob, status, nil
	}

	// Some upstream variants now return a signed file URL payload or fail on the
	// first synthesize URL shape. If the error body still contains a download URL,
	// honor it before surfacing the failure.
	if downloadURL := parseTTSDownloadURL(blob); downloadURL != "" {
		return getTTSBlobFromURL(client, account, downloadURL)
	}
	return nil, status, err
}

// RemoveConversation 隐藏（删除）对话。
func RemoveConversation(client httpclient.AuroraHttpClient, account *accounts.Account, id string, proxy string) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	var url string
	if account.Type == accounts.TypeNoAuth {
		url = strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + "/conversation/" + id
	} else {
		url = BaseURL + "/conversation/" + id
	}
	header := createBaseHeader()
	header.Set("Content-Type", "application/json")
	if !(account.Type == accounts.TypeNoAuth) && account.Token != "" {
		header.Set("Authorization", "Bearer "+account.Token)
	}
	if account.Type == accounts.TypeNoAuth {
		header.Set("Oai-Device-Id", account.Token)
	}
	if account.PUID != "" {
		header.Set("Cookie", "_puid="+account.PUID+";")
	}
	setTeamAccountHeader(header, account)
	payload := bytes.NewBuffer([]byte(`{"is_visible":false}`))
	response, err := client.Request(http.MethodPatch, url, header, nil, payload)
	if err != nil {
		return
	}
	response.Body.Close()
}

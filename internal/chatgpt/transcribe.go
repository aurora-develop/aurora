package chatgpt

import (
	"aurora/httpclient"
	"aurora/internal/accounts"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// TranscribeResult 是 ChatGPT /backend-api/transcribe 的响应。
// 上游通常返回纯文本,也可能返回 JSON 包裹。
type TranscribeResult struct {
	Text string `json:"text,omitempty"`
}

// TranscribeAudio 调用 ChatGPT 内部 /backend-api/transcribe 接口完成语音转文字。
// 参数:
//   - client:   HTTP 客户端
//   - account:   登录态(需要 paid token,免费 token 不支持)
//   - proxy:    代理地址
//   - audioData:  音频文件原始字节
//   - filename:   文件名(如 "audio.mp3")
//   - mimeType:   文件 Content-Type(如 "audio/mpeg")
//   - language:   语言提示(ISO 代码,如 "zh","en",可传 "")
//
// 返回转写后的文本、HTTP 状态码和错误。
func TranscribeAudio(client httpclient.AuroraHttpClient, account *accounts.Account, proxy string, audioData []byte, filename, mimeType, language string) (string, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	if account == nil || account.Token == "" || account.Type == accounts.TypeNoAuth {
		return "", http.StatusBadRequest, fmt.Errorf("audio transcription requires a logged-in ChatGPT access token")
	}
	if len(audioData) == 0 {
		return "", http.StatusBadRequest, fmt.Errorf("empty audio data")
	}

	// 构建 multipart body
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// file 字段
	partWriter, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("create form file: %w", err)
	}
	if _, err := partWriter.Write(audioData); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("write audio data: %w", err)
	}

	// language 字段(可选)
	if lang := strings.TrimSpace(language); lang != "" {
		if err := w.WriteField("language", lang); err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("write language field: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("close multipart writer: %w", err)
	}

	// 请求头
	header := baseHeaderFromAccount(account)
	header.Set("Accept", "application/json")
	header.Set("Content-Type", w.FormDataContentType())
	if account.Token != "" {
		header.Set("Authorization", "Bearer "+account.Token)
	}
	if account.PUID != "" {
		header.Set("Cookie", "_puid="+account.PUID+";")
	}
	setTeamAccountHeader(header, account)

	// oai-did cookie (对齐 ChatGPT Web 客户端,使用账号绑定指纹的 deviceID)
	didCookie := &http.Cookie{
		Name:  "oai-did",
		Value: account.Fingerprint.OaiDeviceID,
		Path:  "/",
	}
	// 合并 BasicCookies + oai-did
	cookies := append([]*http.Cookie{}, BasicCookies...)
	cookies = append(cookies, didCookie)

	// 发送请求
	requestURL := BaseURL + "/transcribe"
	response, err := client.Request(http.MethodPost, requestURL, header, cookies, &buf)
	if err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("transcribe request failed: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("read transcribe response: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", response.StatusCode, fmt.Errorf("transcribe failed (HTTP %d): %s", response.StatusCode, string(body))
	}

	// 解析响应:上游可能返回纯文本或 JSON
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", http.StatusOK, nil
	}

	// 尝试解析为 JSON
	var result TranscribeResult
	if err := json.Unmarshal(body, &result); err == nil && result.Text != "" {
		return result.Text, http.StatusOK, nil
	}

	// 移除可能存在的引号包裹
	text = strings.Trim(text, `"'`)
	return text, http.StatusOK, nil
}

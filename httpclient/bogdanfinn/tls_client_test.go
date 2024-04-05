package bogdanfinn

import (
	"aurora/httpclient"
	chatgpt_types "aurora/typings/chatgpt"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestTlsClient_Request(t *testing.T) {
	client := NewStdClient()
	userAgent := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	proxy := "http://127.0.0.1:7990"
	client.SetProxy(proxy)

	apiUrl := "https://chat.openai.com/backend-anon/sentinel/chat-requirements"
	payload := strings.NewReader(`{"conversation_mode_kind":"primary_assistant"}`)
	header := make(httpclient.AuroraHeaders)
	header.Set("Content-Type", "application/json")
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "*/*")
	header.Set("oai-language", "en-US")
	header.Set("origin", "https://chat.openai.com")
	header.Set("referer", "https://chat.openai.com/")
	header.Set("oai-device-id", "c83b24f0-5a9e-4c43-8915-3f67d4332609")
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, payload)
	if err != nil {
		return
	}
	if response.StatusCode != 200 {
		fmt.Println("Error: ", response.StatusCode)
	}
	var result chatgpt_types.RequirementsResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return
	}
	fmt.Println(result)
}

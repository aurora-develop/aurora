package bogdanfinn

import (
	"aurora/httpclient"
	chatgpt_types "aurora/typings/chatgpt"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

var BaseURL string

func init() {
	_ = godotenv.Load(".env")
	BaseURL = os.Getenv("BASE_URL")
	if BaseURL == "" {
		BaseURL = "https://chatgpt.com/backend-anon"
	}
}
func TestTlsClient_Request(t *testing.T) {
	client := NewStdClient()
	userAgent := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	proxy := "http://127.0.0.1:7990"
	client.SetProxy(proxy)

	apiUrl := BaseURL + "/sentinel/chat-requirements"
	payload := strings.NewReader(`{"conversation_mode_kind":"primary_assistant"}`)
	header := make(httpclient.AuroraHeaders)
	header.Set("Content-Type", "application/json")
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "*/*")
	header.Set("oai-language", "en-US")
	header.Set("origin", "https://chatgpt.com")
	header.Set("referer", "https://chatgpt.com/")
	header.Set("oai-device-id", "c83b24f0-5a9e-4c43-8915-3f67d4332609")
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, payload)
	if err != nil {
		return
	}
	defer response.Body.Close()
	fmt.Println(response.StatusCode)
	if response.StatusCode != 200 {
		fmt.Println("Error: ", response.StatusCode)
	}
	var result chatgpt_types.RequirementsResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return
	}
	fmt.Println(result.Token)
}

func TestChatGPTModel(t *testing.T) {
	client := NewStdClient()
	userAgent := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	apiUrl := "https://chatgpt.com/backend-anon/models"

	header := make(httpclient.AuroraHeaders)
	header.Set("Content-Type", "application/json")
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "*/*")
	header.Set("oai-language", "en-US")
	header.Set("origin", "https://chatgpt.com")
	header.Set("referer", "https://chatgpt.com/")
	header.Set("oai-device-id", "c83b24f0-5a9e-4c43-8915-3f67d4332609")
	response, err := client.Request(http.MethodGet, apiUrl, header, nil, nil)
	if err != nil {
		return
	}
	defer response.Body.Close()
	fmt.Println(response.StatusCode)
	if response.StatusCode != 200 {
		fmt.Println("Error: ", response.StatusCode)
		body, _ := io.ReadAll(response.Body)
		fmt.Println(string(body))
		return
	}

	type EnginesData struct {
		Models []struct {
			Slug         string   `json:"slug"`
			MaxTokens    int      `json:"max_tokens"`
			Title        string   `json:"title"`
			Description  string   `json:"description"`
			Tags         []string `json:"tags"`
			Capabilities struct {
			} `json:"capabilities,omitempty"`
			ProductFeatures struct {
			} `json:"product_features,omitempty"`
		} `json:"models"`
		Categories []struct {
			Category             string `json:"category"`
			HumanCategoryName    string `json:"human_category_name"`
			SubscriptionLevel    string `json:"subscription_level"`
			DefaultModel         string `json:"default_model"`
			CodeInterpreterModel string `json:"code_interpreter_model,omitempty"`
			PluginsModel         string `json:"plugins_model"`
		} `json:"categories"`
	}

	var result EnginesData
	json.NewDecoder(response.Body).Decode(&result)
	fmt.Println(result)
}

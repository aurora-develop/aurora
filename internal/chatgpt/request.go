package chatgpt

import (
	"aurora/conversion/response/chatgpt"
	"aurora/httpclient"
	"aurora/internal/prooftoken"
	"aurora/internal/tokens"
	"aurora/typings"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
	"aurora/util"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"

	"github.com/PuerkitoBio/goquery"

	//http "github.com/bogdanfinn/fhttp"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var BaseURL string

func init() {
	_ = godotenv.Load(".env")
	BaseURL = os.Getenv("BASE_URL")
	if BaseURL == "" {
		BaseURL = "https://chatgpt.com/backend-api"
	}
	cores := []int{8, 12, 16, 24}
	screens := []int{3000, 4000, 6000}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	core := cores[rand.Intn(4)]
	rand.New(rand.NewSource(time.Now().UnixNano()))
	screen := screens[rand.Intn(3)]
	cachedHardware = core + screen
}

var (
	API_REVERSE_PROXY   = os.Getenv("API_REVERSE_PROXY")
	FILES_REVERSE_PROXY = os.Getenv("FILES_REVERSE_PROXY")
	userAgent           = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"
	timeLayout          = "Mon Jan 2 2006 15:04:05"
	BasicCookies        []*http.Cookie
	cachedHardware      = 0
	cachedScripts       = []string{}
	cachedDpl           = ""
	cachedRequireProof  = ""
)

func GetDpl(client httpclient.AuroraHttpClient, proxy string) {
	requestURL := strings.Replace(BaseURL, "/backend-api", "", 1)

	if len(cachedScripts) > 0 {
		return
	}
	if proxy != "" {
		client.SetProxy(proxy)
	}
	header := createBaseHeader()
	response, err := client.Request(http.MethodGet, requestURL, header, nil, nil)

	if err != nil {
		return
	}
	defer response.Body.Close()
	doc, _ := goquery.NewDocumentFromReader(response.Body)
	cachedScripts = nil
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists {
			cachedScripts = append(cachedScripts, src)
			if cachedDpl == "" {
				idx := strings.Index(src, "dpl")
				if idx >= 0 {
					cachedDpl = src[idx:]
				}
			}
		}
	})
	if BasicCookies == nil {
		for _, cookie := range client.GetCookies("https://chatgpt.com") {
			if cookie.Name == "oai-did" {
				continue
			}
			if cookie.Name == "__Secure-next-auth.callback-url" {
				cookie.Value = "https://chatgpt.com"
			}
			BasicCookies = append(BasicCookies, cookie)
		}
	}
	if len(cachedScripts) == 0 {
		cachedScripts = append(cachedScripts, "https://cdn.oaistatic.com/_next/static/chunks/polyfills-78c92fac7aa8fdd8.js?dpl=baf36960d05dde6d8b941194fa4093fb5cb78c6a")
		cachedDpl = "dpl=baf36960d05dde6d8b941194fa4093fb5cb78c6a"
	}
}

type TurnStile struct {
	TurnStileToken   string
	ProofOfWorkToken string
	TurnstileToken   string
}

type ProofWork struct {
	Difficulty string `json:"difficulty,omitempty"`
	Required   bool   `json:"required"`
	Seed       string `json:"seed,omitempty"`
}

func GetConfig() []interface{} {
	return nil
}
func GetInitConfig() []interface{} {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	script := cachedScripts[rand.Intn(len(cachedScripts))]
	startTime := time.Now()
	timeNum := (float64(time.Since(startTime).Nanoseconds()) + rand.Float64()) / 1e6
	loc := time.FixedZone("Eastern Standard Time", -5*60*60)
	parseTime := time.Now().In(loc).Format("Mon Jan 02 2006 15:04:05") + " GMT-0500 (Eastern Standard Time)"
	return []interface{}{cachedHardware, parseTime, int64(4294705152), 0, userAgent, script, cachedDpl, "zh-CN", "zh-CN", 0, "webkitGetUserMedia−function webkitGetUserMedia() { [native code] }", "location", "ontransitionend", timeNum, uuid.NewString()}
}

func CalcProofToken(require *ChatRequire) string {
	return prooftoken.CalcProofToken(require.Proof.Seed, require.Proof.Difficulty, userAgent)
}

type ChatRequire struct {
	Persona      string    `json:"persona,omitempty"`
	Token        string    `json:"token"`
	PrepareToken string    `json:"prepare_token,omitempty"`
	Proof        ProofWork `json:"proofofwork,omitempty"`
	Turnstile    struct {
		Required bool   `json:"required"`
		DX       string `json:"dx,omitempty"`
	} `json:"turnstile"`
	ForceLogin bool `json:"force_login"`
}

type sentinelFinalizeResponse struct {
	Persona     string `json:"persona,omitempty"`
	Token       string `json:"token"`
	ExpireAfter int    `json:"expire_after,omitempty"`
}

func InitTurnStile(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string) (*TurnStile, int, error) {
	return InitSentinel(client, secret, proxy, 0)
}

func InitSentinel(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string, retry int) (*TurnStile, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	requirementsToken := prooftoken.LegacyRequirementsToken(userAgent)
	prepare, status, err := POSTSentinelPrepare(client, secret, requirementsToken)
	if err != nil {
		if secret.IsFree && status == http.StatusUnauthorized && retry < 2 {
			time.Sleep(time.Second * 2)
			secret.Token = uuid.NewString()
			return InitSentinel(client, secret, proxy, retry+1)
		}
		return nil, status, err
	}
	if prepare.ForceLogin {
		if !secret.IsFree {
			return nil, http.StatusUnauthorized, fmt.Errorf("force login required: ChatGPT access token is expired or not accepted")
		}
		if retry > 1 {
			return nil, http.StatusForbidden, fmt.Errorf("force login required")
		}
		time.Sleep(time.Second)
		secret.Token = uuid.NewString()
		return InitSentinel(client, secret, proxy, retry+1)
	}
	if prepare.PrepareToken == "" {
		return nil, status, fmt.Errorf("sentinel prepare token is missing")
	}

	var proofToken string
	if prepare.Proof.Required {
		proofToken = CalcProofToken(prepare)
		if proofToken == "" {
			return nil, http.StatusForbidden, errors.New("calculation proof token failure. Please retry the operation")
		}
	}
	var turnstileToken string
	if prepare.Turnstile.DX != "" {
		turnstileToken = prooftoken.Solve(prepare.Turnstile.DX, requirementsToken)
		if turnstileToken == "" {
			turnstileToken = prooftoken.Solve(prepare.Turnstile.DX, "")
		}
	}

	finalize, status, err := POSTSentinelFinalize(client, secret, prepare.PrepareToken, proofToken, turnstileToken)
	if err != nil {
		if secret.IsFree && status == http.StatusUnauthorized && retry < 2 {
			time.Sleep(time.Second * 2)
			secret.Token = uuid.NewString()
			return InitSentinel(client, secret, proxy, retry+1)
		}
		return nil, status, err
	}
	if finalize.Token == "" {
		return nil, status, fmt.Errorf("sentinel finalize token is missing")
	}

	return &TurnStile{
		TurnStileToken:   finalize.Token,
		ProofOfWorkToken: proofToken,
		TurnstileToken:   turnstileToken,
	}, status, nil
}

func POSTSentinelPrepare(client httpclient.AuroraHttpClient, secret *tokens.Secret, requirementsToken string) (*ChatRequire, int, error) {
	apiUrl, targetPath := sentinelURL(secret, "/sentinel/chat-requirements/prepare")
	bodyJSON, err := json.Marshal(map[string]string{"p": requirementsToken})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	header := sentinelHeader(secret, targetPath)
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("sentinel prepare failed: %s", readResponseSnippet(response.Body, 500))
	}
	var result ChatRequire
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, response.StatusCode, err
	}
	return &result, response.StatusCode, nil
}

func POSTSentinelFinalize(client httpclient.AuroraHttpClient, secret *tokens.Secret, prepareToken, proofToken, turnstileToken string) (*sentinelFinalizeResponse, int, error) {
	apiUrl, targetPath := sentinelURL(secret, "/sentinel/chat-requirements/finalize")
	payload := map[string]string{"prepare_token": prepareToken}
	if proofToken != "" {
		payload["proofofwork"] = proofToken
	}
	if turnstileToken != "" {
		payload["turnstile"] = turnstileToken
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	header := sentinelHeader(secret, targetPath)
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("sentinel finalize failed: %s", readResponseSnippet(response.Body, 500))
	}
	var result sentinelFinalizeResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, response.StatusCode, err
	}
	return &result, response.StatusCode, nil
}

func sentinelURL(secret *tokens.Secret, path string) (string, string) {
	if secret != nil && secret.IsFree {
		return strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + path, "/backend-anon" + path
	}
	return BaseURL + path, "/backend-api" + path
}

func sentinelHeader(secret *tokens.Secret, targetPath string) httpclient.AuroraHeaders {
	header := createBaseHeader()
	header.Set("Accept", "*/*")
	header.Set("Content-Type", "application/json")
	header.Set("x-openai-target-path", targetPath)
	header.Set("x-openai-target-route", targetPath)
	if secret != nil && secret.IsFree && secret.Token != "" {
		header.Set("oai-device-id", secret.Token)
	}
	if secret != nil && !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	setTeamAccountHeader(header, secret)
	return header
}

func readResponseSnippet(body io.Reader, limit int64) string {
	if limit <= 0 {
		limit = 500
	}
	data, err := io.ReadAll(io.LimitReader(body, limit))
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func POSTTurnStile(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string, retry int) (*ChatRequire, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	if cachedRequireProof == "" {
		cachedRequireProof = prooftoken.LegacyRequirementsToken(userAgent)
	}
	var apiUrl string
	if secret.IsFree {
		apiUrl = strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + "/sentinel/chat-requirements"
	} else {
		apiUrl = BaseURL + "/sentinel/chat-requirements"
	}
	payload := bytes.NewBuffer([]byte(`{"p":"` + cachedRequireProof + `"}`))

	header := createBaseHeader()
	header.Set("content-type", "application/json")
	if !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.IsFree {
		header.Set("oai-device-id", secret.Token)
	}
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, payload)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if response.StatusCode == 401 && secret.IsFree {
		if retry > 1 {
			return nil, http.StatusUnauthorized, err
		}
		time.Sleep(time.Second * 2)
		secret.Token = uuid.NewString()
		return POSTTurnStile(client, secret, proxy, retry+1)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, response.StatusCode, fmt.Errorf("failed to get chat requirements")
	}
	var result ChatRequire
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, response.StatusCode, err
	}
	if result.ForceLogin {
		if !secret.IsFree {
			return nil, http.StatusUnauthorized, fmt.Errorf("force login required: ChatGPT access token is expired or not accepted")
		}
		if retry > 1 {
			return nil, http.StatusForbidden, fmt.Errorf("force login required")
		}
		time.Sleep(time.Second * 1)
		secret.Token = uuid.NewString()
		return POSTTurnStile(client, secret, proxy, retry+1)
	}
	if strings.HasPrefix(result.Proof.Difficulty, "00003") {
		if retry > 1 {
			return &result, response.StatusCode, err
		}
		time.Sleep(time.Millisecond * 128)
		return POSTTurnStile(client, secret, proxy, retry+1)
	}

	return &result, response.StatusCode, err
}

var urlAttrMap = make(map[string]string)

type urlAttr struct {
	Url         string `json:"url"`
	Attribution string `json:"attribution"`
}

func setTeamAccountHeader(header httpclient.AuroraHeaders, secret *tokens.Secret) {
	if secret != nil && strings.TrimSpace(secret.TeamUserID) != "" {
		header.Set("Chatgpt-Account-Id", strings.TrimSpace(secret.TeamUserID))
	}
}

func getURLAttribution(client httpclient.AuroraHttpClient, secret *tokens.Secret, url string) string {
	requestURL := BaseURL + "/attributions"
	payload := bytes.NewBuffer([]byte(`{"urls":["` + url + `"]}`))
	header := createBaseHeader()
	if secret != nil && secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	header.Set("Content-Type", "application/json")
	if secret != nil && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	setTeamAccountHeader(header, secret)
	response, err := client.Request(http.MethodPost, requestURL, header, nil, payload)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	var attr urlAttr
	err = json.NewDecoder(response.Body).Decode(&attr)
	if err != nil {
		return ""
	}
	return attr.Attribution
}

func GetCf(proxy string) (string, error) {
	client := &http.Client{}
	if proxy != "" {
		client.Transport = &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(proxy)
			},
		}
	}

	var data = strings.NewReader(`{}`)
	req, err := http.NewRequest("POST", "https://chatgpt.com/cdn-cgi/challenge-platform/h/b/jsd/r/"+util.RandomHexadecimalString(), data)
	if err != nil {
		return "", err
	}
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("origin", "https://chatgpt.com")
	req.Header.Set("sec-ch-ua", `"Google Chrome";v="123", "Not:A-Brand";v="8", "Chromium";v="123"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/44.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("failed to get cf clearance")
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "cf_clearance" {
			return cookie.Value, nil
		}
	}
	return "", errors.New("ailed to get cf clearance")
}

func conversationURL(secret *tokens.Secret, path string) (string, string) {
	if secret != nil && secret.IsFree {
		return strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + path, "/backend-anon" + path
	}
	return BaseURL + path, "/backend-api" + path
}

func conversationHeaders(secret *tokens.Secret, chatToken *TurnStile, accept, targetPath, conduitToken, turnTraceID string) httpclient.AuroraHeaders {
	header := createBaseHeader()
	header.Set("Accept", accept)
	header.Set("Content-Type", "application/json")
	header.Set("x-openai-target-path", targetPath)
	header.Set("x-openai-target-route", targetPath)
	if turnTraceID != "" {
		header.Set("X-Oai-Turn-Trace-Id", turnTraceID)
	}
	if conduitToken != "" {
		header.Set("X-Conduit-Token", conduitToken)
	}
	if chatToken != nil {
		if chatToken.TurnStileToken != "" {
			header.Set("openai-sentinel-chat-requirements-token", chatToken.TurnStileToken)
		}
		if chatToken.ProofOfWorkToken != "" {
			header.Set("openai-sentinel-proof-token", chatToken.ProofOfWorkToken)
		}
		if chatToken.TurnstileToken != "" {
			header.Set("openai-sentinel-turnstile-token", chatToken.TurnstileToken)
		}
	}
	if secret != nil && secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	if secret != nil && secret.IsFree && secret.Token != "" {
		header.Set("oai-device-id", secret.Token)
	}
	if secret != nil && !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	setTeamAccountHeader(header, secret)
	return header
}

func getConduitToken(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, chatToken *TurnStile, turnTraceID string) (string, error) {
	apiUrl, targetPath := conversationURL(secret, "/f/conversation/prepare")
	payload := map[string]interface{}{
		"action":                "next",
		"fork_from_shared_post": false,
		"parent_message_id":     "client-created-root",
		"model":                 conversationPrepareModel(message.Model),
		"timezone_offset_min":   message.TimezoneOffsetMin,
		"timezone":              "Asia/Shanghai",
		"conversation_mode":     map[string]string{"kind": "primary_assistant"},
		"system_hints":          []string{},
		"partial_query": map[string]interface{}{
			"id":      uuid.NewString(),
			"author":  map[string]string{"role": "user"},
			"content": map[string]interface{}{"content_type": "text", "parts": []string{conversationPartialText(message)}},
		},
		"supports_buffering":  true,
		"supported_encodings": []string{"v1"},
		"client_contextual_info": map[string]string{
			"app_name": "chatgpt.com",
		},
		"thinking_effort": "standard",
	}
	if message.ConversationID != "" {
		payload["conversation_id"] = message.ConversationID
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	header := conversationHeaders(secret, chatToken, "*/*", targetPath, "no-token", turnTraceID)
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("conversation prepare failed: %s", string(body))
	}
	var result struct {
		ConduitToken string `json:"conduit_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.ConduitToken == "" {
		return "", fmt.Errorf("missing conduit_token: %s", string(body))
	}
	return result.ConduitToken, nil
}

func conversationPrepareModel(model string) string {
	if model == "" {
		return "auto"
	}
	return model
}

func conversationPartialText(message chatgpt_types.ChatGPTRequest) string {
	for i := len(message.Messages) - 1; i >= 0; i-- {
		msg := message.Messages[i]
		if msg.Author.Role != "user" {
			continue
		}
		for _, part := range msg.Content.Parts {
			if text, ok := part.(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	return "h"
}

func POSTconversation(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, chat_token *TurnStile, proxy string) (*http.Response, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	apiUrl, targetPath := conversationURL(secret, "/f/conversation")
	if API_REVERSE_PROXY != "" {
		apiUrl = API_REVERSE_PROXY
	}
	turnTraceID := uuid.NewString()
	conduitToken, err := getConduitToken(client, message, secret, chat_token, turnTraceID)
	if err != nil {
		return nil, err
	}

	// JSONify the body and add it to the request
	body_json, err := json.Marshal(message)
	if err != nil {
		return &http.Response{}, err
	}
	header := conversationHeaders(secret, chat_token, "text/event-stream", targetPath, conduitToken, turnTraceID)
	if secret.IsFree {
		client.SetCookies("https://chatgpt.com", []*http.Cookie{
			{Name: "oai-device-id", Value: secret.Token, Path: "/", Domain: "chatgpt.com"},
		})
	}

	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewBuffer(body_json))
	if err != nil {
		return nil, err
	}
	return response, nil
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
	} `json:"categories"`
}

func GETengines(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string) (*EnginesData, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	reqUrl := BaseURL + "/models"
	header := make(httpclient.AuroraHeaders)
	header.Set("Content-Type", "application/json")
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "*/*")
	header.Set("oai-language", "en-US")
	header.Set("origin", "https://chatgpt.com")
	header.Set("referer", "https://chatgpt.com/")

	if !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.IsFree {
		header.Set("Oai-Device-Id", secret.Token)
	}
	if secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	resp, err := client.Request(http.MethodGet, reqUrl, header, nil, nil)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var result EnginesData
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, resp.StatusCode, nil
}

func Handle_request_error(c *gin.Context, response *http.Response) bool {
	if response.StatusCode != 200 {
		// Try read response body as JSON
		var error_response map[string]interface{}
		err := json.NewDecoder(response.Body).Decode(&error_response)
		if err != nil {
			// Read response body
			body, _ := io.ReadAll(response.Body)
			c.JSON(response.StatusCode, gin.H{"error": gin.H{
				"message": "Unknown error",
				"type":    "internal_server_error",
				"param":   nil,
				"code":    "500",
				"details": string(body),
			}})
			return true
		}
		c.JSON(response.StatusCode, gin.H{"error": gin.H{
			"message": error_response["detail"],
			"type":    response.Status,
			"param":   nil,
			"code":    "error",
		}})
		return true
	}
	return false
}

type ContinueInfo struct {
	ConversationID string `json:"conversation_id"`
	ParentID       string `json:"parent_id"`
}

type fileInfo struct {
	DownloadURL string `json:"download_url"`
	Status      string `json:"status"`
	URL         string `json:"url"`
}

type ImageGenerationResult struct {
	URL     string
	B64JSON string
}

func GetImageSource(client httpclient.AuroraHttpClient, wg *sync.WaitGroup, url string, prompt string, secret *tokens.Secret, idx int, imgSource []string) {
	defer wg.Done()
	header := make(httpclient.AuroraHeaders)
	// Clear cookies
	if secret != nil && secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "*/*")
	if secret != nil && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	setTeamAccountHeader(header, secret)
	response, err := client.Request(http.MethodGet, url, header, nil, nil)
	if err != nil {
		return
	}
	defer response.Body.Close()
	var file_info fileInfo
	err = json.NewDecoder(response.Body).Decode(&file_info)
	if err != nil || file_info.Status != "success" {
		return
	}
	imgSource[idx] = "[![image](" + file_info.DownloadURL + " \"" + prompt + "\")](" + file_info.DownloadURL + ")"
}

func GetImageDownloadURL(client httpclient.AuroraHttpClient, url string, secret *tokens.Secret) (string, error) {
	header := make(httpclient.AuroraHeaders)
	if secret != nil && secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "*/*")
	if secret != nil && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	setTeamAccountHeader(header, secret)
	response, err := client.Request(http.MethodGet, url, header, nil, nil)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var info fileInfo
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.Status != "" && info.Status != "success" {
		return "", fmt.Errorf("image download url is not ready")
	}
	if info.DownloadURL == "" {
		info.DownloadURL = info.URL
	}
	if info.DownloadURL == "" {
		return "", fmt.Errorf("image download url is missing")
	}
	return info.DownloadURL, nil
}

func DownloadImageBytes(client httpclient.AuroraHttpClient, url string, secret *tokens.Secret) ([]byte, error) {
	header := make(httpclient.AuroraHeaders)
	if secret != nil && secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
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
		return nil, fmt.Errorf("image download failed: %s", string(body))
	}
	return body, nil
}

func addImageResult(results *[]ImageGenerationResult, seen map[string]bool, result ImageGenerationResult) {
	key := result.URL
	if key == "" {
		key = result.B64JSON
	}
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	*results = append(*results, result)
}

func stripDataImagePrefix(value string) (string, bool) {
	if !strings.HasPrefix(value, "data:image/") {
		return value, false
	}
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return value, false
	}
	return parts[1], true
}

func fileDownloadBaseURL() string {
	apiURL := BaseURL + "/files/"
	if FILES_REVERSE_PROXY != "" {
		apiURL = FILES_REVERSE_PROXY
	}
	return strings.TrimRight(apiURL, "/") + "/"
}

func appendAssetPointerResult(client httpclient.AuroraHttpClient, secret *tokens.Secret, results *[]ImageGenerationResult, seen map[string]bool, assetPointer string) {
	if assetPointer == "" {
		return
	}
	assetParts := strings.Split(assetPointer, "//")
	if len(assetParts) != 2 || assetParts[1] == "" {
		return
	}
	downloadURL, err := GetImageDownloadURL(client, fileDownloadBaseURL()+assetParts[1]+"/download", secret)
	if err != nil {
		return
	}
	addImageResult(results, seen, ImageGenerationResult{URL: downloadURL})
}

func appendFileIDResult(client httpclient.AuroraHttpClient, secret *tokens.Secret, results *[]ImageGenerationResult, seen map[string]bool, fileID string) {
	if fileID == "" {
		return
	}
	downloadURL, err := GetImageDownloadURL(client, fileDownloadBaseURL()+fileID+"/download", secret)
	if err != nil {
		return
	}
	addImageResult(results, seen, ImageGenerationResult{URL: downloadURL})
}

func collectImageResultsFromValue(client httpclient.AuroraHttpClient, secret *tokens.Secret, value interface{}, results *[]ImageGenerationResult, seen map[string]bool) {
	switch item := value.(type) {
	case map[string]interface{}:
		if result, ok := item["result"].(string); ok && result != "" {
			if b64, isDataImage := stripDataImagePrefix(result); isDataImage {
				addImageResult(results, seen, ImageGenerationResult{B64JSON: b64})
			}
		}
		for _, key := range []string{"asset_pointer", "assetPointer"} {
			if assetPointer, ok := item[key].(string); ok {
				appendAssetPointerResult(client, secret, results, seen, assetPointer)
			}
		}
		for _, key := range []string{"file_id", "fileId", "id"} {
			if fileID, ok := item[key].(string); ok && strings.HasPrefix(fileID, "file-") {
				appendFileIDResult(client, secret, results, seen, fileID)
			}
		}
		for _, key := range []string{"download_url", "downloadUrl", "url"} {
			if rawURL, ok := item[key].(string); ok && strings.HasPrefix(rawURL, "http") {
				addImageResult(results, seen, ImageGenerationResult{URL: rawURL})
			}
		}
		for _, nested := range item {
			collectImageResultsFromValue(client, secret, nested, results, seen)
		}
	case []interface{}:
		for _, nested := range item {
			collectImageResultsFromValue(client, secret, nested, results, seen)
		}
	case string:
		if b64, isDataImage := stripDataImagePrefix(item); isDataImage {
			addImageResult(results, seen, ImageGenerationResult{B64JSON: b64})
		}
	}
}

func CollectImageResults(response *http.Response, client httpclient.AuroraHttpClient, secret *tokens.Secret) ([]ImageGenerationResult, string, string, error) {
	reader := bufio.NewReader(response.Body)
	var originalResponse chatgpt_types.ChatGPTResponse
	var convID string
	var results []ImageGenerationResult
	seen := make(map[string]bool)
	var textParts []string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return results, convID, strings.Join(textParts, ""), err
		}
		if len(line) < 6 {
			continue
		}
		line = line[6:]
		if strings.HasPrefix(line, "[DONE]") {
			break
		}
		originalResponse.Message.ID = ""
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			collectImageResultsFromValue(client, secret, raw, &results, seen)
		}
		if err := json.Unmarshal([]byte(line), &originalResponse); err != nil {
			continue
		}
		if originalResponse.Error != nil {
			return results, convID, strings.Join(textParts, ""), fmt.Errorf("image generation error: %v", originalResponse.Error)
		}
		if originalResponse.ConversationID != convID {
			if convID == "" {
				convID = originalResponse.ConversationID
			} else {
				continue
			}
		}
		if originalResponse.Message.Recipient != "all" {
			continue
		}
		if originalResponse.Message.Content.ContentType == "text" && len(originalResponse.Message.Content.Parts) > 0 {
			if text, ok := originalResponse.Message.Content.Parts[0].(string); ok && text != "" {
				textParts = append(textParts, text)
			}
			continue
		}
		if originalResponse.Message.Content.ContentType != "multimodal_text" {
			continue
		}
		for _, part := range originalResponse.Message.Content.Parts {
			jsonItem, _ := json.Marshal(part)
			var dalleContent chatgpt_types.DalleContent
			if err := json.Unmarshal(jsonItem, &dalleContent); err != nil || dalleContent.AssetPointer == "" {
				continue
			}
			appendAssetPointerResult(client, secret, &results, seen, dalleContent.AssetPointer)
		}
	}
	return results, convID, strings.Join(textParts, ""), nil
}

func conversationFetchHeaders(secret *tokens.Secret) httpclient.AuroraHeaders {
	header := createBaseHeader()
	header.Set("Accept", "application/json")
	header.Set("Content-Type", "application/json")
	if secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	setTeamAccountHeader(header, secret)
	return header
}

func getConversation(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID string) (map[string]interface{}, error) {
	if conversationID == "" {
		return nil, fmt.Errorf("missing conversation id")
	}
	reqURL := BaseURL + "/conversation/" + conversationID
	response, err := client.Request(http.MethodGet, reqURL, conversationFetchHeaders(secret), nil, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get conversation failed: %s", string(body))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func collectImageResultsFromConversation(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversation map[string]interface{}) []ImageGenerationResult {
	var results []ImageGenerationResult
	seen := make(map[string]bool)
	collectImageResultsFromValue(client, secret, conversation, &results, seen)
	return results
}

func findImageGenerationError(value interface{}) string {
	switch item := value.(type) {
	case map[string]interface{}:
		if itemType, ok := item["type"].(string); ok {
			switch itemType {
			case "content_policy_violation", "content_policy_error":
				if message, ok := item["message"].(string); ok && message != "" {
					return message
				}
				return "Image generation was rejected by the upstream content policy."
			}
		}
		if code, ok := item["code"].(string); ok && strings.Contains(strings.ToLower(code), "content_policy") {
			if message, ok := item["message"].(string); ok && message != "" {
				return message
			}
			return "Image generation was rejected by the upstream content policy."
		}
		for _, nested := range item {
			if message := findImageGenerationError(nested); message != "" {
				return message
			}
		}
	case []interface{}:
		for _, nested := range item {
			if message := findImageGenerationError(nested); message != "" {
				return message
			}
		}
	}
	return ""
}

func PollImageResults(client httpclient.AuroraHttpClient, secret *tokens.Secret, conversationID string, initial []ImageGenerationResult) ([]ImageGenerationResult, error) {
	if len(initial) > 0 || conversationID == "" {
		return initial, nil
	}
	var lastErr error
	for i := 0; i < 45; i++ {
		if i > 0 {
			time.Sleep(2 * time.Second)
		}
		conversation, err := getConversation(client, secret, conversationID)
		if err != nil {
			lastErr = err
			continue
		}
		if message := findImageGenerationError(conversation); message != "" {
			return nil, errors.New(message)
		}
		results := collectImageResultsFromConversation(client, secret, conversation)
		if len(results) > 0 {
			return results, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

func imageModelSlug(model string) string {
	if model == "" || strings.HasPrefix(model, "dall-e") {
		model = "gpt-image-2"
	}
	if model == "gpt-image-2" || strings.HasPrefix(model, "gpt-image") {
		return "auto"
	}
	return model
}

func imageConversationHeaders(secret *tokens.Secret, turnStile *TurnStile, conduitToken, accept string) httpclient.AuroraHeaders {
	header := createBaseHeader()
	header.Set("Content-Type", "application/json")
	header.Set("Accept", accept)
	header.Set("OpenAI-Sentinel-Chat-Requirements-Token", turnStile.TurnStileToken)
	if turnStile.ProofOfWorkToken != "" {
		header.Set("OpenAI-Sentinel-Proof-Token", turnStile.ProofOfWorkToken)
	}
	if turnStile.TurnstileToken != "" {
		header.Set("OpenAI-Sentinel-Turnstile-Token", turnStile.TurnstileToken)
	}
	if conduitToken != "" {
		header.Set("X-Conduit-Token", conduitToken)
	}
	if accept == "text/event-stream" {
		header.Set("X-Oai-Turn-Trace-Id", uuid.NewString())
	}
	if secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	setTeamAccountHeader(header, secret)
	return header
}

func prepareImageConversation(client httpclient.AuroraHttpClient, secret *tokens.Secret, turnStile *TurnStile, prompt, model string) (string, error) {
	payload := map[string]interface{}{
		"action":                "next",
		"fork_from_shared_post": false,
		"parent_message_id":     uuid.NewString(),
		"model":                 imageModelSlug(model),
		"client_prepare_state":  "success",
		"timezone_offset_min":   -480,
		"timezone":              "Asia/Shanghai",
		"conversation_mode":     map[string]string{"kind": "primary_assistant"},
		"system_hints":          []string{"picture_v2"},
		"partial_query": map[string]interface{}{
			"id":      uuid.NewString(),
			"author":  map[string]string{"role": "user"},
			"content": map[string]interface{}{"content_type": "text", "parts": []string{prompt}},
		},
		"supports_buffering":  true,
		"supported_encodings": []string{"v1"},
		"client_contextual_info": map[string]string{
			"app_name": "chatgpt.com",
		},
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	response, err := client.Request(http.MethodPost, BaseURL+"/f/conversation/prepare", imageConversationHeaders(secret, turnStile, "", "*/*"), nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("prepare image conversation failed: %s", string(body))
	}
	var result struct {
		ConduitToken string `json:"conduit_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.ConduitToken == "" {
		return "", fmt.Errorf("missing conduit_token: %s", string(body))
	}
	return result.ConduitToken, nil
}

func GeneratePictureConversationImages(client httpclient.AuroraHttpClient, secret *tokens.Secret, turnStile *TurnStile, prompt, model, proxy string) ([]ImageGenerationResult, string, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	conduitToken, err := prepareImageConversation(client, secret, turnStile, prompt, model)
	if err != nil {
		return nil, "", err
	}
	payload := map[string]interface{}{
		"action": "next",
		"messages": []map[string]interface{}{
			{
				"id":          uuid.NewString(),
				"author":      map[string]string{"role": "user"},
				"create_time": time.Now().Unix(),
				"content":     map[string]interface{}{"content_type": "text", "parts": []string{prompt}},
				"metadata": map[string]interface{}{
					"developer_mode_connector_ids": []interface{}{},
					"selected_github_repos":        []interface{}{},
					"selected_all_github_repos":    false,
					"system_hints":                 []string{"picture_v2"},
					"serialization_metadata":       map[string]interface{}{"custom_symbol_offsets": []interface{}{}},
				},
			},
		},
		"parent_message_id":        uuid.NewString(),
		"model":                    imageModelSlug(model),
		"client_prepare_state":     "sent",
		"timezone_offset_min":      -480,
		"timezone":                 "Asia/Shanghai",
		"conversation_mode":        map[string]string{"kind": "primary_assistant"},
		"enable_message_followups": true,
		"system_hints":             []string{"picture_v2"},
		"supports_buffering":       true,
		"supported_encodings":      []string{"v1"},
		"client_contextual_info": map[string]interface{}{
			"is_dark_mode":      false,
			"time_since_loaded": 1200,
			"page_height":       1072,
			"page_width":        1724,
			"pixel_ratio":       1.2,
			"screen_height":     1440,
			"screen_width":      2560,
			"app_name":          "chatgpt.com",
		},
		"paragen_cot_summary_display_override": "allow",
		"force_parallel_switch":                "auto",
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	response, err := client.Request(http.MethodPost, BaseURL+"/f/conversation", imageConversationHeaders(secret, turnStile, conduitToken, "text/event-stream"), nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, "", fmt.Errorf("image conversation failed: %s", string(body))
	}
	results, conversationID, upstreamText, err := CollectImageResults(response, client, secret)
	if err != nil {
		return results, upstreamText, err
	}
	results, err = PollImageResults(client, secret, conversationID, results)
	if err != nil {
		return results, upstreamText, err
	}
	return results, upstreamText, nil
}

func Handler(c *gin.Context, response *http.Response, client httpclient.AuroraHttpClient, secret *tokens.Secret, uuid string, translated_request chatgpt_types.ChatGPTRequest, stream bool, model string) (string, *ContinueInfo) {
	max_tokens := false

	// Create a bufio.Reader from the response body
	reader := bufio.NewReader(response.Body)

	// Read the response byte by byte until a newline character is encountered
	if stream {
		// Response content type is text/event-stream
		c.Header("Content-Type", "text/event-stream")
	} else {
		// Response content type is application/json
		c.Header("Content-Type", "application/json")
	}
	var finish_reason string
	var previous_text typings.StringStruct
	var original_response chatgpt_types.ChatGPTResponse
	var isRole = true
	var waitSource = false
	var isEnd = false
	var imgSource []string
	var convId string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", nil
		}
		if len(line) < 6 {
			continue
		}
		// Remove "data: " from the beginning of the line
		line = line[6:]
		// Check if line starts with [DONE]
		if !strings.HasPrefix(line, "[DONE]") {
			// Parse the line as JSON
			err = json.Unmarshal([]byte(line), &original_response)
			if err != nil {
				continue
			}
			if original_response.Error != nil {
				c.JSON(500, gin.H{"error": original_response.Error})
				return "", nil
			}
			if original_response.ConversationID != convId {
				if convId == "" {
					convId = original_response.ConversationID
				} else {
					continue
				}
			}
			if !(original_response.Message.Author.Role == "assistant" || (original_response.Message.Author.Role == "tool" && original_response.Message.Content.ContentType != "text")) || original_response.Message.Content.Parts == nil {
				continue
			}
			if original_response.Message.Metadata.MessageType == "" {
				continue
			}
			if original_response.Message.Metadata.MessageType != "next" && original_response.Message.Metadata.MessageType != "continue" || !strings.HasSuffix(original_response.Message.Content.ContentType, "text") {
				continue
			}
			if original_response.Message.EndTurn != nil {
				if waitSource {
					waitSource = false
				}
				isEnd = true
			}
			if len(original_response.Message.Metadata.Citations) != 0 {
				r := []rune(original_response.Message.Content.Parts[0].(string))
				if waitSource {
					if string(r[len(r)-1:]) == "】" {
						waitSource = false
					} else {
						continue
					}
				}
				offset := 0
				for _, citation := range original_response.Message.Metadata.Citations {
					rl := len(r)
					attr := urlAttrMap[citation.Metadata.URL]
					if attr == "" {
						u, _ := url.Parse(citation.Metadata.URL)
						BaseURL := u.Scheme + "://" + u.Host + "/"
						attr = getURLAttribution(client, secret, BaseURL)
						if attr != "" {
							urlAttrMap[citation.Metadata.URL] = attr
						}
					}
					original_response.Message.Content.Parts[0] = string(r[:citation.StartIx+offset]) + " ([" + attr + "](" + citation.Metadata.URL + " \"" + citation.Metadata.Title + "\"))" + string(r[citation.EndIx+offset:])
					r = []rune(original_response.Message.Content.Parts[0].(string))
					offset += len(r) - rl
				}
			} else if waitSource {
				continue
			}
			response_string := ""
			if original_response.Message.Recipient != "all" {
				continue
			}
			if original_response.Message.Content.ContentType == "multimodal_text" {
				apiUrl := BaseURL + "/files/"
				if FILES_REVERSE_PROXY != "" {
					apiUrl = FILES_REVERSE_PROXY
				}
				imgSource = make([]string, len(original_response.Message.Content.Parts))
				var wg sync.WaitGroup
				for index, part := range original_response.Message.Content.Parts {
					jsonItem, _ := json.Marshal(part)
					var dalle_content chatgpt_types.DalleContent
					err = json.Unmarshal(jsonItem, &dalle_content)
					if err != nil {
						continue
					}
					url := apiUrl + strings.Split(dalle_content.AssetPointer, "//")[1] + "/download"
					wg.Add(1)
					go GetImageSource(client, &wg, url, dalle_content.Metadata.Dalle.Prompt, secret, index, imgSource)
				}
				wg.Wait()
				translated_response := official_types.NewChatCompletionChunk(strings.Join(imgSource, ""), model)
				if isRole {
					translated_response.Choices[0].Delta.Role = original_response.Message.Author.Role
				}
				response_string = "data: " + translated_response.String() + "\n\n"
			}
			if response_string == "" {
				response_string = chatgpt.ConvertToString(&original_response, &previous_text, isRole, model)
			}
			if response_string == "" {
				if isEnd {
					goto endProcess
				} else {
					continue
				}
			}
			if response_string == "【" {
				waitSource = true
				continue
			}
		endProcess:
			isRole = false
			if stream {
				_, err = c.Writer.WriteString(response_string)
				if err != nil {
					return "", nil
				}
				c.Writer.Flush()
			}

			if original_response.Message.Metadata.FinishDetails != nil {
				if original_response.Message.Metadata.FinishDetails.Type == "max_tokens" {
					max_tokens = true
				}
				finish_reason = original_response.Message.Metadata.FinishDetails.Type
			}
			if isEnd {
				if stream {
					final_line := official_types.StopChunk(finish_reason, model)
					c.Writer.WriteString("data: " + final_line.String() + "\n\n")
				}
				break
			}
		}
	}
	if !max_tokens {
		return strings.Join(imgSource, "") + previous_text.Text, nil
	}
	return strings.Join(imgSource, "") + previous_text.Text, &ContinueInfo{
		ConversationID: original_response.ConversationID,
		ParentID:       original_response.Message.ID,
	}
}

type AuthSession struct {
	User struct {
		Id           string        `json:"id"`
		Name         string        `json:"name"`
		Email        string        `json:"email"`
		Image        string        `json:"image"`
		Picture      string        `json:"picture"`
		Idp          string        `json:"idp"`
		Iat          int           `json:"iat"`
		Mfa          bool          `json:"mfa"`
		Groups       []interface{} `json:"groups"`
		IntercomHash string        `json:"intercom_hash"`
	} `json:"user"`
	Expires      time.Time `json:"expires"`
	AccessToken  string    `json:"accessToken"`
	AuthProvider string    `json:"authProvider"`
}

func GETTokenForRefreshToken(client httpclient.AuroraHttpClient, refresh_token string, proxy string) (interface{}, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	rawUrl := "https://auth.openai.com/oauth/token"

	data := map[string]interface{}{
		"redirect_uri":  "com.openai.chat://auth.openai.com/ios/com.openai.chat/callback",
		"grant_type":    "refresh_token",
		"client_id":     "pdlLIX2Y72MIl2rhLhTE9VV9bN905kBh",
		"refresh_token": refresh_token,
	}

	reqBody, err := json.Marshal(data)
	if err != nil {
		return nil, 0, err
	}

	header := make(httpclient.AuroraHeaders)
	//req, _ := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	header.Set("authority", "auth.openai.com")
	header.Set("Accept-Language", "en-US,en;q=0.9")
	header.Set("Content-Type", "application/json")
	header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
	header.Set("Accept", "*/*")
	resp, err := client.Request(http.MethodPost, rawUrl, header, nil, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, 0, err
	}
	return result, resp.StatusCode, nil
}

func GETTokenForSessionToken(client httpclient.AuroraHttpClient, session_token string, proxy string) (interface{}, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	url := "https://chatgpt.com/api/auth/session"
	header := make(httpclient.AuroraHeaders)
	header.Set("authority", "chat.openai.com")
	header.Set("accept-language", "zh-CN,zh;q=0.9")
	header.Set("User-Agent", userAgent)
	header.Set("Accept", "*/*")
	header.Set("oai-language", "en-US")
	header.Set("origin", "https://chatgpt.com")
	header.Set("referer", "https://chatgpt.com/")
	header.Set("cookie", "__Secure-next-auth.session-token="+session_token)
	resp, err := client.Request(http.MethodGet, url, header, nil, nil)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var result AuthSession
	json.NewDecoder(resp.Body).Decode(&result)

	cookies := parseCookies(resp.Cookies())
	if value, ok := cookies["__Secure-next-auth.session-token"]; ok {
		session_token = value
	}
	openai_sessionToken := official_types.NewOpenAISessionToken(session_token, result.AccessToken)
	return openai_sessionToken, resp.StatusCode, nil
}

func parseCookies(cookies []*http.Cookie) map[string]string {
	cookieDict := make(map[string]string)
	for _, cookie := range cookies {
		cookieDict[cookie.Name] = cookie.Value
	}
	return cookieDict
}

func createBaseHeader() httpclient.AuroraHeaders {
	header := make(httpclient.AuroraHeaders)
	header.Set("accept", "*/*")
	header.Set("accept-language", "en-US,en;q=0.9")
	header.Set("oai-language", util.RandomLanguage())
	header.Set("origin", "https://chatgpt.com")
	header.Set("referer", "https://chatgpt.com/")
	header.Set("sec-ch-ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	header.Set("sec-ch-ua-arch", `"x86"`)
	header.Set("sec-ch-ua-bitness", `"64"`)
	header.Set("sec-ch-ua-full-version", `"148.0.7778.218"`)
	header.Set("sec-ch-ua-full-version-list", `"Chromium";v="148.0.7778.218", "Google Chrome";v="148.0.7778.218", "Not/A)Brand";v="99.0.0.0"`)
	header.Set("sec-ch-ua-mobile", "?0")
	header.Set("sec-ch-ua-model", `""`)
	header.Set("sec-ch-ua-platform", `"Windows"`)
	header.Set("sec-ch-ua-platform-version", `"19.0.0"`)
	header.Set("priority", "u=1, i")
	header.Set("sec-fetch-dest", "empty")
	header.Set("sec-fetch-mode", "cors")
	header.Set("sec-fetch-site", "same-origin")
	header.Set("user-agent", userAgent)
	header.Set("oai-device-id", uuid.New().String())
	header.Set("oai-session-id", uuid.New().String())
	header.Set("oai-client-version", "prod-a9e268687461965b9507d0c5eeb8d58ad00b12dd")
	header.Set("oai-client-build-number", "7215851")
	return header
}

func HandlerTTS(response *http.Response, input string) (string, string) {
	reader := bufio.NewReader(response.Body)

	var original_response chatgpt_types.ChatGPTResponse
	var convId string
	var fallbackMsgID string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", ""
		}
		if len(line) < 6 {
			continue
		}
		line = line[6:]
		if !strings.HasPrefix(line, "[DONE]") {
			original_response.Message.ID = ""
			err = json.Unmarshal([]byte(line), &original_response)
			if err != nil {
				continue
			}
			if original_response.Error != nil {
				return "", ""
			}
			if original_response.Message.ID == "" {
				continue
			}
			if original_response.ConversationID != convId {
				if convId == "" {
					convId = original_response.ConversationID
				} else {
					continue
				}
			}
			if original_response.Message.Author.Role != "assistant" {
				continue
			}

			// Newer upstream responses are not always an exact single-part echo of the
			// requested TTS input. Prefer an exact match, then fall back to the first
			// assistant message in the same conversation so synthesize still works.
			if fallbackMsgID == "" {
				fallbackMsgID = original_response.Message.ID
			}
			if len(original_response.Message.Content.Parts) == 0 {
				continue
			}
			for _, rawPart := range original_response.Message.Content.Parts {
				part, ok := rawPart.(string)
				if !ok {
					continue
				}
				if part == input || strings.Contains(part, input) || strings.Contains(input, part) {
					return original_response.Message.ID, convId
				}
			}
		}
	}
	if fallbackMsgID != "" && convId != "" {
		return fallbackMsgID, convId
	}
	return "", ""
}

func getTTSBlobFromURL(client httpclient.AuroraHttpClient, secret *tokens.Secret, reqURL string) ([]byte, int, error) {
	header := createBaseHeader()
	header.Set("Accept", "audio/*,*/*")
	if !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.IsFree {
		header.Set("oai-device-id", secret.Token)
	}
	if secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	setTeamAccountHeader(header, secret)
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

func GetTTS(client httpclient.AuroraHttpClient, secret *tokens.Secret, msgId, convId, voice, format, proxy string) ([]byte, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	params := url.Values{}
	params.Set("message_id", msgId)
	params.Set("conversation_id", convId)
	params.Set("voice", voice)
	params.Set("format", format)
	var reqUrl string
	if secret.IsFree {
		reqUrl = strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + "/synthesize?" + params.Encode()
	} else {
		reqUrl = BaseURL + "/synthesize?" + params.Encode()
	}

	blob, status, err := getTTSBlobFromURL(client, secret, reqUrl)
	if err == nil {
		if downloadURL := parseTTSDownloadURL(blob); downloadURL != "" {
			return getTTSBlobFromURL(client, secret, downloadURL)
		}
		return blob, status, nil
	}

	// Some upstream variants now return a signed file URL payload or fail on the
	// first synthesize URL shape. If the error body still contains a download URL,
	// honor it before surfacing the failure.
	if downloadURL := parseTTSDownloadURL(blob); downloadURL != "" {
		return getTTSBlobFromURL(client, secret, downloadURL)
	}
	return nil, status, err
}

func RemoveConversation(client httpclient.AuroraHttpClient, secret *tokens.Secret, id string, proxy string) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	var url string
	if secret.IsFree {
		url = strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + "/conversation/" + id
	} else {
		url = BaseURL + "/conversation/" + id
	}
	header := createBaseHeader()
	header.Set("Content-Type", "application/json")
	if !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.IsFree {
		header.Set("oai-device-id", secret.Token)
	}
	if secret.PUID != "" {
		header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	setTeamAccountHeader(header, secret)
	payload := bytes.NewBuffer([]byte(`{"is_visible":false}`))
	response, err := client.Request(http.MethodPatch, url, header, nil, payload)
	if err != nil {
		return
	}
	response.Body.Close()
}

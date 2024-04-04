package chatgpt

import (
	"aurora/httpclient"
	"aurora/internal/tokens"
	"aurora/typings"
	chatgpt_types "aurora/typings/chatgpt"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aurorax-neo/go-logger"

	"github.com/gorilla/websocket"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/gin-gonic/gin"

	chatgpt_response_converter "aurora/conversion/response/chatgpt"

	official_types "aurora/typings/official"
)

type connInfo struct {
	conn   *websocket.Conn
	uuid   string
	expire time.Time
	ticker *time.Ticker
	lock   bool
}

var (
	client, _ = tls_client.NewHttpClient(tls_client.NewNoopLogger(), []tls_client.HttpClientOption{
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
		tls_client.WithTimeoutSeconds(600),
		tls_client.WithClientProfile(profiles.Okhttp4Android13),
	}...)
	API_REVERSE_PROXY   = os.Getenv("API_REVERSE_PROXY")
	FILES_REVERSE_PROXY = os.Getenv("FILES_REVERSE_PROXY")
	connPool            = map[string][]*connInfo{}
	poolMutex           = sync.Mutex{}
	TurnStilePool       = map[string]*TurnStile{}
	userAgent           = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
)

func getWSURL(token string, retry int) (string, error) {
	//request, err := fhttp.NewRequest(fhttp.MethodPost, "https://chat.openai.com/backend-api/register-websocket", nil)
	//if err != nil {
	//	return "", err
	//}
	//request.Header.Set("User-Agent", userAgent)
	//request.Header.Set("Accept", "*/*")
	//request.Header.Set("Oai-Language", "en-US")
	//if token != "" {
	//	request.Header.Set("Authorization", "Bearer "+token)
	//}
	//response, err := client.Do(request)
	var WSSResp chatgpt_types.ChatGPTWSSResponse
	_, err := httpclient.NewStdClient().Client.R().
		SetResult(&WSSResp).
		Post("https://chat.openai.com/backend-api/register-websocket")
	logger.Logger.Info(fmt.Sprint("getWSURL: ", WSSResp.WssUrl))
	if err != nil {
		if retry > 3 {
			return "", err
		}
		time.Sleep(time.Second) // wait 1s to get ws url
		return getWSURL(token, retry+1)
	}
	return WSSResp.WssUrl, nil
}

func createWSConn(url string, connInfo *connInfo, retry int) error {
	dialer := websocket.DefaultDialer
	dialer.EnableCompression = true
	dialer.Subprotocols = []string{"json.reliable.webpubsub.azure.v1"}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		if retry > 3 {
			return err
		}
		time.Sleep(time.Second) // wait 1s to recreate ws
		return createWSConn(url, connInfo, retry+1)
	}
	connInfo.conn = conn
	connInfo.expire = time.Now().Add(time.Minute * 30)
	ticker := time.NewTicker(time.Second * 8)
	connInfo.ticker = ticker
	go func(ticker *time.Ticker) {
		defer ticker.Stop()
		for {
			<-ticker.C
			if err := connInfo.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				connInfo.conn.Close()
				connInfo.conn = nil
				break
			}
		}
	}(ticker)
	return nil
}

func findAvailConn(token string, uuid string) *connInfo {
	for _, value := range connPool[token] {
		if !value.lock {
			value.lock = true
			value.uuid = uuid
			return value
		}
	}
	newConnInfo := connInfo{uuid: uuid, lock: true}
	connPool[token] = append(connPool[token], &newConnInfo)
	return &newConnInfo
}
func findSpecConn(token string, uuid string) *connInfo {
	for _, value := range connPool[token] {
		if value.uuid == uuid {
			return value
		}
	}
	return &connInfo{}
}
func UnlockSpecConn(token string, uuid string) {
	for _, value := range connPool[token] {
		if value.uuid == uuid {
			value.lock = false
		}
	}
}
func InitWSConn(token string, uuid string, proxy string) error {
	connInfo := findAvailConn(token, uuid)
	conn := connInfo.conn
	isExpired := connInfo.expire.IsZero() || time.Now().After(connInfo.expire)
	if conn == nil || isExpired {
		if proxy != "" {
			client.SetProxy(proxy)
		}
		if conn != nil {
			connInfo.ticker.Stop()
			conn.Close()
			connInfo.conn = nil
		}
		wssURL, err := getWSURL(token, 0)
		if err != nil {
			return err
		}
		createWSConn(wssURL, connInfo, 0)
		if err != nil {
			return err
		}
		return nil
	} else {
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*100)
		go func() {
			defer cancelFunc()
			for {
				_, _, err := conn.NextReader()
				if err != nil {
					break
				}
				if ctx.Err() != nil {
					break
				}
			}
		}()
		<-ctx.Done()
		err := ctx.Err()
		if err != nil {
			switch err {
			case context.Canceled:
				connInfo.ticker.Stop()
				conn.Close()
				connInfo.conn = nil
				connInfo.lock = false
				return InitWSConn(token, uuid, proxy)
			case context.DeadlineExceeded:
				return nil
			default:
				return nil
			}
		}
		return nil
	}
}

type ChatRequire struct {
	Token  string `json:"token"`
	Arkose struct {
		Required bool   `json:"required"`
		DX       string `json:"dx,omitempty"`
	} `json:"arkose"`

	Turnstile struct {
		Required bool `json:"required"`
	}
}

type TurnStile struct {
	TurnStileToken string
	Arkose         bool
	ExpireAt       time.Time
}

func InitTurnStile(client *httpclient.RestyClient, secret *tokens.Secret, proxy string) (*TurnStile, int, error) {
	poolMutex.Lock()
	defer poolMutex.Unlock()
	currTurnToken := TurnStilePool[secret.Token]
	if currTurnToken == nil || currTurnToken.ExpireAt.Before(time.Now()) {
		result, err := POSTTurnStile(client, secret, proxy)
		if err != nil {
			return nil, 500, err
		}

		currTurnToken = &TurnStile{
			TurnStileToken: result.Token,
			Arkose:         result.Arkose.Required,
			ExpireAt:       time.Now().Add(5 * time.Minute),
		}
		TurnStilePool[secret.Token] = currTurnToken
	}
	return currTurnToken, 0, nil
}

func POSTTurnStile(client *httpclient.RestyClient, secret *tokens.Secret, proxy string) (*chatgpt_types.RequirementsResponse, error) {
	if proxy != "" {
		client.Client.SetProxy(proxy)
	}
	if !secret.IsFree && secret.Token != "" {
		client.Client.SetHeader("Authorization", "Bearer "+secret.Token)
	}
	if secret.IsFree {
		client.Client.SetHeader("oai-device-id", secret.Token)
	}
	apiUrl := "https://chat.openai.com/backend-api/sentinel/chat-requirements"
	var result *chatgpt_types.RequirementsResponse
	response, err := client.Client.R().
		SetBody(`{"conversation_mode_kind":"primary_assistant"}`).
		SetResult(&result).
		Post(apiUrl)
	if err != nil {
		logger.Logger.Debug(fmt.Sprint("POSTTurnStile: ", err))
		return nil, err
	}
	if response.StatusCode() != 200 {
		logger.Logger.Debug(fmt.Sprint("POSTTurnStile: ", response.String()))
		return nil, errors.New("error sending request")
	}
	return result, err
}

var urlAttrMap = make(map[string]string)

type urlAttr struct {
	Url         string `json:"url"`
	Attribution string `json:"attribution"`
}

func getURLAttribution(access_token string, puid string, url string) string {
	request, err := fhttp.NewRequest(fhttp.MethodPost, "https://chat.openai.com/backend-api/attributions", bytes.NewBuffer([]byte(`{"urls":["`+url+`"]}`)))
	if err != nil {
		return ""
	}
	if puid != "" {
		request.Header.Set("Cookie", "_puid="+puid+";")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Oai-Language", "en-US")
	request.Header.Set("Origin", "https://chat.openai.com")
	request.Header.Set("Referer", "https://chat.openai.com/")
	if access_token != "" {
		request.Header.Set("Authorization", "Bearer "+access_token)
	}
	if err != nil {
		return ""
	}
	response, err := client.Do(request)
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

func POSTconversation(client *httpclient.RestyClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, chat_token *TurnStile, proxy string) (*http.Response, error) {
	if proxy != "" {
		client.Client.SetProxy(proxy)
	}
	apiUrl := "https://chat.openai.com/backend-anon/conversation"
	if API_REVERSE_PROXY != "" {
		apiUrl = API_REVERSE_PROXY
	}
	arkoseToken := message.ArkoseToken
	message.ArkoseToken = ""
	// Clear cookies
	index, err := client.Client.R().Get("https://chat.openai.com")
	cookies := index.Cookies()
	if secret.PUID != "" {
		cookies = append(cookies, &http.Cookie{
			Name:  "_puid",
			Value: secret.PUID,
		})
	}
	client.Client.SetCookies(index.Cookies())
	if chat_token.Arkose {
		client.Client.SetHeader("openai-sentinel-arkose-token", arkoseToken)
	}
	if chat_token.TurnStileToken != "" {
		client.Client.SetHeader("Openai-Sentinel-Chat-Requirements-Token", chat_token.TurnStileToken)
	}
	if !secret.IsFree && secret.Token != "" {
		client.Client.SetHeader("Authorization", "Bearer "+secret.Token)
	}
	if secret.IsFree {
		client.Client.SetHeader("oai-device-id", secret.Token)
	}
	response, err := client.Client.R().
		SetBody(message).
		SetDoNotParseResponse(true).
		Post(apiUrl)
	if err != nil {
		return &http.Response{}, err
	}
	if response.StatusCode() != 200 {
		logger.Logger.Debug(fmt.Sprint("POSTconversation: ", response.String(), response.StatusCode()))
		return response.RawResponse, errors.New("error sending request")
	}
	return response.RawResponse, err
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

func GETengines(secret *tokens.Secret, proxy string) (*EnginesData, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	reqUrl := "https://chat.openai.com/backend-api/models"
	req, _ := fhttp.NewRequest("GET", reqUrl, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("oai-language", "en-US")
	req.Header.Set("origin", "https://chat.openai.com")
	req.Header.Set("referer", "https://chat.openai.com/")

	if !secret.IsFree && secret.Token != "" {
		req.Header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.IsFree {
		req.Header.Set("Oai-Device-Id", secret.Token)
	}
	if secret.PUID != "" {
		req.Header.Set("Cookie", "_puid="+secret.PUID+";")
	}
	resp, err := client.Do(req)
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
			c.JSON(500, gin.H{"error": gin.H{
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
}

func GetImageSource(wg *sync.WaitGroup, url string, prompt string, token string, puid string, idx int, imgSource []string) {
	defer wg.Done()
	request, err := fhttp.NewRequest(fhttp.MethodGet, url, nil)
	if err != nil {
		return
	}
	// Clear cookies
	if puid != "" {
		request.Header.Set("Cookie", "_puid="+puid+";")
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "*/*")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
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

func Handler(c *gin.Context, response *http.Response, secret *tokens.Secret, uuid string, translated_request chatgpt_types.ChatGPTRequest, stream bool) (string, *ContinueInfo) {
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
	var isWSS = false
	var convId string
	var respId string
	var wssUrl string
	var connInfo *connInfo
	var wsSeq int
	var isWSInterrupt bool = false
	var interruptTimer *time.Timer

	if !strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		isWSS = true
		connInfo = findSpecConn(secret.Token, uuid)
		if connInfo.conn == nil {
			c.JSON(500, gin.H{"error": "No websocket connection"})
			return "", nil
		}
		var wssResponse chatgpt_types.ChatGPTWSSResponse
		json.NewDecoder(response.Body).Decode(&wssResponse)
		wssUrl = wssResponse.WssUrl
		respId = wssResponse.ResponseId
		convId = wssResponse.ConversationId
	}
	for {
		var line string
		var err error
		if isWSS {
			var messageType int
			var message []byte
			if isWSInterrupt {
				if interruptTimer == nil {
					interruptTimer = time.NewTimer(10 * time.Second)
				}
				select {
				case <-interruptTimer.C:
					c.JSON(500, gin.H{"error": "WS interrupt & new WS timeout"})
					return "", nil
				default:
					goto reader
				}
			}
		reader:
			messageType, message, err = connInfo.conn.ReadMessage()
			if err != nil {
				connInfo.ticker.Stop()
				connInfo.conn.Close()
				connInfo.conn = nil
				err := createWSConn(wssUrl, connInfo, 0)
				if err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return "", nil
				}
				isWSInterrupt = true
				connInfo.conn.WriteMessage(websocket.TextMessage, []byte("{\"type\":\"sequenceAck\",\"sequenceId\":"+strconv.Itoa(wsSeq)+"}"))
				continue
			}
			if messageType == websocket.TextMessage {
				var wssMsgResponse chatgpt_types.WSSMsgResponse
				json.Unmarshal(message, &wssMsgResponse)
				if wssMsgResponse.Data.ResponseId != respId {
					continue
				}
				wsSeq = wssMsgResponse.SequenceId
				if wsSeq%50 == 0 {
					connInfo.conn.WriteMessage(websocket.TextMessage, []byte("{\"type\":\"sequenceAck\",\"sequenceId\":"+strconv.Itoa(wsSeq)+"}"))
				}
				base64Body := wssMsgResponse.Data.Body
				bodyByte, err := base64.StdEncoding.DecodeString(base64Body)
				if err != nil {
					continue
				}
				if isWSInterrupt {
					isWSInterrupt = false
					interruptTimer.Stop()
				}
				line = string(bodyByte)
			}
		} else {
			line, err = reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				return "", nil
			}
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
						baseURL := u.Scheme + "://" + u.Host + "/"
						attr = getURLAttribution(secret.Token, secret.PUID, baseURL)
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
				apiUrl := "https://chat.openai.com/backend-api/files/"
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
					go GetImageSource(&wg, url, dalle_content.Metadata.Dalle.Prompt, secret.Token, secret.PUID, index, imgSource)
				}
				wg.Wait()
				translated_response := official_types.NewChatCompletionChunk(strings.Join(imgSource, ""))
				if isRole {
					translated_response.Choices[0].Delta.Role = original_response.Message.Author.Role
				}
				response_string = "data: " + translated_response.String() + "\n\n"
			}
			if response_string == "" {
				response_string = chatgpt_response_converter.ConvertToString(&original_response, &previous_text, isRole)
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
			isRole = false
			if stream {
				_, err = c.Writer.WriteString(response_string)
				if err != nil {
					return "", nil
				}
			}
		endProcess:
			c.Writer.Flush()

			if original_response.Message.Metadata.FinishDetails != nil {
				if original_response.Message.Metadata.FinishDetails.Type == "max_tokens" {
					max_tokens = true
				}
				finish_reason = original_response.Message.Metadata.FinishDetails.Type
			}
			if isEnd {
				if stream {
					final_line := official_types.StopChunk(finish_reason)
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

func GETTokenForRefreshToken(refresh_token string, proxy string) (interface{}, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	url := "https://auth0.openai.com/oauth/token"

	data := map[string]interface{}{
		"redirect_uri":  "com.openai.chat://auth0.openai.com/ios/com.openai.chat/callback",
		"grant_type":    "refresh_token",
		"client_id":     "pdlLIX2Y72MIl2rhLhTE9VV9bN905kBh",
		"refresh_token": refresh_token,
	}

	reqBody, err := json.Marshal(data)
	if err != nil {
		return nil, 0, err
	}

	req, _ := fhttp.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	req.Header.Set("authority", "auth0.openai.com")
	req.Header.Add("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
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

func GETTokenForSessionToken(session_token string, proxy string) (interface{}, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	url := "https://chat.openai.com/api/auth/session"
	req, _ := fhttp.NewRequest("GET", url, nil)

	req.Header.Set("authority", "chat.openai.com")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("oai-language", "en-US")
	req.Header.Set("origin", "https://chat.openai.com")
	req.Header.Set("referer", "https://chat.openai.com/")
	req.Header.Set("cookie", "__Secure-next-auth.session-token="+session_token)
	resp, err := client.Do(req)
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

func parseCookies(cookies []*fhttp.Cookie) map[string]string {

	cookieDict := make(map[string]string)
	for _, cookie := range cookies {
		cookieDict[cookie.Name] = cookie.Value
	}
	return cookieDict
}

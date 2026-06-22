package chatgpt

import (
	"aurora/conversion/response/chatgpt"
	"aurora/httpclient"
	"aurora/internal/prooftoken"
	"aurora/internal/so"
	"aurora/internal/tokens"
	"aurora/internal/turnstile"
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
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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
	// oaiDeviceID / oaiSessionID 进程启动时随机生成。
	// 每次进程启动都重新生成,保持"每次运行都是新设备"的风控画像,
	// 避免多个进程/部署共享同一个指纹导致关联降权。
	// 不落盘:二进制发布到不同机器时指纹天然不同。
	oaiDeviceID        = uuid.NewString()
	oaiSessionID       = uuid.NewString()
	oaiStartTime       = time.Now()
	timeLayout         = "Mon Jan 2 2006 15:04:05"
	BasicCookies       []*http.Cookie
	cachedHardware     = 0
	cachedScripts      = []string{}
	cachedDpl          = ""
	cachedRequireProof = ""
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
			// __Secure-next-auth.callback-url 在登录后服务端会下发,这里强制为根路径
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
	SOToken          string
	soSession        *so.Session
	soSnapshotDX     string
	soChatToken      string
	soFlow           string
	soOnce           sync.Once
	soResult         string
	soErr            error
}

type ProofWork struct {
	Difficulty string `json:"difficulty,omitempty"`
	Required   bool   `json:"required"`
	Seed       string `json:"seed,omitempty"`
}

type SoSegment struct {
	Required    bool   `json:"required"`
	CollectorDX string `json:"collector_dx,omitempty"`
	SnapshotDX  string `json:"snapshot_dx,omitempty"`
}

func GetInitConfig() []interface{} {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	script := cachedScripts[rng.Intn(len(cachedScripts))]
	nowMs := float64(time.Now().UnixMilli())
	perfNow := float64(int64(rng.Float64()*49000)+1000) + rng.Float64()
	timeOrigin := nowMs - perfNow
	loc := time.FixedZone("Pacific Standard Time", -8*60*60)
	parseTime := time.Now().In(loc).Format("Mon Jan 02 2006 15:04:05") + " GMT-0800 (Pacific Standard Time)"

	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	reactSuffix := make([]byte, 11)
	for i := range reactSuffix {
		reactSuffix[i] = letters[rng.Intn(len(letters))]
	}

	return []interface{}{
		cachedHardware,     // [0]  screen.width + screen.height
		parseTime,          // [1]  Date.toString()
		int64(4294967296),  // [2]  jsHeapSizeLimit
		rng.Float64(),      // [3]  Math.random()
		defaultUserAgent(), // [4]  navigator.userAgent
		script,             // [5]  currentScript.src
		nil,                // [6]  documentElement[data-build]
		"en-US",            // [7]  navigator.language
		"en-US,en",         // [8]  navigator.languages.join(",")
		rng.Float64(),      // [9]  Math.random()
		"vibrate−function vibrate() { [native code] }", // [10] navigator 原型方法
		"_reactListening" + string(reactSuffix),        // [11] document 随机 key
		"requestIdleCallback",                          // [12] window 随机 key
		perfNow,                                        // [13] performance.now()
		oaiDeviceID,                                    // [14] device_id
		"",                                             // [15] location.search
		16,                                             // [16] hardwareConcurrency (对齐 prooftoken.NewConfig)
		timeOrigin,                                     // [17] performance.timeOrigin
		0, 0, 0, 0, 0, 0, 0,                            // [18-24] "X in window" 检查
	}
}

func CalcProofToken(require *ChatRequire, state *ChatClientState) string {
	ua := defaultUserAgent()
	if state != nil && state.UserAgent != "" {
		ua = state.UserAgent
	}
	return prooftoken.SolveProofToken(require.Proof.Seed, require.Proof.Difficulty, ua)
}

type ChatRequire struct {
	Persona      string    `json:"persona,omitempty"`
	Token        string    `json:"token"`
	PrepareToken string    `json:"prepare_token,omitempty"`
	Proof        ProofWork `json:"proofofwork"`
	Turnstile    struct {
		Required bool   `json:"required"`
		DX       string `json:"dx,omitempty"`
	} `json:"turnstile"`
	So         SoSegment `json:"so"`
	ForceLogin bool      `json:"force_login"`
}

type sentinelFinalizeResponse struct {
	Persona     string `json:"persona,omitempty"`
	Token       string `json:"token"`
	ExpireAfter int    `json:"expire_after,omitempty"`
}

func InitTurnStileWithState(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string, state *ChatClientState) (*TurnStile, int, error) {
	return InitSentinelWithState(client, secret, proxy, 0, state)
}

func InitSentinel(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string, retry int) (*TurnStile, int, error) {
	return InitSentinelWithState(client, secret, proxy, retry, nil)
}

func InitSentinelWithState(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxy string, retry int, state *ChatClientState) (*TurnStile, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	ua := defaultUserAgent()
	if state != nil && state.UserAgent != "" {
		ua = state.UserAgent
	}
	requirementsToken := prooftoken.NewConfig(ua).RequirementsToken()
	prepare, status, err := POSTSentinelPrepareWithState(client, secret, requirementsToken, state)
	if err != nil {
		if secret.IsFree && status == http.StatusUnauthorized && retry < 2 {
			time.Sleep(time.Second * 2)
			secret.Token = uuid.NewString()
			return InitSentinelWithState(client, secret, proxy, retry+1, state)
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
		return InitSentinelWithState(client, secret, proxy, retry+1, state)
	}
	if prepare.PrepareToken == "" {
		return nil, status, fmt.Errorf("sentinel prepare token is missing")
	}

	var proofToken string
	if prepare.Proof.Required {
		proofToken = CalcProofToken(prepare, state)
		if proofToken == "" {
			return nil, http.StatusForbidden, errors.New("calculation proof token failure. Please retry the operation")
		}
	}
	var turnstileToken string
	if prepare.Turnstile.DX != "" {
		turnstileToken, _ = turnstile.SolveDX(requirementsToken, prepare.Turnstile.DX)
		if turnstileToken == "" {
			turnstileToken, _ = turnstile.SolveDX(requirementsToken, prepare.Turnstile.DX)
		}
	}

	finalize, status, err := POSTSentinelFinalizeWithState(client, secret, prepare.PrepareToken, proofToken, turnstileToken, state)
	if err != nil {
		if secret.IsFree && status == http.StatusUnauthorized && retry < 2 {
			time.Sleep(time.Second * 2)
			secret.Token = uuid.NewString()
			return InitSentinelWithState(client, secret, proxy, retry+1, state)
		}
		return nil, status, err
	}
	if finalize.Token == "" {
		return nil, status, fmt.Errorf("sentinel finalize token is missing")
	}

	ts := &TurnStile{
		TurnStileToken:   finalize.Token,
		ProofOfWorkToken: proofToken,
		TurnstileToken:   turnstileToken,
	}

	// so 段:对齐 out.js se()/Et() 行为——仅在服务端要求时启动 collector(fire-and-forget,
	// 不阻塞当前 prepare)。Snapshot 由 ensureSOToken() 在首次发请求时同步触发。
	if prepare.So.Required && prepare.So.CollectorDX != "" && prepare.So.SnapshotDX != "" && prepare.Token != "" {
		ts.soSession = so.NewSession(requirementsToken, prepare.So.CollectorDX)
		ts.soSnapshotDX = prepare.So.SnapshotDX
		ts.soChatToken = prepare.Token
		ts.soFlow = stateFlow(state, ua)
		ts.soSession.Start()
	}

	return ts, status, nil
}

// stateFlow 推导 so token 里的 flow 字段(对齐 deob_js/out.js:924 ce() 行为)。
// 优先用 secret.Token 当作 flow 标识;若 secret 不可用则用 ua 简写。
func stateFlow(state *ChatClientState, ua string) string {
	if state != nil && state.DeviceID != "" {
		return state.DeviceID
	}
	if ua != "" {
		return "chatgpt-freeaccount"
	}
	return "chatgpt"
}

// soDeviceIDFor 给出 openai-sentinel-so-token 的 deviceID 参数。对齐 out.js
// sessionObserverToken() 流程,deviceID 是 ne.get() 的 key,也是 ce({...}, t) 里的
// id;实际取值对应 qn.getCookies()["oai-did"](out.js:735),即 secret.Token。
func soDeviceIDFor(secret *tokens.Secret) string {
	if secret != nil && secret.Token != "" {
		return secret.Token
	}
	return ""
}

// ensureSOToken 懒求值 openai-sentinel-so-token header 值:第一次调用时跑
// snapshot_dx(复用 collector 留下的 VM 寄存器),后续直接返回缓存结果。
// 对齐 out.js sessionObserverToken():取 snapshot 后用 ce({so,c}, id, flow) 编码。
// deviceID 是这次请求使用的实际 deviceID(通常来自 secret.Token 或 cookie)。
func (ts *TurnStile) ensureSOToken(deviceID string) string {
	if ts == nil || ts.soSession == nil {
		return ts.SOToken
	}
	ts.soOnce.Do(func() {
		soResult, err := ts.soSession.Snapshot(ts.soSnapshotDX)
		if err != nil {
			ts.soErr = err
			return
		}
		ts.soResult = soResult
	})
	if ts.soErr != nil {
		return ""
	}
	if ts.SOToken != "" {
		return ts.SOToken
	}
	tok, err := so.BuildToken(ts.soResult, ts.soChatToken, deviceID, ts.soFlow)
	if err != nil {
		return ""
	}
	ts.SOToken = tok
	return ts.SOToken
}

func POSTSentinelPrepare(client httpclient.AuroraHttpClient, secret *tokens.Secret, requirementsToken string) (*ChatRequire, int, error) {
	return POSTSentinelPrepareWithState(client, secret, requirementsToken, nil)
}

func POSTSentinelPrepareWithState(client httpclient.AuroraHttpClient, secret *tokens.Secret, requirementsToken string, state *ChatClientState) (*ChatRequire, int, error) {
	apiUrl, targetPath := sentinelURL(secret, "/sentinel/chat-requirements/prepare")
	bodyJSON, err := json.Marshal(map[string]string{"p": requirementsToken})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	header := sentinelHeaderWithState(secret, targetPath, state)
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
	return POSTSentinelFinalizeWithState(client, secret, prepareToken, proofToken, turnstileToken, nil)
}

func POSTSentinelFinalizeWithState(client httpclient.AuroraHttpClient, secret *tokens.Secret, prepareToken, proofToken, turnstileToken string, state *ChatClientState) (*sentinelFinalizeResponse, int, error) {
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
	header := sentinelHeaderWithState(secret, targetPath, state)
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

// sentinelReqResponse 是 POST /sentinel/req 的响应。
// 服务端会返回 token + flow 字段(对齐 sdk.deob.pretty.js / OpenSentinel client.js)。
type sentinelReqResponse struct {
	Token     string `json:"token"`
	Flow      string `json:"flow"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	ChatReq   string `json:"chat_req,omitempty"` // 备用:有时服务端把 chat-requirements token 嵌在这里
	Persona   string `json:"persona,omitempty"`
}

// POSTSentinelReq 调用 /sentinel/req 端点 (对齐 conversation.txt 抓包的第 3 步)。
//
// 请求格式(对齐 sdk.deob.pretty.js OpenSentinel/client.js):
//
//	POST /backend-api/sentinel/req
//	Content-Type: text/plain;charset=UTF-8
//	body: {"p":<requirementsToken>,"id":<deviceID>,"flow":"conversation"}
//
// 响应:服务端下发 flow token,可作为后续 prepare/finalize 的辅助 token。
// 失败返回 (nil, status, err)。
func POSTSentinelReq(client httpclient.AuroraHttpClient, secret *tokens.Secret, requirementsToken, deviceID, flow string, state *ChatClientState) (*sentinelReqResponse, int, error) {
	if flow == "" {
		flow = "conversation"
	}
	apiUrl, targetPath := sentinelURL(secret, "/sentinel/req")
	bodyJSON, err := json.Marshal(map[string]string{
		"p":    requirementsToken,
		"id":   deviceID,
		"flow": flow,
	})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	header := createBaseHeaderForState(state)
	header.Set("Accept", "*/*")
	// 对齐 conversation.txt:sentinel/req 端点用 text/plain;charset=UTF-8
	header.Set("Content-Type", "text/plain;charset=UTF-8")
	header.Set("X-Openai-Target-Path", targetPath)
	header.Set("X-Openai-Target-Route", targetPath)
	// referer 应该指向 sentinel/frame.html(对齐 conversation.txt 抓包)
	if state == nil || state.ConversationID == "" {
		header.Set("Referer", "https://chatgpt.com/backend-api/sentinel/frame.html?sv=20260423af3c")
	}
	if secret != nil && secret.IsFree && secret.Token != "" {
		header.Set("Oai-Device-Id", secret.Token)
	}
	if secret != nil && !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	setTeamAccountHeader(header, secret)
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("sentinel req failed: %s", readResponseSnippet(response.Body, 500))
	}
	var result sentinelReqResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, response.StatusCode, err
	}
	return &result, response.StatusCode, nil
}

func sentinelHeader(secret *tokens.Secret, targetPath string) httpclient.AuroraHeaders {
	return sentinelHeaderWithState(secret, targetPath, nil)
}

func sentinelHeaderWithState(secret *tokens.Secret, targetPath string, state *ChatClientState) httpclient.AuroraHeaders {
	header := createBaseHeaderForState(state)
	header.Set("Accept", "*/*")
	header.Set("Content-Type", "application/json")
	header.Set("X-Openai-Target-Path", targetPath)
	header.Set("X-Openai-Target-Route", targetPath)
	if secret != nil && secret.IsFree && secret.Token != "" {
		header.Set("Oai-Device-Id", secret.Token)
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

func conversationURL(secret *tokens.Secret, path string) (string, string) {
	if secret != nil && secret.IsFree {
		return strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + path, "/backend-anon" + path
	}
	return BaseURL + path, "/backend-api" + path
}

func conversationHeaders(secret *tokens.Secret, chatToken *TurnStile, accept, targetPath, conduitToken, turnTraceID string) httpclient.AuroraHeaders {
	return conversationHeadersWithState(secret, chatToken, accept, targetPath, conduitToken, turnTraceID, nil)
}

func conversationHeadersWithState(secret *tokens.Secret, chatToken *TurnStile, accept, targetPath, conduitToken, turnTraceID string, state *ChatClientState) httpclient.AuroraHeaders {
	header := createBaseHeaderForState(state)
	header.Set("Accept", accept)
	header.Set("Content-Type", "application/json")
	header.Set("X-Openai-Target-Path", targetPath)
	header.Set("X-Openai-Target-Route", targetPath)
	if turnTraceID != "" {
		header.Set("X-Oai-Turn-Trace-Id", turnTraceID)
	}
	if conduitToken != "" || strings.HasSuffix(targetPath, "/f/conversation") || strings.HasSuffix(targetPath, "/f/conversation/prepare") {
		// /f/conversation 与 /f/conversation/prepare 都必须携带 X-Conduit-Token 头。
		// 真实流程:none 状态时送空字符串(服务端会签发新 token),
		// sent/success/conversation 状态时送上一步返回的 token。
		// 不再发送字面量 "no-token" —— 那是旧客户端占位,真实浏览器从不发送。
		header.Set("X-Conduit-Token", conduitToken)
	}
	// /f/conversation(主请求)专属头:心跳/遥测,sentinel token 之外的反作弊画像。
	// prepare 阶段不发这两头。
	if strings.HasSuffix(targetPath, "/f/conversation") && !strings.HasSuffix(targetPath, "/prepare") {
		header.Set("Oai-Echo-Logs", "0,3352,1,4100,0,6435,1,6501,0,6506,1,9918,0,11782,1,11804")
		header.Set("Oai-Telemetry", "[1,null]")
	}
	if chatToken != nil {
		if chatToken.TurnStileToken != "" {
			header.Set("Openai-Sentinel-Chat-Requirements-Token", chatToken.TurnStileToken)
		}
		if chatToken.ProofOfWorkToken != "" {
			header.Set("Openai-Sentinel-Proof-Token", chatToken.ProofOfWorkToken)
		}
		if chatToken.TurnstileToken != "" {
			header.Set("Openai-Sentinel-Turnstile-Token", chatToken.TurnstileToken)
		}
		// openai-sentinel-so-token:对齐 out.js sessionObserverToken() 行为,需要在
		// 首次发请求前触发 snapshot(fire-and-forget collector 必须已经起好)。
		// deviceID 沿用 secret.Token(对应 out.js qn.getCookies()["oai-did"])。
		if soToken := chatToken.ensureSOToken(soDeviceIDFor(secret)); soToken != "" {
			header.Set("Openai-Sentinel-So-Token", soToken)
		}
	}
	cookieStr := ""
	if secret != nil && secret.PUID != "" {
		cookieStr = "_puid=" + secret.PUID
	}
	if secret != nil && secret.IsFree && secret.Token != "" {
		header.Set("Oai-Device-Id", secret.Token)
		// free 用户的 oai-did 也塞进 cookie
		if cookieStr != "" {
			cookieStr += "; "
		}
		cookieStr += "oai-did=" + secret.Token
	}
	if cookieStr != "" {
		header["Cookie"] = cookieStr
	}
	if secret != nil && !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	setTeamAccountHeader(header, secret)
	return header
}

type chatWebsocketURLResponse struct {
	WebsocketURL string `json:"websocket_url"`
}

var chatWebsocketIDCounter int64 = 4

func nextChatWebsocketID() int64 {
	return atomic.AddInt64(&chatWebsocketIDCounter, 1)
}

func getChatWebsocketURL(client httpclient.AuroraHttpClient, secret *tokens.Secret) (string, error) {
	return getChatWebsocketURLWithState(client, secret, nil)
}

func getChatWebsocketURLWithState(client httpclient.AuroraHttpClient, secret *tokens.Secret, state *ChatClientState) (string, error) {
	apiURL, targetPath := conversationURL(secret, "/celsius/ws/user")
	header := conversationHeadersWithState(secret, nil, "*/*", targetPath, "", "", state)
	response, err := client.Request(http.MethodGet, apiURL, header, nil, nil)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("celsius ws user failed: %s", string(body))
	}
	var result chatWebsocketURLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.WebsocketURL == "" {
		return "", fmt.Errorf("celsius ws user missing websocket_url: %s", string(body))
	}
	return result.WebsocketURL, nil
}

func DialChatWebsocket(client httpclient.AuroraHttpClient, secret *tokens.Secret) (*websocket.Conn, error) {
	return DialChatWebsocketWithState(client, secret, nil)
}

func DialChatWebsocketWithState(client httpclient.AuroraHttpClient, secret *tokens.Secret, state *ChatClientState) (*websocket.Conn, error) {
	return DialChatWebsocketWithStateAndProxy(client, secret, state, "")
}

func DialChatWebsocketWithStateAndProxy(client httpclient.AuroraHttpClient, secret *tokens.Secret, state *ChatClientState, proxy string) (*websocket.Conn, error) {
	wsURL, err := getChatWebsocketURLWithState(client, secret, state)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}
	if proxyFunc, err := websocketProxyFunc(proxy); err != nil {
		return nil, err
	} else if proxyFunc != nil {
		dialer.Proxy = proxyFunc
	}
	header := http.Header{}
	ua := defaultUserAgent()
	if state != nil && state.UserAgent != "" {
		ua = state.UserAgent
	}
	header.Set("User-Agent", ua)
	header.Set("Origin", "https://chatgpt.com")
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return nil, err
	}
	initMsg := []map[string]interface{}{
		{"id": 1, "command": map[string]interface{}{
			"type":     "connect",
			"presence": map[string]string{"type": "presence", "state": "background"},
		}},
		{"id": 2, "command": map[string]interface{}{"type": "subscribe", "topic_id": "calpico-chatgpt"}},
		{"id": 3, "command": map[string]interface{}{"type": "subscribe", "topic_id": "conversations"}},
		{"id": 4, "command": map[string]interface{}{"type": "subscribe", "topic_id": "app_notifications"}},
	}
	if err := conn.WriteJSON(initMsg); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func websocketProxyFunc(proxy string) (func(*http.Request) (*url.URL, error), error) {
	if proxy == "" {
		return http.ProxyFromEnvironment, nil
	}
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return nil, err
	}
	return http.ProxyURL(proxyURL), nil
}

func parseChatWebsocketFrames(raw []byte) []map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		var frames []map[string]interface{}
		if err := json.Unmarshal(raw, &frames); err != nil {
			return nil
		}
		return frames
	}
	var frame map[string]interface{}
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil
	}
	return []map[string]interface{}{frame}
}

func chatWebsocketEncodedItem(frame map[string]interface{}, topicID string) string {
	if frame == nil {
		return ""
	}
	if frameTopicID, _ := frame["topic_id"].(string); frameTopicID != "" && frameTopicID != topicID {
		return ""
	}
	payload, ok := frame["payload"].(map[string]interface{})
	if !ok {
		return ""
	}
	nested, ok := payload["payload"].(map[string]interface{})
	if !ok {
		return ""
	}
	encoded, _ := nested["encoded_item"].(string)
	return encoded
}

func chatWebsocketSSEItems(frame map[string]interface{}, topicID string) []string {
	if encoded := chatWebsocketEncodedItem(frame, topicID); encoded != "" {
		return []string{encoded}
	}
	if update := chatWebsocketConversationUpdateItem(frame, topicID); update != "" {
		return []string{update}
	}
	return nil
}

func chatWebsocketConversationUpdateItem(frame map[string]interface{}, topicID string) string {
	if frame == nil {
		return ""
	}
	frameTopicID, _ := frame["topic_id"].(string)
	if frameTopicID != "" && frameTopicID != topicID && frameTopicID != "conversations" {
		return ""
	}
	payload, ok := frame["payload"].(map[string]interface{})
	if !ok {
		return ""
	}
	payloadType, _ := payload["type"].(string)
	if payloadType != "conversation-update" {
		if nested, ok := payload["payload"].(map[string]interface{}); ok {
			if nestedType, _ := nested["type"].(string); nestedType == "conversation-update" {
				payload = nested
				payloadType = nestedType
			}
		}
	}
	if payloadType != "conversation-update" {
		return ""
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return "data: " + string(body) + "\n"
}

func chatWebsocketWriteEncodedItem(writer *io.PipeWriter, encoded string) bool {
	if encoded == "" {
		return false
	}
	if !strings.HasSuffix(encoded, "\n") {
		encoded += "\n"
	}
	_, _ = writer.Write([]byte(encoded))
	return strings.Contains(encoded, "data: [DONE]") || strings.Contains(encoded, "data:[DONE]")
}

func chatWebsocketStreamReader(conn *websocket.Conn, topicID string) (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	subMsg := []map[string]interface{}{
		{"id": nextChatWebsocketID(), "command": map[string]interface{}{
			"type":     "subscribe",
			"topic_id": topicID,
			"offset":   "0",
		}},
	}
	if err := conn.WriteJSON(subMsg); err != nil {
		_ = reader.Close()
		return nil, err
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	})
	go func() {
		defer writer.Close()
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		done := make(chan error, 1)
		go func() {
			for {
				conn.SetReadDeadline(time.Now().Add(120 * time.Second))
				_, raw, err := conn.ReadMessage()
				if err != nil {
					done <- err
					return
				}
				for _, frame := range parseChatWebsocketFrames(raw) {
					frameType, _ := frame["type"].(string)
					if frameType == "reply" {
						reply, _ := frame["reply"].(map[string]interface{})
						replyTopicID, _ := reply["topic_id"].(string)
						if replyTopicID != topicID {
							continue
						}
						catchups, _ := reply["catchups"].([]interface{})
						for _, catchup := range catchups {
							catchupFrame, _ := catchup.(map[string]interface{})
							for _, item := range chatWebsocketSSEItems(catchupFrame, topicID) {
								if chatWebsocketWriteEncodedItem(writer, item) {
									done <- nil
									return
								}
							}
						}
						continue
					}
					if frameType != "message" {
						continue
					}
					for _, item := range chatWebsocketSSEItems(frame, topicID) {
						if chatWebsocketWriteEncodedItem(writer, item) {
							done <- nil
							return
						}
					}
				}
			}
		}()
		for {
			select {
			case <-ticker.C:
				_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			case err := <-done:
				if err != nil {
					_ = writer.CloseWithError(err)
				}
				return
			}
		}
	}()
	return reader, nil
}

func shouldUseWebsocketHandoff(readingWebsocket bool, handoffTopicID string, wsConn *websocket.Conn, text string, imgSource []string) bool {
	if readingWebsocket || handoffTopicID == "" || wsConn == nil {
		return false
	}
	return text == "" && strings.Join(imgSource, "") == ""
}

// PrepareState 表示 /f/conversation/prepare 的客户端状态机:
// none -> sent -> success -> conversation
// 真实浏览器严格按此顺序触发;漏掉任何一阶段都会被服务端识别为非标准客户端,
// 进而把请求路由到 mini 池。
type PrepareState string

const (
	PrepareStateNone    PrepareState = "none"
	PrepareStateSent    PrepareState = "sent"
	PrepareStateSuccess PrepareState = "success"
)

func getConduitToken(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, chatToken *TurnStile, turnTraceID string) (string, error) {
	return getConduitTokenWithState(client, message, secret, chatToken, turnTraceID, nil, PrepareStateNone, "")
}

func getConduitTokenWithState(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, chatToken *TurnStile, turnTraceID string, state *ChatClientState, prepareState PrepareState, previousConduitToken string) (string, error) {
	message = requestWithClientState(message, state)
	apiUrl, targetPath := conversationURL(secret, "/f/conversation/prepare")
	parentMessageID := message.ParentMessageID
	if parentMessageID == "" {
		parentMessageID = "client-created-root"
	}
	payload := map[string]interface{}{
		"action":                 "next",
		"parent_message_id":      parentMessageID,
		"model":                  conversationPrepareModel(message.Model),
		"client_prepare_state":   string(prepareState),
		"timezone_offset_min":    message.TimezoneOffsetMin,
		"timezone":               "America/Los_Angeles",
		"conversation_mode":      map[string]string{"kind": "primary_assistant"},
		"system_hints":           []string{},
		"supports_buffering":     true,
		"supported_encodings":    []string{"v1"},
		"client_contextual_info": conversationPrepareClientContext(message),
	}
	// partial_query 只在 sent / success 阶段携带,none 阶段用户还没开始打字
	if prepareState == PrepareStateSent || prepareState == PrepareStateSuccess {
		payload["partial_query"] = map[string]interface{}{
			"id":      uuid.NewString(),
			"author":  map[string]string{"role": "user"},
			"content": map[string]interface{}{"content_type": "text", "parts": []string{conversationPartialText(message)}},
		}
	}
	if message.ConversationID != "" {
		payload["conversation_id"] = message.ConversationID
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	// 关键:conduit token 在每一步都不同,严格按"上一步响应拿到的 token"作为下一步的请求头
	header := conversationHeadersWithState(secret, chatToken, "*/*", targetPath, previousConduitToken, turnTraceID, state)
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
	return result.ConduitToken, nil
}

func PrepareConversationConduit(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, proxy string, turnTraceID string) (string, error) {
	return PrepareConversationConduitWithState(client, message, secret, proxy, turnTraceID, nil)
}

func PrepareConversationConduitWithState(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, proxy string, turnTraceID string, state *ChatClientState) (string, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	return getConduitTokenWithState(client, message, secret, nil, turnTraceID, state, PrepareStateNone, "")
}

// PrepareConversationConduitFull 走完整的 none -> sent -> success 三态,
// 每次 prepare 都用上一步返回的 conduit_token 作下一步请求头。
// success 状态返回的 token 用于 POST /f/conversation,这是真实浏览器
// 进入"主路由决策"前的最后一步 —— 缺这一步会让后端降级到 mini 池。
func PrepareConversationConduitFull(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, proxy string, turnTraceID string, state *ChatClientState) (string, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	// 在三态 prepare 之前先确保 CookieJar 有 CF 注入的 cf_clearance / __cf_bm
	// 等关键 cookie,否则直接被 CF 拦截,根本到不了 OpenAI 后端。
	ensureBootstrapped(client, secret)
	// Step 1: none —— 用户还没开始打字,partial_query 不带
	token1, err := getConduitTokenWithState(client, message, secret, nil, turnTraceID, state, PrepareStateNone, "")
	if err != nil {
		return "", fmt.Errorf("prepare(none) failed: %w", err)
	}
	// Step 2: sent —— 打字中,带 partial_query
	token2, err := getConduitTokenWithState(client, message, secret, nil, turnTraceID, state, PrepareStateSent, token1)
	if err != nil {
		return "", fmt.Errorf("prepare(sent) failed: %w", err)
	}
	// Step 3: success —— 用户按回车,后端在这一步给出模型路由决策
	token3, err := getConduitTokenWithState(client, message, secret, nil, turnTraceID, state, PrepareStateSuccess, token2)
	if err != nil {
		return "", fmt.Errorf("prepare(success) failed: %w", err)
	}
	return token3, nil
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
				return runeSlice(text, 5)
			}
		}
	}
	return "h"
}

func runeSlice(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) > maxRunes {
		r = r[:maxRunes]
	}
	return string(r)
}

func conversationPrepareClientContext(message chatgpt_types.ChatGPTRequest) map[string]interface{} {
	info := map[string]interface{}{"app_name": "chatgpt.com"}
	for key, value := range message.ClientContextualInfo {
		info[key] = value
	}
	info["app_name"] = "chatgpt.com"
	return info
}

func POSTconversation(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, chat_token *TurnStile, proxy string) (*http.Response, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	turnTraceID := uuid.NewString()
	conduitToken, err := getConduitToken(client, message, secret, nil, turnTraceID)
	if err != nil {
		return nil, err
	}
	return POSTconversationPrepared(client, message, secret, chat_token, proxy, conduitToken, turnTraceID)
}

func POSTconversationPrepared(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, chat_token *TurnStile, proxy string, conduitToken string, turnTraceID string) (*http.Response, error) {
	return POSTconversationPreparedWithState(client, message, secret, chat_token, proxy, conduitToken, turnTraceID, nil)
}

func POSTconversationPreparedWithState(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, chat_token *TurnStile, proxy string, conduitToken string, turnTraceID string, state *ChatClientState) (*http.Response, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	message = requestWithClientState(message, state)
	apiUrl, targetPath := conversationURL(secret, "/f/conversation")
	if API_REVERSE_PROXY != "" {
		apiUrl = API_REVERSE_PROXY
	}
	// JSONify the body and add it to the request
	body_json, err := json.Marshal(message)
	if err != nil {
		return &http.Response{}, err
	}
	header := conversationHeadersWithState(secret, chat_token, "text/event-stream", targetPath, conduitToken, turnTraceID, state)
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

type HandlerResult struct {
	Text              string
	ThinkingText      string
	ConversationID    string
	ParentMessageID   string
	Sentinel          []map[string]interface{}
	ArtifactSignals   []ArtifactSignal
	SandboxArtifacts  []SandboxArtifact
	PDFArtifacts      []PDFArtifact
	GeneratedImageIDs []string
	StopSent          bool
	Continue          *ContinueInfo
	// ToolCalls 在 Tools 模式启用时携带从 <tool_call>{...}</tool_call> 协议
	// 抽取出的工具调用列表。当 len(ToolCalls) > 0 时,FinishReason 为 "tool_calls"。
	ToolCalls []official_types.ToolCall
}

type conversationPatchState struct {
	response chatgpt_types.ChatGPTResponse
	channel  string
}

type conversationStreamEvent struct {
	response       chatgpt_types.ChatGPTResponse
	chunk          *official_types.ChatCompletionChunk
	text           string
	role           string
	conversationID string
	messageID      string
	channel        string
	finishReason   string
	isStop         bool
}

func sseDataPayloads(line string) []string {
	var payloads []string
	for _, part := range strings.Split(strings.TrimRight(line, "\r\n"), "\n") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "data:") {
			continue
		}
		payloads = append(payloads, splitSSEDataPayloads(strings.TrimSpace(strings.TrimPrefix(part, "data:")))...)
	}
	return payloads
}

func splitSSEDataPayloads(payload string) []string {
	var payloads []string
	for {
		payload = strings.TrimSpace(payload)
		if payload == "" {
			return payloads
		}
		if strings.HasPrefix(payload, "data:") {
			payload = strings.TrimSpace(strings.TrimPrefix(payload, "data:"))
			continue
		}
		if strings.HasPrefix(payload, "[DONE]") {
			payloads = append(payloads, "[DONE]")
			payload = payload[len("[DONE]"):]
			continue
		}

		reader := strings.NewReader(payload)
		decoder := json.NewDecoder(reader)
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err == nil {
			payloads = append(payloads, string(raw))
			payload = payload[decoder.InputOffset():]
			continue
		}

		next := strings.Index(payload, "data:")
		if next < 0 {
			return payloads
		}
		if first := strings.TrimSpace(payload[:next]); first != "" {
			payloads = append(payloads, first)
		}
		payload = payload[next:]
	}
}

func sseEventName(line string) (string, bool) {
	for _, part := range strings.Split(strings.TrimRight(line, "\r\n"), "\n") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "event:") {
			return strings.TrimSpace(strings.TrimPrefix(part, "event:")), true
		}
	}
	return "", false
}

func streamHandoffTopicFromPayload(payload string, currentEvent string) (string, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return "", false
	}
	eventType, _ := raw["type"].(string)
	if eventType == "stream_handoff" {
		if topicID := streamHandoffTopicFromEvent(raw); topicID != "" {
			return topicID, true
		}
		return "", true
	}
	if eventType == "server_ste_metadata" || currentEvent == "server_ste_metadata" {
		if topicID := streamHandoffTopicFromMetadata(raw); topicID != "" {
			return topicID, true
		}
		return "", eventType == "server_ste_metadata"
	}
	if eventType == "resume_conversation_token" {
		return "", true
	}
	return "", false
}

func streamHandoffTopicFromEvent(raw map[string]interface{}) string {
	options, ok := raw["options"].([]interface{})
	if !ok {
		return ""
	}
	for _, optionValue := range options {
		option, ok := optionValue.(map[string]interface{})
		if !ok {
			continue
		}
		optionType, _ := option["type"].(string)
		if optionType != "subscribe_ws_topic" {
			continue
		}
		topicID, _ := option["topic_id"].(string)
		return topicID
	}
	return ""
}

func streamHandoffTopicFromMetadata(raw map[string]interface{}) string {
	if turnExchangeID, _ := raw["turn_exchange_id"].(string); turnExchangeID != "" {
		return "conversation-turn-" + turnExchangeID
	}
	metadata, ok := raw["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	if turnExchangeID, _ := metadata["turn_exchange_id"].(string); turnExchangeID != "" {
		return "conversation-turn-" + turnExchangeID
	}
	return ""
}

func parseConversationEvent(line string, state *conversationPatchState, model string) (conversationStreamEvent, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return conversationStreamEvent{}, false
	}

	if chunk, ok := chatCompletionChunkFromRaw(raw, model); ok {
		event := conversationStreamEvent{
			chunk:          &chunk,
			text:           firstChunkContent(chunk),
			role:           firstChunkRole(chunk),
			conversationID: chunk.ConversationID,
			channel:        channelFromValue(raw),
			finishReason:   firstChunkFinishReason(chunk),
		}
		event.isStop = event.finishReason != ""
		return event, true
	}

	var direct chatgpt_types.ChatGPTResponse
	if err := json.Unmarshal([]byte(line), &direct); err == nil && isUsableConversationResponse(direct) {
		channel := channelFromValue(raw)
		state.channel = firstNonEmpty(channel, state.channel)
		return conversationStreamEvent{response: direct, messageID: direct.Message.ID, channel: state.channel}, true
	}

	if response, ok := responseFromValue(raw["v"]); ok {
		state.response = response
		if channel := channelFromValue(raw["v"]); channel != "" {
			state.channel = channel
		}
		return conversationStreamEvent{response: state.response, messageID: state.response.Message.ID, channel: state.channel}, true
	}
	if text, ok := raw["v"].(string); ok && raw["p"] == nil && raw["o"] == nil {
		ensureConversationPatchDefaults(state)
		current, _ := state.response.Message.Content.Parts[0].(string)
		state.response.Message.Content.Parts[0] = current + text
		return conversationStreamEvent{response: state.response, messageID: state.response.Message.ID, channel: state.channel}, true
	}

	if patchPath, ok := raw["p"].(string); ok {
		patchOperation, _ := raw["o"].(string)
		if applyConversationPatch(state, patchPath, patchOperation, raw["v"]) {
			return conversationStreamEvent{response: state.response, messageID: state.response.Message.ID, channel: state.channel}, true
		}
	}

	return conversationStreamEvent{}, false
}

func chatCompletionChunkFromRaw(raw map[string]interface{}, model string) (official_types.ChatCompletionChunk, bool) {
	choices, ok := raw["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return official_types.ChatCompletionChunk{}, false
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return official_types.ChatCompletionChunk{}, false
	}
	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return official_types.ChatCompletionChunk{}, false
	}

	text, _ := delta["content"].(string)
	chunk := official_types.NewChatCompletionChunk(text, model)
	if id, ok := raw["id"].(string); ok && id != "" {
		chunk.ID = id
	}
	if object, ok := raw["object"].(string); ok && object != "" {
		chunk.Object = object
	}
	if created, ok := numberToInt64(raw["created"]); ok {
		chunk.Created = created
	}
	if upstreamModel, ok := raw["model"].(string); ok && upstreamModel != "" {
		chunk.Model = upstreamModel
	}
	if role, ok := delta["role"].(string); ok && role != "" {
		chunk.Choices[0].Delta.Role = role
	}
	if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
		chunk.Choices[0].FinishReason = finishReason
	}
	if conversationID, ok := raw["conversation_id"].(string); ok && conversationID != "" {
		chunk.ConversationID = conversationID
	}
	if sentinel, ok := raw["sentinel"].(map[string]interface{}); ok {
		chunk.Sentinel = sentinel
	}
	return chunk, true
}

func channelFromValue(value interface{}) string {
	switch item := value.(type) {
	case map[string]interface{}:
		if channel, _ := item["channel"].(string); channel != "" {
			return channel
		}
		if delta, ok := item["delta"].(map[string]interface{}); ok {
			if channel, _ := delta["channel"].(string); channel != "" {
				return channel
			}
		}
		if choices, ok := item["choices"].([]interface{}); ok {
			for _, choiceValue := range choices {
				choice, ok := choiceValue.(map[string]interface{})
				if !ok {
					continue
				}
				if channel, _ := choice["channel"].(string); channel != "" {
					return channel
				}
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if channel, _ := delta["channel"].(string); channel != "" {
						return channel
					}
				}
			}
		}
		if message, ok := item["message"].(map[string]interface{}); ok {
			if channel := channelFromValue(message); channel != "" {
				return channel
			}
		}
		if nested, ok := item["v"].(map[string]interface{}); ok {
			if channel := channelFromValue(nested); channel != "" {
				return channel
			}
		}
	}
	return ""
}

func numberToInt64(value interface{}) (int64, bool) {
	switch item := value.(type) {
	case float64:
		return int64(item), true
	case int64:
		return item, true
	case int:
		return int64(item), true
	default:
		return 0, false
	}
}

func firstChunkContent(chunk official_types.ChatCompletionChunk) string {
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

func firstChunkRole(chunk official_types.ChatCompletionChunk) string {
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Role
}

func normalizeOpenAIContentDelta(currentText string, incoming string) string {
	if incoming == "" {
		return ""
	}
	if currentText == "" {
		return incoming
	}
	if strings.HasPrefix(incoming, currentText) {
		return incoming[len(currentText):]
	}
	return incoming
}

func firstStringPart(parts []interface{}) string {
	if len(parts) == 0 {
		return ""
	}
	text, _ := parts[0].(string)
	return text
}

func firstChunkFinishReason(chunk official_types.ChatCompletionChunk) string {
	if len(chunk.Choices) == 0 || chunk.Choices[0].FinishReason == nil {
		return ""
	}
	if reason, ok := chunk.Choices[0].FinishReason.(string); ok {
		return reason
	}
	return fmt.Sprint(chunk.Choices[0].FinishReason)
}

func sentinelsFromResponse(response chatgpt_types.ChatGPTResponse) []map[string]interface{} {
	var raw map[string]interface{}
	data, err := json.Marshal(response)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var sentinel []map[string]interface{}
	collectSentinelsFromValue(raw["sentinel"], &sentinel)
	collectSentinelsFromValue(raw["message"], &sentinel)
	return sentinel
}

func collectSentinelsFromValue(value interface{}, sentinel *[]map[string]interface{}) {
	switch item := value.(type) {
	case map[string]interface{}:
		if event, ok := item["event"].(string); ok && event != "" {
			*sentinel = append(*sentinel, item)
		}
		for _, nested := range item {
			collectSentinelsFromValue(nested, sentinel)
		}
	case []interface{}:
		for _, nested := range item {
			collectSentinelsFromValue(nested, sentinel)
		}
	}
}

func isUsableConversationResponse(response chatgpt_types.ChatGPTResponse) bool {
	return response.Error != nil ||
		response.Message.ID != "" ||
		response.Message.Author.Role != "" ||
		len(response.Message.Content.Parts) > 0 ||
		response.Message.EndTurn != nil
}

func responseFromValue(value interface{}) (chatgpt_types.ChatGPTResponse, bool) {
	if value == nil {
		return chatgpt_types.ChatGPTResponse{}, false
	}
	data, err := json.Marshal(value)
	if err != nil {
		return chatgpt_types.ChatGPTResponse{}, false
	}

	var response chatgpt_types.ChatGPTResponse
	if err := json.Unmarshal(data, &response); err == nil && isUsableConversationResponse(response) {
		return response, true
	}

	var message chatgpt_types.Message
	if err := json.Unmarshal(data, &message); err == nil && (message.ID != "" || message.Author.Role != "" || len(message.Content.Parts) > 0 || message.EndTurn != nil) {
		response.Message = message
		return response, true
	}

	return chatgpt_types.ChatGPTResponse{}, false
}

func applyConversationPatch(state *conversationPatchState, patchPath string, operation string, value interface{}) bool {
	ensureConversationPatchDefaults(state)
	switch {
	case patchPath == "/conversation_id":
		if text, ok := value.(string); ok {
			state.response.ConversationID = text
		}
	case patchPath == "/message":
		if response, ok := responseFromValue(value); ok {
			if response.ConversationID != "" {
				state.response.ConversationID = response.ConversationID
			}
			state.response.Message = response.Message
		}
		if channel := channelFromValue(value); channel != "" {
			state.channel = channel
		}
	case patchPath == "/message/id":
		if text, ok := value.(string); ok {
			state.response.Message.ID = text
		}
	case patchPath == "/message/channel":
		if text, ok := value.(string); ok {
			state.channel = text
		}
	case patchPath == "/message/author/role":
		if text, ok := value.(string); ok {
			state.response.Message.Author.Role = text
		}
	case patchPath == "/message/recipient":
		if text, ok := value.(string); ok {
			state.response.Message.Recipient = text
		}
	case patchPath == "/message/content/content_type":
		if text, ok := value.(string); ok {
			state.response.Message.Content.ContentType = text
		}
	case patchPath == "/message/content/parts":
		if parts, ok := value.([]interface{}); ok {
			state.response.Message.Content.Parts = parts
		}
	case strings.HasPrefix(patchPath, "/message/content/parts/0"):
		if text, ok := value.(string); ok {
			current, _ := state.response.Message.Content.Parts[0].(string)
			if operation == "append" {
				text = current + text
			}
			state.response.Message.Content.Parts[0] = text
		}
	case patchPath == "/message/metadata/message_type":
		if text, ok := value.(string); ok {
			state.response.Message.Metadata.MessageType = text
		}
	case patchPath == "/message/metadata/model_slug":
		if text, ok := value.(string); ok {
			state.response.Message.Metadata.ModelSlug = text
		}
	case patchPath == "/message/metadata/finish_details":
		if value == nil {
			state.response.Message.Metadata.FinishDetails = nil
			break
		}
		data, err := json.Marshal(value)
		if err != nil {
			break
		}
		var finishDetails chatgpt_types.FinishDetails
		if json.Unmarshal(data, &finishDetails) == nil {
			state.response.Message.Metadata.FinishDetails = &finishDetails
		}
	case patchPath == "/message/end_turn":
		state.response.Message.EndTurn = value
	default:
		return false
	}
	return true
}

func ensureConversationPatchDefaults(state *conversationPatchState) {
	if state.response.Message.Author.Role == "" {
		state.response.Message.Author.Role = "assistant"
	}
	if state.response.Message.Recipient == "" {
		state.response.Message.Recipient = "all"
	}
	if state.response.Message.Content.ContentType == "" {
		state.response.Message.Content.ContentType = "text"
	}
	if state.response.Message.Content.Parts == nil {
		state.response.Message.Content.Parts = []interface{}{""}
	}
	if state.response.Message.Metadata.MessageType == "" {
		state.response.Message.Metadata.MessageType = "next"
	}
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
	header.Set("User-Agent", defaultUserAgent())
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
	header.Set("User-Agent", defaultUserAgent())
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
	header.Set("User-Agent", defaultUserAgent())
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
	return imageConversationHeadersWithState(secret, turnStile, conduitToken, accept, nil)
}

func imageConversationHeadersWithState(secret *tokens.Secret, turnStile *TurnStile, conduitToken, accept string, state *ChatClientState) httpclient.AuroraHeaders {
	header := createBaseHeaderForState(state)
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

func prepareImageConversation(client httpclient.AuroraHttpClient, secret *tokens.Secret, turnStile *TurnStile, prompt, model string, state *ChatClientState) (string, error) {
	parentMessageID := "client-created-root"
	if state != nil && state.ParentMessageID != "" {
		parentMessageID = state.ParentMessageID
	}
	payload := map[string]interface{}{
		"action":                "next",
		"fork_from_shared_post": false,
		"parent_message_id":     parentMessageID,
		"model":                 imageModelSlug(model),
		"client_prepare_state":  "success",
		"timezone_offset_min":   420,
		"timezone":              "America/Los_Angeles",
		"conversation_mode":     map[string]string{"kind": "primary_assistant"},
		"system_hints":          []string{"picture_v2"},
		"partial_query": map[string]interface{}{
			"id":      uuid.NewString(),
			"author":  map[string]string{"role": "user"},
			"content": map[string]interface{}{"content_type": "text", "parts": []string{prompt}},
		},
		"supports_buffering":     true,
		"supported_encodings":    []string{"v1"},
		"client_contextual_info": state.ClientContextualInfo(),
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	response, err := client.Request(http.MethodPost, BaseURL+"/f/conversation/prepare", imageConversationHeadersWithState(secret, turnStile, "", "*/*", state), nil, bytes.NewReader(bodyJSON))
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
	state := NewChatClientState()
	conduitToken, err := prepareImageConversation(client, secret, turnStile, prompt, model, state)
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
		"parent_message_id":                    state.ParentMessageID,
		"model":                                imageModelSlug(model),
		"client_prepare_state":                 "sent",
		"timezone_offset_min":                  420,
		"timezone":                             "America/Los_Angeles",
		"conversation_mode":                    map[string]string{"kind": "primary_assistant"},
		"enable_message_followups":             true,
		"system_hints":                         []string{"picture_v2"},
		"supports_buffering":                   true,
		"supported_encodings":                  []string{"v1"},
		"client_contextual_info":               state.ClientContextualInfo(),
		"paragen_cot_summary_display_override": "allow",
		"force_parallel_switch":                "auto",
		"thinking_effort":                      "standard",
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	response, err := client.Request(http.MethodPost, BaseURL+"/f/conversation", imageConversationHeadersWithState(secret, turnStile, conduitToken, "text/event-stream", state), nil, bytes.NewReader(bodyJSON))
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

// ImageEditReference 表示已经上传到 ChatGPT 文件服务的一张源图,
// 用于构造 /f/conversation 时的 image_asset_pointer 部件。
type ImageEditReference struct {
	FileID   string
	Width    int
	Height   int
	Size     int
	MimeType string
	Filename string
}

// GeneratePictureConversationImagesWithReferences 在原有文生图流程基础上支持
// 携带已上传的源图(image_asset_pointer + attachments)进入对话,
// 用于实现 OpenAI 兼容的 /v1/images/edits 和 /v1/images/variations。
// 当 references 为空时,行为等价于 GeneratePictureConversationImages。
func GeneratePictureConversationImagesWithReferences(client httpclient.AuroraHttpClient, secret *tokens.Secret, turnStile *TurnStile, prompt, model, proxy string, references []ImageEditReference) ([]ImageGenerationResult, string, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	state := NewChatClientState()
	conduitToken, err := prepareImageConversation(client, secret, turnStile, prompt, model, state)
	if err != nil {
		return nil, "", err
	}

	// 组装 message.parts:每个 reference -> image_asset_pointer,然后追加 prompt 文本
	parts := make([]interface{}, 0, len(references)+1)
	attachments := make([]map[string]interface{}, 0, len(references))
	for _, ref := range references {
		if ref.FileID == "" {
			continue
		}
		part := map[string]interface{}{
			"content_type":  "image_asset_pointer",
			"asset_pointer": "file-service://" + ref.FileID,
		}
		if ref.Width > 0 {
			part["width"] = ref.Width
		}
		if ref.Height > 0 {
			part["height"] = ref.Height
		}
		if ref.Size > 0 {
			part["size_bytes"] = ref.Size
		}
		parts = append(parts, part)

		attachment := map[string]interface{}{
			"id":     ref.FileID,
			"size":   ref.Size,
			"name":   ref.Filename,
			"mime":   ref.MimeType,
			"mimeType": ref.MimeType,
			"source": "library",
		}
		if ref.Width > 0 {
			attachment["width"] = ref.Width
		}
		if ref.Height > 0 {
			attachment["height"] = ref.Height
		}
		attachments = append(attachments, attachment)
	}
	if prompt != "" {
		parts = append(parts, prompt)
	}

	var content map[string]interface{}
	if len(parts) == 0 {
		content = map[string]interface{}{"content_type": "text", "parts": []string{prompt}}
	} else {
		content = map[string]interface{}{"content_type": "multimodal_text", "parts": parts}
	}

	metadata := map[string]interface{}{
		"developer_mode_connector_ids": []interface{}{},
		"selected_github_repos":        []interface{}{},
		"selected_all_github_repos":    false,
		"system_hints":                 []string{"picture_v2"},
		"serialization_metadata":       map[string]interface{}{"custom_symbol_offsets": []interface{}{}},
	}
	if len(attachments) > 0 {
		metadata["attachments"] = attachments
	}

	payload := map[string]interface{}{
		"action": "next",
		"messages": []map[string]interface{}{
			{
				"id":          uuid.NewString(),
				"author":      map[string]string{"role": "user"},
				"create_time": time.Now().Unix(),
				"content":     content,
				"metadata":    metadata,
			},
		},
		"parent_message_id":                    state.ParentMessageID,
		"model":                                imageModelSlug(model),
		"client_prepare_state":                 "sent",
		"timezone_offset_min":                  420,
		"timezone":                             "America/Los_Angeles",
		"conversation_mode":                    map[string]string{"kind": "primary_assistant"},
		"enable_message_followups":             true,
		"system_hints":                         []string{"picture_v2"},
		"supports_buffering":                   true,
		"supported_encodings":                  []string{"v1"},
		"client_contextual_info":               state.ClientContextualInfo(),
		"paragen_cot_summary_display_override": "allow",
		"force_parallel_switch":                "auto",
		"thinking_effort":                      "standard",
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	response, err := client.Request(http.MethodPost, BaseURL+"/f/conversation", imageConversationHeadersWithState(secret, turnStile, conduitToken, "text/event-stream", state), nil, bytes.NewReader(bodyJSON))
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
	result := HandlerDetailed(c, response, client, secret, uuid, translated_request, stream, model)
	return result.Text, result.Continue
}

func HandlerDetailed(c *gin.Context, response *http.Response, client httpclient.AuroraHttpClient, secret *tokens.Secret, uuid string, translated_request chatgpt_types.ChatGPTRequest, stream bool, model string) HandlerResult {
	return HandlerDetailedWithWebsocket(c, response, client, secret, uuid, translated_request, stream, model, nil)
}

type HandlerDetailedOptions struct {
	Websocket        *websocket.Conn
	ClientState      *ChatClientState
	ArtifactDelivery string
	ProxyURL         string
	// Tools 启用工具调用解析:设置后,HandlerDetailedWithOptions 会把
	// 累积的 text 喂给 toolcall.Parser,把 <tool_call>{...}</tool_call>
	// 切成 OpenAI delta.tool_calls 流式 chunk,并在 HandlerResult.ToolCalls
	// 中返回完整调用列表(用于多轮工具调用循环)。
	// 为空时保持原行为不变(向后兼容)。
	Tools []official_types.Tool
}

func HandlerDetailedWithWebsocket(c *gin.Context, response *http.Response, client httpclient.AuroraHttpClient, secret *tokens.Secret, uuid string, translated_request chatgpt_types.ChatGPTRequest, stream bool, model string, wsConn *websocket.Conn) HandlerResult {
	return HandlerDetailedWithOptions(c, response, client, secret, uuid, translated_request, stream, model, HandlerDetailedOptions{Websocket: wsConn})
}

func HandlerDetailedWithOptions(c *gin.Context, response *http.Response, client httpclient.AuroraHttpClient, secret *tokens.Secret, uuid string, translated_request chatgpt_types.ChatGPTRequest, stream bool, model string, options HandlerDetailedOptions) HandlerResult {
	if model == "" {
		model = translated_request.Model
	}
	wsConn := options.Websocket
	if options.ClientState != nil {
		options.ClientState.ApplyToRequest(&translated_request)
	}
	max_tokens := false

	// Create a bufio.Reader from the response body
	reader := bufio.NewReader(response.Body)
	if stream && client != nil && secret != nil {
		if wsConn == nil {
			if conn, err := DialChatWebsocketWithStateAndProxy(client, secret, options.ClientState, options.ProxyURL); err == nil {
				wsConn = conn
				defer wsConn.Close()
			}
		} else {
			defer wsConn.Close()
		}
	}

	// Read the response byte by byte until a newline character is encountered
	if stream {
		// Response content type is text/event-stream
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
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
	var sentinel []map[string]interface{}
	var thinkingText string
	var activeChannel string
	var assistantMessageID string
	artifactState := newArtifactAccumulator()
	artifactConfig := ArtifactStreamConfig{Delivery: options.ArtifactDelivery}
	var patchState conversationPatchState
	var handoffTopicID string
	var currentEvent string
	var readingWebsocket bool
	var websocketStream io.ReadCloser
	emitSentinels := func(items []map[string]interface{}) {
		if len(items) == 0 {
			return
		}
		sentinel = append(sentinel, items...)
		if !stream {
			return
		}
		for _, item := range items {
			chunk := official_types.NewChatCompletionChunk("", model)
			if convId != "" {
				chunk.ConversationID = convId
			}
			chunk.Sentinel = item
			c.Writer.WriteString("data: " + chunk.String() + "\n\n")
			c.Writer.Flush()
		}
	}
	observeArtifacts := func(line string) {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return
		}
		if cid := firstConversationID(raw); cid != "" && convId == "" {
			convId = cid
		}
		events := artifactState.ObserveRaw(raw, convId)
		emitSentinels(materializeArtifactEvents(client, secret, convId, events, artifactConfig))
		if artifactState.LastAssistantMsgID != "" {
			assistantMessageID = artifactState.LastAssistantMsgID
		}
		if artifactState.ConversationID != "" && convId == "" {
			convId = artifactState.ConversationID
		}
	}
	emitThinking := func(delta string) {
		if delta == "" {
			return
		}
		thinkingText += delta
		emitSentinels([]map[string]interface{}{{
			"event": "thinking",
			"kind":  "analysis",
			"delta": delta,
		}})
	}
	finalizeArtifacts := func() {
		emitSentinels(materializeArtifactEvents(client, secret, convId, artifactState.Finalize(), artifactConfig))
	}
readLoop:
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				break
			}
			if err != io.EOF {
				return HandlerResult{}
			}
		}
		if eventName, ok := sseEventName(line); ok {
			currentEvent = eventName
		}
		for _, line := range sseDataPayloads(line) {
			// Check if line starts with [DONE]
			if strings.HasPrefix(line, "[DONE]") {
				if shouldUseWebsocketHandoff(readingWebsocket, handoffTopicID, wsConn, previous_text.Text, imgSource) {
					wsReader, err := chatWebsocketStreamReader(wsConn, handoffTopicID)
					if err == nil {
						websocketStream = wsReader
						defer websocketStream.Close()
						reader = bufio.NewReader(wsReader)
						readingWebsocket = true
						currentEvent = ""
						continue readLoop
					}
				}
				finalizeArtifacts()
				break readLoop
			}
			observeArtifacts(line)
			if topicID, skip := streamHandoffTopicFromPayload(line, currentEvent); skip {
				if topicID != "" {
					handoffTopicID = topicID
				}
				currentEvent = ""
				continue
			}
			// Parse the line as JSON
			streamEvent, ok := parseConversationEvent(line, &patchState, model)
			if !ok {
				currentEvent = ""
				continue
			}
			if streamEvent.chunk != nil {
				if streamEvent.conversationID != "" {
					convId = streamEvent.conversationID
				}
				if streamEvent.chunk.Sentinel != nil {
					sentinel = append(sentinel, streamEvent.chunk.Sentinel)
				}
				deltaText := normalizeOpenAIContentDelta(previous_text.Text, streamEvent.text)
				if streamEvent.channel != "" {
					activeChannel = streamEvent.channel
				}
				if streamEvent.finishReason != "" {
					finish_reason = streamEvent.finishReason
					if finish_reason == "length" {
						max_tokens = true
					}
					isEnd = true
				}
				if activeChannel == "analysis" {
					emitThinking(streamEvent.text)
					if streamEvent.isStop {
						if stream {
							finalLine := official_types.StopChunkWithConversation(finish_reason, model, convId)
							c.Writer.WriteString("data: " + finalLine.String() + "\n\n")
							c.Writer.Flush()
						}
						if max_tokens && convId != "" && assistantMessageID != "" {
							finalizeArtifacts()
							return HandlerResult{
								Text:              strings.Join(imgSource, "") + previous_text.Text,
								ThinkingText:      thinkingText,
								ConversationID:    convId,
								ParentMessageID:   assistantMessageID,
								Sentinel:          sentinel,
								ArtifactSignals:   artifactState.Signals,
								SandboxArtifacts:  artifactState.SandboxArtifacts,
								PDFArtifacts:      artifactState.PDFArtifacts,
								GeneratedImageIDs: artifactState.ImageFileIDs,
								StopSent:          true,
								Continue: &ContinueInfo{
									ConversationID: convId,
									ParentID:       assistantMessageID,
								},
							}
						}
						finalizeArtifacts()
						return HandlerResult{
							Text:              strings.Join(imgSource, "") + previous_text.Text,
							ThinkingText:      thinkingText,
							ConversationID:    convId,
							ParentMessageID:   assistantMessageID,
							Sentinel:          sentinel,
							ArtifactSignals:   artifactState.Signals,
							SandboxArtifacts:  artifactState.SandboxArtifacts,
							PDFArtifacts:      artifactState.PDFArtifacts,
							GeneratedImageIDs: artifactState.ImageFileIDs,
							StopSent:          true,
						}
					}
					currentEvent = ""
					continue
				}
				if stream {
					outChunk := *streamEvent.chunk
					if len(outChunk.Choices) > 0 {
						outChunk.Choices[0].Delta.Content = deltaText
						if streamEvent.role == "" || !isRole {
							outChunk.Choices[0].Delta.Role = ""
						}
					}
					if streamEvent.isStop && outChunk.ConversationID == "" {
						outChunk.ConversationID = convId
					}
					shouldWrite := deltaText != "" ||
						(streamEvent.role != "" && isRole) ||
						streamEvent.chunk.Sentinel != nil ||
						streamEvent.isStop
					if shouldWrite {
						c.Writer.WriteString("data: " + outChunk.String() + "\n\n")
						c.Writer.Flush()
					}
					if streamEvent.role != "" && isRole {
						isRole = false
					}
				}
				if deltaText != "" {
					previous_text.Text += deltaText
				}
				if streamEvent.isStop {
					if max_tokens && convId != "" && assistantMessageID != "" {
						finalizeArtifacts()
						return HandlerResult{
							Text:              strings.Join(imgSource, "") + previous_text.Text,
							ThinkingText:      thinkingText,
							ConversationID:    convId,
							ParentMessageID:   assistantMessageID,
							Sentinel:          sentinel,
							ArtifactSignals:   artifactState.Signals,
							SandboxArtifacts:  artifactState.SandboxArtifacts,
							PDFArtifacts:      artifactState.PDFArtifacts,
							GeneratedImageIDs: artifactState.ImageFileIDs,
							StopSent:          true,
							Continue: &ContinueInfo{
								ConversationID: convId,
								ParentID:       assistantMessageID,
							},
						}
					}
					finalizeArtifacts()
					return HandlerResult{
						Text:              strings.Join(imgSource, "") + previous_text.Text,
						ThinkingText:      thinkingText,
						ConversationID:    convId,
						ParentMessageID:   assistantMessageID,
						Sentinel:          sentinel,
						ArtifactSignals:   artifactState.Signals,
						SandboxArtifacts:  artifactState.SandboxArtifacts,
						PDFArtifacts:      artifactState.PDFArtifacts,
						GeneratedImageIDs: artifactState.ImageFileIDs,
						StopSent:          true,
					}
				}
				currentEvent = ""
				continue
			}
			original_response = streamEvent.response
			if original_response.Error != nil {
				c.JSON(500, gin.H{"error": original_response.Error})
				return HandlerResult{}
			}
			sentinel = append(sentinel, sentinelsFromResponse(original_response)...)
			if original_response.ConversationID != convId {
				if convId == "" {
					convId = original_response.ConversationID
				} else {
					continue
				}
			}
			if streamEvent.channel != "" {
				activeChannel = streamEvent.channel
			}
			if original_response.Message.ID != "" && (original_response.Message.Author.Role == "assistant" || original_response.Message.Author.Role == "tool") {
				assistantMessageID = original_response.Message.ID
			}
			if activeChannel == "analysis" {
				thinkingDelta := normalizeOpenAIContentDelta(thinkingText, firstStringPart(original_response.Message.Content.Parts))
				emitThinking(thinkingDelta)
				currentEvent = ""
				continue
			}
			if !(original_response.Message.Author.Role == "assistant" || (original_response.Message.Author.Role == "tool" && original_response.Message.Content.ContentType != "text")) || original_response.Message.Content.Parts == nil {
				continue
			}
			if original_response.Message.Metadata.MessageType == "" && activeChannel != "final" {
				continue
			}
			if (original_response.Message.Metadata.MessageType != "next" && original_response.Message.Metadata.MessageType != "continue" && activeChannel != "final") || !strings.HasSuffix(original_response.Message.Content.ContentType, "text") {
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
					return HandlerResult{}
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
					final_line := official_types.StopChunkWithConversation(finish_reason, model, convId)
					c.Writer.WriteString("data: " + final_line.String() + "\n\n")
					c.Writer.Flush()
				}
				finalizeArtifacts()
				return HandlerResult{
					Text:              strings.Join(imgSource, "") + previous_text.Text,
					ThinkingText:      thinkingText,
					ConversationID:    convId,
					ParentMessageID:   assistantMessageID,
					Sentinel:          sentinel,
					ArtifactSignals:   artifactState.Signals,
					SandboxArtifacts:  artifactState.SandboxArtifacts,
					PDFArtifacts:      artifactState.PDFArtifacts,
					GeneratedImageIDs: artifactState.ImageFileIDs,
					StopSent:          stream,
				}
			}
			currentEvent = ""
		}
		if err == io.EOF {
			break
		}
	}
	if !max_tokens {
		finalizeArtifacts()
		return HandlerResult{
			Text:              strings.Join(imgSource, "") + previous_text.Text,
			ThinkingText:      thinkingText,
			ConversationID:    convId,
			ParentMessageID:   assistantMessageID,
			Sentinel:          sentinel,
			ArtifactSignals:   artifactState.Signals,
			SandboxArtifacts:  artifactState.SandboxArtifacts,
			PDFArtifacts:      artifactState.PDFArtifacts,
			GeneratedImageIDs: artifactState.ImageFileIDs,
		}
	}
	finalizeArtifacts()
	return HandlerResult{
		Text:              strings.Join(imgSource, "") + previous_text.Text,
		ThinkingText:      thinkingText,
		ConversationID:    convId,
		ParentMessageID:   assistantMessageID,
		Sentinel:          sentinel,
		ArtifactSignals:   artifactState.Signals,
		SandboxArtifacts:  artifactState.SandboxArtifacts,
		PDFArtifacts:      artifactState.PDFArtifacts,
		GeneratedImageIDs: artifactState.ImageFileIDs,
		Continue: &ContinueInfo{
			ConversationID: original_response.ConversationID,
			ParentID:       original_response.Message.ID,
		},
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
	header.Set("Authority", "auth.openai.com")
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
	header.Set("Authority", "chat.openai.com")
	header.Set("Accept-Language", "en-US,en;q=0.9")
	header.Set("User-Agent", defaultUserAgent())
	header.Set("Accept", "*/*")
	header.Set("Oai-Language", "en-US")
	header.Set("Origin", "https://chatgpt.com")
	header.Set("Referer", "https://chatgpt.com/")
	header.Set("Cookie", "__Secure-next-auth.session-token="+session_token)
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
	return createBaseHeaderForState(nil)
}

func createBaseHeaderForState(state *ChatClientState) httpclient.AuroraHeaders {
	header := make(httpclient.AuroraHeaders)
	// 对齐 conversation.txt 2026-06 抓包:Chrome 148 Win64 英文浏览器
	header.Set("Accept", "*/*")
	header.Set("Accept-Language", "en-US,en;q=0.9")
	header.Set("Oai-Language", "en-US")
	header.Set("Origin", "https://chatgpt.com")
	// referer 跟 state.ConversationID 联动;空就发首页
	if state != nil && state.ConversationID != "" {
		header.Set("Referer", "https://chatgpt.com/c/"+state.ConversationID)
	} else {
		header.Set("Referer", "https://chatgpt.com/")
	}
	// sec-ch-ua-* 对齐 Chrome 148 (与 UA / prooftoken 同步)
	header.Set("Sec-Ch-Ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	header.Set("Sec-Ch-Ua-Arch", `"x86"`)
	header.Set("Sec-Ch-Ua-Bitness", `"64"`)
	header.Set("Sec-Ch-Ua-Full-Version", `"148.0.7778.98"`)
	header.Set("Sec-Ch-Ua-Full-Version-List", `"Chromium";v="148.0.7778.98", "Google Chrome";v="148.0.7778.98", "Not/A)Brand";v="99.0.0.0"`)
	header.Set("Sec-Ch-Ua-Mobile", "?0")
	header.Set("Sec-Ch-Ua-Model", `""`)
	header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	// Sec-Ch-Ua-Platform-Version 报 "10.0.0" (Windows 10) — 与 UA 报 Windows NT 10.0 保持一致,
	// 否则 UA/platform-version 交叉对不上会被 Cloudflare 标记。
	header.Set("Sec-Ch-Ua-Platform-Version", `"15.0.0"`)
	header.Set("Priority", "u=1, i")
	header.Set("Sec-Fetch-Dest", "empty")
	header.Set("Sec-Fetch-Mode", "cors")
	header.Set("Sec-Fetch-Site", "same-origin")
	ua := util.FixedUserAgent
	if state != nil && state.UserAgent != "" {
		ua = state.UserAgent
	}
	header.Set("User-Agent", ua)
	deviceID := oaiDeviceID
	sessionID := oaiSessionID
	if state != nil {
		if state.DeviceID != "" {
			deviceID = state.DeviceID
		}
		if state.SessionID != "" {
			sessionID = state.SessionID
		}
	}
	header.Set("Oai-Device-Id", deviceID)
	header.Set("Oai-Session-Id", sessionID)
	// 对齐 conversation.txt 2026-06 抓包的 build / version
	header.Set("Oai-Client-Version", "prod-497f333866796e100096ad083b51ca949d22e751")
	header.Set("Oai-Client-Build-Number", "7646290")
	return header
}

// defaultUserAgent 返回全局统一的 User-Agent (Chrome 148 Windows)。
// 一律走 util.FixedUserAgent,不再随机 —
//  1. 网络 header 用途: 防止与 sec-ch-ua-* 失配触发 Cloudflare 风控;
//  2. fingerprint/PoW 用途: 内部算 token 用的 UA 必须跟实际请求一致,
//     随机会让 prooftoken 跟真实 UA 错位导致 sentinel 验证失败。
func defaultUserAgent() string {
	return util.FixedUserAgent
}

func HandlerTTS(response *http.Response, input string) (string, string) {
	reader := bufio.NewReader(response.Body)

	var convId string
	var fallbackMsgID string
	var patchState conversationPatchState

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
		for _, payload := range sseDataPayloads(line) {
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

func getTTSBlobFromURL(client httpclient.AuroraHttpClient, secret *tokens.Secret, reqURL string) ([]byte, int, error) {
	header := createBaseHeader()
	header.Set("Accept", "audio/*,*/*")
	if !secret.IsFree && secret.Token != "" {
		header.Set("Authorization", "Bearer "+secret.Token)
	}
	if secret.IsFree {
		header.Set("Oai-Device-Id", secret.Token)
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
		header.Set("Oai-Device-Id", secret.Token)
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

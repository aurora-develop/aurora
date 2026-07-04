package chatgpt

import (
	"aurora/conversion/response/chatgpt"
	"aurora/httpclient"
	"aurora/internal/browserfp"
	"aurora/internal/fingerprint"
	"aurora/internal/prooftoken"
	"aurora/internal/so"
	"aurora/internal/sseparser"
	"aurora/internal/tokens"
	"aurora/internal/turnstile"
	"aurora/typings"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
	"aurora/util"
	"bufio"
	"bytes"
	"encoding/base64"
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
	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/websocket"
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
	TurnStileToken              string
	ProofOfWorkToken            string
	TurnstileToken              string
	ChatRequirementsPrepareToken string // prepare 接口返回的 prepare_token, 在 sentinel/ping 时注入
	ChatRequirementsToken       string // finalize 接口返回的 chat-requirements token (sentinel/ping 复用)
	SentinelReqToken            string // /sentinel/req 返回的 token
	SentinelReqPersona          string // /sentinel/req 返回的 persona
	SOToken                     string
	soSession                   *so.Session
	soSnapshotDX                string
	soChatToken                 string
	soFlow                      string
	soOnce                      sync.Once
	soResult                    string
	soErr                       error
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

	// 构建 TurnStile (先于 finalize)
	ts := &TurnStile{
		ProofOfWorkToken:            proofToken,
		TurnstileToken:              turnstileToken,
		ChatRequirementsPrepareToken: prepare.PrepareToken,
	}

	// so 段
	if prepare.So.Required && prepare.So.CollectorDX != "" && prepare.So.SnapshotDX != "" && prepare.Token != "" {
		ts.soSession = so.NewSession(requirementsToken, prepare.So.CollectorDX)
		ts.soSnapshotDX = prepare.So.SnapshotDX
		ts.soChatToken = prepare.Token
		ts.soFlow = stateFlow(state, ua)
		ts.soSession.Start()
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

	ts.TurnStileToken = finalize.Token
	ts.ChatRequirementsToken = finalize.Token

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

// sentinelExtraData 对齐 chatgpt.com JS 中 rHn() 编码的 OpenAI-Sentinel-Extra-Data header。
// base64(JSON) 格式,携带 conversation ID / 消息 ID 和 token 存在信号。
type sentinelExtraData struct {
	Version        int                     `json:"v"`
	SequenceNumber int                     `json:"sequence_number"`
	Signals        sentinelExtraSignals     `json:"signals"`
	ConversationID string                  `json:"conversation_id,omitempty"`
	LastMessageID  string                  `json:"last_message_id,omitempty"`
}

type sentinelExtraSignals struct {
	PingSource                  string `json:"ping_source"`
	SOTokenPresent              string `json:"so_token_present"`
	TurnstileTokenPresent       string `json:"turnstile_token_present"`
	ProofTokenPresent           string `json:"proof_token_present"`
	PrepareTokenPresent         string `json:"prepare_token_present"`
	ChatRequirementsTokenPresent string `json:"chat_requirements_token_present"`
}

func buildSentinelExtraData(conversationID, lastMessageID string, prepareToken string, chatRequirementsToken string, soTokenPresent bool, turnstileTokenPresent bool, proofTokenPresent bool, pingSource string, sequenceNumber int) string {
	if pingSource == "" {
		pingSource = "session_observer_background_submit"
	}
	signals := sentinelExtraSignals{
		PingSource:                  pingSource,
		SOTokenPresent:              boolToStr(soTokenPresent),
		TurnstileTokenPresent:       boolToStr(turnstileTokenPresent),
		ProofTokenPresent:           boolToStr(proofTokenPresent),
		PrepareTokenPresent:         boolToStr(prepareToken != ""),
		ChatRequirementsTokenPresent: boolToStr(chatRequirementsToken != ""),
	}
	data := sentinelExtraData{
		Version:        1,
		SequenceNumber: sequenceNumber,
		Signals:        signals,
	}
	if conversationID != "" {
		data.ConversationID = "WEB:" + conversationID
	}
	if lastMessageID != "" {
		data.LastMessageID = lastMessageID
	}
	payload, _ := json.Marshal(data)
	return base64.StdEncoding.EncodeToString(payload)
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// pingSentinelResponse 是 POST /backend-api/sentinel/ping 的响应。
type pingSentinelResponse struct {
	Status string `json:"status"`
}

// POSTSentinelPing 调用 /sentinel/ping 端点 — 对齐浏览器端在对话进行中
// 发送的风控汇报请求。携带所有已计算的 sentinel token + Extra-Data。
//
// 对齐 2026-06-24 抓包: ping 在对话上下文中发送,携带 conversation_id 与 last_message_id。
// 两种 ping_source:
//   - "session_observer_background_submit" (seq 0): 后台 SO token 提交
//   - "conversation_heartbeat" (seq 1): 对话心跳
//
// conversationID 不带 "WEB:" 前缀 (函数内补)。
func POSTSentinelPing(client httpclient.AuroraHttpClient, secret *tokens.Secret, ts *TurnStile, conversationID, lastMessageID string, state *ChatClientState) error {
	return POSTSentinelPingWithSource(client, secret, ts, conversationID, lastMessageID, state, "session_observer_background_submit", 0)
}

// POSTSentinelPingWithSource 支持 ping_source 和 sequence_number 自定义。
func POSTSentinelPingWithSource(client httpclient.AuroraHttpClient, secret *tokens.Secret, ts *TurnStile, conversationID, lastMessageID string, state *ChatClientState, pingSource string, sequenceNumber int) error {
	apiUrl, targetPath := sentinelURL(secret, "/sentinel/ping")
	header := sentinelHeaderWithState(secret, targetPath, state)
	// 注入所有 sentinel token header
	if ts != nil {
		if ts.ChatRequirementsPrepareToken != "" {
			header.Set("Openai-Sentinel-Chat-Requirements-Prepare-Token", ts.ChatRequirementsPrepareToken)
		}
		if ts.ChatRequirementsToken != "" {
			header.Set("Openai-Sentinel-Chat-Requirements-Token", ts.ChatRequirementsToken)
		} else if ts.TurnStileToken != "" {
			header.Set("Openai-Sentinel-Chat-Requirements-Token", ts.TurnStileToken)
		}
		if ts.TurnstileToken != "" {
			header.Set("Openai-Sentinel-Turnstile-Token", ts.TurnstileToken)
		}
		if ts.ProofOfWorkToken != "" {
			header.Set("Openai-Sentinel-Proof-Token", ts.ProofOfWorkToken)
		}
		if soToken := ts.ensureSOToken(soDeviceIDFor(secret)); soToken != "" {
			header.Set("Openai-Sentinel-So-Token", soToken)
		}
		extraData := buildSentinelExtraData(
			conversationID,
			lastMessageID,
			ts.ChatRequirementsPrepareToken,
			ts.ChatRequirementsToken,
			ts.ensureSOToken(soDeviceIDFor(secret)) != "",
			ts.TurnstileToken != "",
			ts.ProofOfWorkToken != "",
			pingSource,
			sequenceNumber,
		)
		header.Set("Openai-Sentinel-Extra-Data", extraData)
	}
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, nil)
	if err != nil {
		return fmt.Errorf("sentinel ping failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("sentinel ping failed: %s", readResponseSnippet(response.Body, 500))
	}
	return nil
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

// conversationInitResponse 是 POST /conversation/init 的响应。
// 对齐浏览器 2026-06 chatgpt.com 抓包。
type conversationInitResponse struct {
	Type              string `json:"type"`
	BannerInfo        any    `json:"banner_info"`
	DefaultModelSlug  string `json:"default_model_slug"`
	AtlasModeEnabled  any    `json:"atlas_mode_enabled"`
}

// POSTConversationInit 调用 /conversation/init 端点 — 对齐浏览器行为:
// 在 sentinel 流程完成后调用,获取对话元数据(default_model_slug, limits 等)。
// 浏览器在页面加载时调用此 API 以建立会话上下文。
func POSTConversationInit(client httpclient.AuroraHttpClient, secret *tokens.Secret, state *ChatClientState) (*conversationInitResponse, error) {
	// free 用户走 backend-anon,paid 走 backend-api
	var apiUrl string
	if secret != nil && secret.IsFree {
		apiUrl = strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + "/conversation/init"
	} else {
		apiUrl = BaseURL + "/conversation/init"
	}
	targetPath := "/backend-api/conversation/init"
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
	payload := map[string]any{
		"requested_default_model": nil,
		"conversation_id":         nil,
		"timezone_offset_min":     -480,
		"conversation_origin":     nil,
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("conversation init failed: %s", readResponseSnippet(response.Body, 500))
	}
	var result conversationInitResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
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

// buildSentinelReqToken 为 /sentinel/req 端点生成指纹 token。
//
// 对齐 2026-06-24 浏览器抓包: /sentinel/req 使用与 prepare **完全相同** 的
// 25 元素 Build25 格式,唯一区别是 [3] nonce=2 (prepare=1)。
// 直接复用 fingerprint.Build25(),不手写重复数组。
func buildSentinelReqToken(state *ChatClientState) string {
	ua := defaultUserAgent()
	deviceID := oaiDeviceID
	if state != nil {
		if state.UserAgent != "" {
			ua = state.UserAgent
		}
		if state.DeviceID != "" {
			deviceID = state.DeviceID
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	fp := browserfp.Get()

	opts := fingerprint.Options{
		UserAgent:           ua,
		ScreenWidth:         fp.ScreenWidth,
		ScreenHeight:        fp.ScreenHeight,
		HardwareConcurrency: fp.HardwareConcurrency,
		JSHeapSizeLimit:     fp.JSHeapSizeLimit,
		BuildID:             fp.BuildID,
		Languages:           strings.Split(browserfp.LanguageJoin(fp.Language), ","),
		Rand:                rng,
	}

	config := fingerprint.Build25(opts)
	config[3] = 2      // nonce: req 用 2 (prepare 用 1)
	config[14] = deviceID

	encoded := prooftoken.EncodeConfig(config)
	return "gAAAAAC" + encoded + "~S"
}

// randomReactSuffix 生成类似 React container suffix 的随机字符串。
func randomReactSuffix() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 11)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

// randomWindowKey 返回随机 window 属性名。
func randomWindowKey() string {
	keys := []string{"onseeking", "onfocus", "onblur", "requestIdleCallback", "webkitRequestAnimationFrame", "__oai_so_bc", "__oai_so_ly"}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return keys[rng.Intn(len(keys))]
}

// POSTSentinelReq 调用 /sentinel/req 端点 (对齐 2026-06-24 浏览器抓包)。
//
// /sentinel/req 使用与 /chat-requirements/prepare **相同** 的 25 元素指纹格式,
// 仅 [3] nonce 不同 (prepare=1, req=2)。
func POSTSentinelReq(client httpclient.AuroraHttpClient, secret *tokens.Secret, requirementsToken, deviceID, flow string, state *ChatClientState) (*sentinelReqResponse, int, error) {
	if flow == "" {
		flow = "conversation"
	}
	// 使用与 prepare 相同的指纹格式,但 nonce=2
	reqToken := buildSentinelReqToken(state)
	apiUrl, targetPath := sentinelURL(secret, "/sentinel/req")
	bodyJSON, err := json.Marshal(map[string]string{
		"p":    reqToken,
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
		header.Set("X-Conduit-Token", conduitToken)
	}
	if strings.HasSuffix(targetPath, "/f/conversation") && !strings.HasSuffix(targetPath, "/prepare") {
		header.Set("Oai-Echo-Logs", "0,943,1,65876,0,68124,1,68930")
		header.Set("Oai-Telemetry", "[1,null]")
	}
	if chatToken != nil {
		if chatToken.TurnStileToken != "" {
			header.Set("Openai-Sentinel-Chat-Requirements-Token", chatToken.TurnStileToken)
		}
		if chatToken.ChatRequirementsPrepareToken != "" {
			header.Set("Openai-Sentinel-Chat-Requirements-Prepare-Token", chatToken.ChatRequirementsPrepareToken)
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
		Proxy:            fhttp.ProxyFromEnvironment,
	}
	if proxyFunc, err := websocketProxyFunc(proxy); err != nil {
		return nil, err
	} else if proxyFunc != nil {
		dialer.Proxy = proxyFunc
	}
	header := fhttp.Header{}
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

func websocketProxyFunc(proxy string) (func(*fhttp.Request) (*url.URL, error), error) {
	if proxy == "" {
		return fhttp.ProxyFromEnvironment, nil
	}
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return nil, err
	}
	return fhttp.ProxyURL(proxyURL), nil
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

// PrepareConversationConduitFullWithSentinel 与 PrepareConversationConduitFull 相同,
// 但在三态 prepare 的每一步都携带已获取的 sentinel token 头。
// 对齐浏览器行为:sentinel 流程(prepare→ping→finalize)在 prepare 流程之前完成,
// conduit token 在 sentinel 上下文中签发,服务器据此判定客户端可信度与模型路由。
//
// 浏览器真实顺序:
//  1. /sentinel/req          → oai-sc cookie (会话级)
//  2. /chat-requirements/prepare → challenge
//  3. /sentinel/ping         → 风控汇报
//  4. /chat-requirements/finalize → chat-requirements token
//  5. /f/conversation/prepare (none→sent→success) → conduit tokens (带 sentinel 头)
//  6. /f/conversation        → 主请求
func PrepareConversationConduitFullWithSentinel(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, secret *tokens.Secret, proxy string, turnTraceID string, state *ChatClientState, turnStile *TurnStile) (string, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	ensureBootstrapped(client, secret)
	// Step 1: none
	token1, err := getConduitTokenWithState(client, message, secret, turnStile, turnTraceID, state, PrepareStateNone, "")
	if err != nil {
		return "", fmt.Errorf("prepare(none) failed: %w", err)
	}
	// Step 2: sent
	token2, err := getConduitTokenWithState(client, message, secret, turnStile, turnTraceID, state, PrepareStateSent, token1)
	if err != nil {
		return "", fmt.Errorf("prepare(sent) failed: %w", err)
	}
	// Step 3: success
	token3, err := getConduitTokenWithState(client, message, secret, turnStile, turnTraceID, state, PrepareStateSuccess, token2)
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

func parseConversationEvent(line string, state *sseparser.PatchState, model string) (conversationStreamEvent, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return conversationStreamEvent{}, false
	}

	if chunk, ok := sseparser.ChunkFromRaw(raw, model); ok {
		event := conversationStreamEvent{
			chunk:          &chunk,
			text:           sseparser.ChunkContent(chunk),
			role:           sseparser.ChunkRole(chunk),
			conversationID: chunk.ConversationID,
			channel:        sseparser.ChannelFromValue(raw),
			finishReason:   sseparser.ChunkFinishReason(chunk),
		}
		event.isStop = event.finishReason != ""
		return event, true
	}

	var direct chatgpt_types.ChatGPTResponse
	if err := json.Unmarshal([]byte(line), &direct); err == nil && sseparser.IsUsableConversationResponse(direct) {
		channel := sseparser.ChannelFromValue(raw)
		state.Channel = firstNonEmpty(channel, state.Channel)
		return conversationStreamEvent{response: direct, messageID: direct.Message.ID, channel: state.Channel}, true
	}

	if response, ok := sseparser.ResponseFromValue(raw["v"]); ok {
		state.Response = response
		if channel := sseparser.ChannelFromValue(raw["v"]); channel != "" {
			state.Channel = channel
		}
		return conversationStreamEvent{response: state.Response, messageID: state.Response.Message.ID, channel: state.Channel}, true
	}
	if text, ok := raw["v"].(string); ok && raw["p"] == nil && raw["o"] == nil {
		sseparser.EnsurePatchDefaults(state)
		current, _ := state.Response.Message.Content.Parts[0].(string)
		state.Response.Message.Content.Parts[0] = current + text
		return conversationStreamEvent{response: state.Response, messageID: state.Response.Message.ID, channel: state.Channel}, true
	}

	if patchPath, ok := raw["p"].(string); ok {
		patchOperation, _ := raw["o"].(string)
		// 处理批量 patch: {"p": "", "o": "patch", "v": [{"p": "...", "o": "append", "v": "..."}, ...]}
		// 新版 ChatGPT Web 会在最后把多个 patch 打包成一条 SSE 发出。
		if patchPath == "" && patchOperation == "patch" {
			if batch, ok := raw["v"].([]interface{}); ok {
				applied := false
				for _, item := range batch {
					op, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					subPath, _ := op["p"].(string)
					subOp, _ := op["o"].(string)
					if sseparser.ApplyPatch(state, subPath, subOp, op["v"]) {
						applied = true
					}
				}
				if applied {
					return conversationStreamEvent{response: state.Response, messageID: state.Response.Message.ID, channel: state.Channel}, true
				}
			}
		}
		if sseparser.ApplyPatch(state, patchPath, patchOperation, raw["v"]) {
			return conversationStreamEvent{response: state.Response, messageID: state.Response.Message.ID, channel: state.Channel}, true
		}
	}

	return conversationStreamEvent{}, false
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
	var patchState sseparser.PatchState
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
		if stream {
			reasoningChunk := official_types.NewReasoningChunk(delta, model)
			if convId != "" {
				reasoningChunk.ConversationID = convId
			}
			c.Writer.WriteString("data: " + reasoningChunk.String() + "\n\n")
			c.Writer.Flush()
		}
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
		if eventName, ok := sseparser.EventName(line); ok {
			currentEvent = eventName
		}
		for _, line := range sseparser.DataPayloads(line) {
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
			if topicID, skip := sseparser.HandoffTopicFromPayload(line, currentEvent); skip {
				if topicID != "" {
					handoffTopicID = topicID
				}
				currentEvent = ""
				continue
			}
			// Parse the line as JSON
			streamEvent, ok := parseConversationEvent(line, &patchState, model)
			if os.Getenv("DEBUG_SSE") != "" {
				debugText := streamEvent.text
				debugSrc := "chunk"
				if streamEvent.response.Message.ID != "" {
					debugText = sseparser.FirstStringPart(streamEvent.response.Message.Content.Parts)
					debugSrc = "response"
				}
				raw := strings.TrimSpace(line)
				if len(raw) > 200 {
					raw = raw[:200] + "..."
				}
				fmt.Printf("[sse-in] src=%s channel=%q textLen=%d finish=%q parsed=%v raw=%q\n", debugSrc, streamEvent.channel, len(debugText), streamEvent.finishReason, ok, raw)
			}
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
				deltaText := sseparser.NormalizeContentDelta(previous_text.Text, streamEvent.text)
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
			sentinel = append(sentinel, sseparser.SentinelsFromResponse(original_response)...)
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
				thinkingDelta := sseparser.NormalizeContentDelta(thinkingText, sseparser.FirstStringPart(original_response.Message.Content.Parts))
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
	// 对齐 2026-06-24 chatgpt.com 浏览器抓包:Chrome 147 Win64
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
	// sec-ch-ua-* 对齐 Chrome 148 (与 UA / prooftoken 同步, 对齐 2026-06-24 浏览器抓包)
	header.Set("Sec-Ch-Ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	header.Set("Sec-Ch-Ua-Mobile", "?0")
	header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
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
	// 对齐 2026-06-24 chatgpt.com 浏览器抓包的 build / version
	if fp := browserfp.Get(); fp != nil {
		header.Set("Oai-Client-Version", fp.BuildID)
	} else {
		header.Set("Oai-Client-Version", browserfp.DefaultBuildID)
	}
	header.Set("Oai-Client-Build-Number", "7823760")
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

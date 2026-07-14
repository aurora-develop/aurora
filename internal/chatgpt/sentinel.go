package chatgpt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"aurora/httpclient"
	"aurora/internal/accounts"
	"aurora/internal/browserfp"
	"aurora/internal/fingerprint"
	"aurora/internal/prooftoken"
	"aurora/internal/so"
	"aurora/internal/turnstile"
)

// TurnStile 表示一次 sentinel 风控流程的完整状态，
// 包含所有从 prepare / finalize / ping 等端点获得的 token。
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

// ProofWork 表示 proof-of-work 要求。
type ProofWork struct {
	Difficulty string `json:"difficulty,omitempty"`
	Required   bool   `json:"required"`
	Seed       string `json:"seed,omitempty"`
}

// SoSegment 表示 sentinel SO token 要求。
type SoSegment struct {
	Required    bool   `json:"required"`
	CollectorDX string `json:"collector_dx,omitempty"`
	SnapshotDX  string `json:"snapshot_dx,omitempty"`
}

// ChatRequire 是 /sentinel/chat-requirements/prepare 的响应结构。
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

// sentinelReqResponse 是 POST /sentinel/req 的响应。
type sentinelReqResponse struct {
	Token     string `json:"token"`
	Flow      string `json:"flow"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	ChatReq   string `json:"chat_req,omitempty"`
	Persona   string `json:"persona,omitempty"`
}

// pingSentinelResponse 是 POST /backend-api/sentinel/ping 的响应。
type pingSentinelResponse struct {
	Status string `json:"status"`
}

// sentinelExtraData 对齐 chatgpt.com JS 中编码的 OpenAI-Sentinel-Extra-Data header。
type sentinelExtraData struct {
	Version        int                `json:"v"`
	SequenceNumber int                `json:"sequence_number"`
	Signals        sentinelExtraSignals `json:"signals"`
	ConversationID string             `json:"conversation_id,omitempty"`
	LastMessageID  string             `json:"last_message_id,omitempty"`
}

type sentinelExtraSignals struct {
	PingSource                   string `json:"ping_source"`
	SOTokenPresent               string `json:"so_token_present"`
	TurnstileTokenPresent        string `json:"turnstile_token_present"`
	ProofTokenPresent            string `json:"proof_token_present"`
	PrepareTokenPresent          string `json:"prepare_token_present"`
	ChatRequirementsTokenPresent string `json:"chat_requirements_token_present"`
}

// GetInitConfig 生成 25 元素的初始化配置数组（用于 sentinel/fingerprint）。
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

// CalcProofToken 计算 proof-of-work token。
func CalcProofToken(require *ChatRequire, state *ChatClientState) string {
	ua := defaultUserAgent()
	if state != nil && state.UserAgent != "" {
		ua = state.UserAgent
	}
	return prooftoken.SolveProofToken(require.Proof.Seed, require.Proof.Difficulty, ua)
}

// InitTurnStileWithState 初始化 TurnStile（等同于 InitSentinelWithState 但 retry=0）。
func InitTurnStileWithState(client httpclient.AuroraHttpClient, account *accounts.Account, proxy string, state *ChatClientState) (*TurnStile, int, error) {
	return InitSentinelWithState(client, account, proxy, 0, state)
}

// InitSentinel 初始化 sentinel 风控流程。
func InitSentinel(client httpclient.AuroraHttpClient, account *accounts.Account, proxy string, retry int) (*TurnStile, int, error) {
	return InitSentinelWithState(client, account, proxy, retry, nil)
}

// InitSentinelWithState 初始化 sentinel 风控流程（带 state）。
func InitSentinelWithState(client httpclient.AuroraHttpClient, account *accounts.Account, proxy string, retry int, state *ChatClientState) (*TurnStile, int, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	ua := defaultUserAgent()
	if state != nil && state.UserAgent != "" {
		ua = state.UserAgent
	}
	requirementsToken := prooftoken.NewConfig(ua).RequirementsToken()

	prepare, status, err := POSTSentinelPrepareWithState(client, account, requirementsToken, state)
	if err != nil {
		if account.Type == accounts.TypeNoAuth && status == http.StatusUnauthorized && retry < 2 {
			time.Sleep(time.Second * 2)
			account.Token = uuid.NewString()
			return InitSentinelWithState(client, account, proxy, retry+1, state)
		}
		return nil, status, err
	}
	if prepare.ForceLogin {
		if !(account.Type == accounts.TypeNoAuth) {
			return nil, http.StatusUnauthorized, fmt.Errorf("force login required: ChatGPT access token is expired or not accepted")
		}
		if retry > 1 {
			return nil, http.StatusForbidden, fmt.Errorf("force login required")
		}
		time.Sleep(time.Second)
		account.Token = uuid.NewString()
		return InitSentinelWithState(client, account, proxy, retry+1, state)
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

	// 构建 TurnStile（先于 finalize）
	ts := &TurnStile{
		ProofOfWorkToken:             proofToken,
		TurnstileToken:               turnstileToken,
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

	finalize, status, err := POSTSentinelFinalizeWithState(client, account, prepare.PrepareToken, proofToken, turnstileToken, state)
	if err != nil {
		if account.Type == accounts.TypeNoAuth && status == http.StatusUnauthorized && retry < 2 {
			time.Sleep(time.Second * 2)
			account.Token = uuid.NewString()
			return InitSentinelWithState(client, account, proxy, retry+1, state)
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

// stateFlow 推导 so token 里的 flow 字段。
func stateFlow(state *ChatClientState, ua string) string {
	if state != nil && state.DeviceID != "" {
		return state.DeviceID
	}
	if ua != "" {
		return "chatgpt-freeaccount"
	}
	return "chatgpt"
}

// soDeviceIDFor 给出 openai-sentinel-so-token 的 deviceID 参数。
func soDeviceIDFor(account *accounts.Account) string {
	if account != nil && account.Token != "" {
		return account.Token
	}
	return ""
}

// ensureSOToken 懒求值 openai-sentinel-so-token header 值。
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

// POSTSentinelPing 调用 /sentinel/ping，默认 ping_source="session_observer_background_submit", seq=0。
func POSTSentinelPing(client httpclient.AuroraHttpClient, account *accounts.Account, ts *TurnStile, conversationID, lastMessageID string, state *ChatClientState) error {
	return POSTSentinelPingWithSource(client, account, ts, conversationID, lastMessageID, state, "session_observer_background_submit", 0)
}

// POSTSentinelPingWithSource 支持自定义 ping_source 和 sequence_number。
func POSTSentinelPingWithSource(client httpclient.AuroraHttpClient, account *accounts.Account, ts *TurnStile, conversationID, lastMessageID string, state *ChatClientState, pingSource string, sequenceNumber int) error {
	apiUrl, targetPath := sentinelURL(account, "/sentinel/ping")
	header := sentinelHeaderWithState(account, targetPath, state)
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
		if soToken := ts.ensureSOToken(soDeviceIDFor(account)); soToken != "" {
			header.Set("Openai-Sentinel-So-Token", soToken)
		}
		extraData := buildSentinelExtraData(
			conversationID,
			lastMessageID,
			ts.ChatRequirementsPrepareToken,
			ts.ChatRequirementsToken,
			ts.ensureSOToken(soDeviceIDFor(account)) != "",
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

// POSTSentinelPrepare 发送 /sentinel/chat-requirements/prepare（无 state）。
func POSTSentinelPrepare(client httpclient.AuroraHttpClient, account *accounts.Account, requirementsToken string) (*ChatRequire, int, error) {
	return POSTSentinelPrepareWithState(client, account, requirementsToken, nil)
}

// POSTSentinelPrepareWithState 发送 /sentinel/chat-requirements/prepare（带 state）。
func POSTSentinelPrepareWithState(client httpclient.AuroraHttpClient, account *accounts.Account, requirementsToken string, state *ChatClientState) (*ChatRequire, int, error) {
	apiUrl, targetPath := sentinelURL(account, "/sentinel/chat-requirements/prepare")
	bodyJSON, err := json.Marshal(map[string]string{"p": requirementsToken})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	header := sentinelHeaderWithState(account, targetPath, state)
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

// POSTSentinelFinalize 发送 /sentinel/chat-requirements/finalize（无 state）。
func POSTSentinelFinalize(client httpclient.AuroraHttpClient, account *accounts.Account, prepareToken, proofToken, turnstileToken string) (*sentinelFinalizeResponse, int, error) {
	return POSTSentinelFinalizeWithState(client, account, prepareToken, proofToken, turnstileToken, nil)
}

// POSTSentinelFinalizeWithState 发送 /sentinel/chat-requirements/finalize（带 state）。
func POSTSentinelFinalizeWithState(client httpclient.AuroraHttpClient, account *accounts.Account, prepareToken, proofToken, turnstileToken string, state *ChatClientState) (*sentinelFinalizeResponse, int, error) {
	apiUrl, targetPath := sentinelURL(account, "/sentinel/chat-requirements/finalize")
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
	header := sentinelHeaderWithState(account, targetPath, state)
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

// buildSentinelReqToken 为 /sentinel/req 端点生成指纹 token (nonce=2)。
func buildSentinelReqToken(state *ChatClientState, account *accounts.Account) string {
	ua := defaultUserAgent()
	deviceID := oaiDeviceID
	if account != nil && account.Fingerprint.OaiDeviceID != "" {
		deviceID = account.Fingerprint.OaiDeviceID
	}
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

func randomReactSuffix() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 11)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

func randomWindowKey() string {
	keys := []string{"onseeking", "onfocus", "onblur", "requestIdleCallback", "webkitRequestAnimationFrame", "__oai_so_bc", "__oai_so_ly"}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return keys[rng.Intn(len(keys))]
}

// POSTSentinelReq 调用 /sentinel/req 端点。
func POSTSentinelReq(client httpclient.AuroraHttpClient, account *accounts.Account, requirementsToken, deviceID, flow string, state *ChatClientState) (*sentinelReqResponse, int, error) {
	if flow == "" {
		flow = "conversation"
	}
	reqToken := buildSentinelReqToken(state, account)
	apiUrl, targetPath := sentinelURL(account, "/sentinel/req")
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
	header.Set("Content-Type", "text/plain;charset=UTF-8")
	header.Set("X-Openai-Target-Path", targetPath)
	header.Set("X-Openai-Target-Route", targetPath)
	if state == nil || state.ConversationID == "" {
		header.Set("Referer", "https://chatgpt.com/backend-api/sentinel/frame.html?sv=20260423af3c")
	}
	if account != nil && account.Type == accounts.TypeNoAuth && account.Token != "" {
		header.Set("Oai-Device-Id", account.Token)
	}
	if account != nil && !(account.Type == accounts.TypeNoAuth) && account.Token != "" {
		header.Set("Authorization", "Bearer "+account.Token)
	}
	setTeamAccountHeader(header, account)
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

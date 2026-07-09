package accounts

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"aurora/httpclient"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/websocket"
)

// wssCommand 是 Service 层向 WSS goroutine 发送的指令
type wssCommand struct {
	Type    string      // "subscribe", "close"
	Payload interface{}
	Result  chan<- wssResult
}

type wssResult struct {
	Data []byte
	Err  error
}

// WSSActor 每个 free/puid Account 持有一个 goroutine
// 统一管理 WSS 连接、心跳保活、消息发送和重连
type WSSActor struct {
	account  *Account
	commands chan wssCommand
	done     chan struct{}
	started  bool
	mu       sync.Mutex

	// 运行时状态（run goroutine 内使用）
	conn         *websocket.Conn
	idCounter    int       // 消息 ID，从 5 开始递增
	lastActivity time.Time // 最后一次发送消息的时间，用于推迟心跳
}

type wssURLResponse struct {
	WebsocketURL string `json:"websocket_url"`
}

func NewWSSActor(account *Account) *WSSActor {
	return &WSSActor{
		account:  account,
		commands: make(chan wssCommand, 16),
		done:     make(chan struct{}),
	}
}

// Start 启动 WSS goroutine
func (a *WSSActor) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started {
		return
	}
	a.started = true
	a.done = make(chan struct{})
	go a.run()
}

// Stop 停止 WSS goroutine
func (a *WSSActor) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.started {
		return
	}
	select {
	case <-a.done:
		return
	default:
		close(a.done)
	}
	a.started = false
}

// Subscribe 订阅一个 topic（通过 channel 发送指令到 goroutine）
func (a *WSSActor) Subscribe(topicID string) error {
	result := make(chan wssResult, 1)
	a.commands <- wssCommand{
		Type:    "subscribe",
		Payload: topicID,
		Result:  result,
	}
	r := <-result
	return r.Err
}

// ─── goroutine 主循环 ─────────────────────────────────────────────

func (a *WSSActor) run() {
	reconnectDelay := time.Second

	for {
		// 连接并初始化
		err := a.connect()
		if err != nil {
			select {
			case <-a.done:
				return
			default:
			}
			// 指数退避重连，封顶 60s
			time.Sleep(reconnectDelay)
			reconnectDelay = time.Duration(math.Min(
				float64(reconnectDelay*2),
				float64(60*time.Second),
			))
			continue
		}

		// 连接成功，重置重连延迟
		reconnectDelay = time.Second
		a.idCounter = 5
		a.lastActivity = time.Now()

		// 启动读 goroutine
		readCh := make(chan []byte, 64)
		errCh := make(chan error, 1)
		go a.readLoop(readCh, errCh)

		// 心跳定时器 (30s)
		heartbeat := time.NewTicker(30 * time.Second)

		err = a.mainLoop(readCh, errCh, heartbeat)
		heartbeat.Stop()

		if a.conn != nil {
			a.conn.Close()
			a.conn = nil
		}

		// 正常退出
		if err == nil {
			return
		}
		// 异常退出 → 重连
	}
}

// connect 建立 WSS 连接并完成初始化握手
func (a *WSSActor) connect() error {
	if a.account.Type == TypeNoAuth {
		return fmt.Errorf("noauth account cannot establish WSS")
	}
	if a.account.Client == nil {
		return fmt.Errorf("account client not initialized")
	}

	// 1. GET /celsius/ws/user
	wsURL, err := a.getWebsocketURL()
	if err != nil {
		return fmt.Errorf("get ws url: %w", err)
	}

	// 2. Dial WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		Proxy:            fhttp.ProxyFromEnvironment,
	}

	// 配置代理
	if a.account.Proxy != "" {
		proxyFunc, err := websocketProxyFunc(a.account.Proxy)
		if err == nil && proxyFunc != nil {
			dialer.Proxy = proxyFunc
		}
	}

	header := fhttp.Header{}
	header.Set("User-Agent", a.account.Fingerprint.UserAgent)
	if header.Get("User-Agent") == "" {
		header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	}
	header.Set("Origin", "https://chatgpt.com")

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	a.conn = conn

	// 3. 发送 4 条初始化消息，id: 1-4
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
		return fmt.Errorf("ws write init: %w", err)
	}

	// 4. 等待收到 4 条 reply（id:1-4）
	for i := 0; i < 4; i++ {
		conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			conn.Close()
			return fmt.Errorf("ws read init reply %d: %w", i+1, err)
		}
		// 解析并确认是 reply
		var frames []map[string]interface{}
		if len(raw) > 0 && raw[0] == '[' {
			json.Unmarshal(raw, &frames)
		} else {
			var frame map[string]interface{}
			json.Unmarshal(raw, &frame)
			frames = []map[string]interface{}{frame}
		}
		for _, f := range frames {
			ftype, _ := f["type"].(string)
			if ftype == "reply" {
				// 收到 reply 即确认
			}
		}
	}

	return nil
}

// getWebsocketURL 调用 /celsius/ws/user 获取 WSS URL
func (a *WSSActor) getWebsocketURL() (string, error) {
	apiURL := a.buildWSSURL()
	headers := httpclient.AuroraHeaders{
		"User-Agent": a.account.Fingerprint.UserAgent,
		"Accept":     "*/*",
		"Origin":     "https://chatgpt.com",
	}
	// 如果有 token，加 Authorization
	if a.account.Token != "" {
		headers["Authorization"] = "Bearer " + a.account.Token
	}
	// 如果有 OAI-Device-ID
	if a.account.Fingerprint.OaiDeviceID != "" {
		headers["Oai-Device-Id"] = a.account.Fingerprint.OaiDeviceID
	}

	response, err := a.account.Client.Request(httpclient.GET, apiURL, headers, nil, nil)
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

	var result wssURLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.WebsocketURL == "" {
		return "", fmt.Errorf("celsius ws user missing websocket_url: %s", string(body))
	}
	return result.WebsocketURL, nil
}

// buildWSSURL 构造 /celsius/ws/user 的完整 URL
func (a *WSSActor) buildWSSURL() string {
	baseURL := "https://chatgpt.com/backend-api"
	return baseURL + "/celsius/ws/user"
}

// readLoop 持续读取 WSS 消息
func (a *WSSActor) readLoop(readCh chan<- []byte, errCh chan<- error) {
	for {
		a.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		_, raw, err := a.conn.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		readCh <- raw
	}
}

// mainLoop select 主循环
func (a *WSSActor) mainLoop(readCh <-chan []byte, errCh <-chan error, heartbeat *time.Ticker) error {
	for {
		select {
		case <-a.done:
			return nil

		case cmd := <-a.commands:
			switch cmd.Type {
			case "close":
				if cmd.Result != nil {
					cmd.Result <- wssResult{}
				}
				return nil

			case "subscribe":
				topicID, _ := cmd.Payload.(string)
				err := a.sendSubscribe(topicID)
				if cmd.Result != nil {
					if err != nil {
						cmd.Result <- wssResult{Err: err}
					} else {
						cmd.Result <- wssResult{}
					}
				}
				a.lastActivity = time.Now()

			default:
				if cmd.Result != nil {
					cmd.Result <- wssResult{}
				}
			}

		case raw := <-readCh:
			// 收到消息 — 回复就是心跳确认，不需要额外处理
			_ = raw

		case <-heartbeat.C:
			// 心跳保活：距上次发送超过 30s 才发
			if time.Since(a.lastActivity) >= 30*time.Second {
				a.sendPresence()
				a.lastActivity = time.Now()
			}

		case <-errCh:
			// 连接断开，退出循环触发重连
			return fmt.Errorf("ws connection lost")
		}
	}
}

// sendSubscribe 发送 subscribe 命令
func (a *WSSActor) sendSubscribe(topicID string) error {
	if a.conn == nil {
		return fmt.Errorf("ws not connected")
	}
	msg := []map[string]interface{}{
		{"id": a.nextID(), "command": map[string]interface{}{
			"type":     "subscribe",
			"topic_id": topicID,
		}},
	}
	return a.conn.WriteJSON(msg)
}

// sendPresence 发送心跳（presence foreground）
func (a *WSSActor) sendPresence() {
	if a.conn == nil {
		return
	}
	msg := []map[string]interface{}{
		{"id": a.nextID(), "command": map[string]interface{}{
			"type":    "presence",
			"state":   "foreground",
		}},
	}
	_ = a.conn.WriteJSON(msg)
}

// nextID 线程安全的 ID 计数器
func (a *WSSActor) nextID() int {
	id := a.idCounter
	a.idCounter++
	return id
}

// websocketProxyFunc 为 WebSocket 代理创建 Dialer Proxy 函数
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


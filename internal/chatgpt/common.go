package chatgpt

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"aurora/httpclient"
	"aurora/internal/accounts"
	"aurora/internal/headerbuilder"
	"aurora/util"
)

var BaseURL string

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

// GetDpl 从 ChatGPT 首页解析 script 标签和 dpl 参数。
// 缓存结果,避免重复请求。
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

// defaultUserAgent 返回全局统一的 User-Agent (Chrome 148 Windows)。
func defaultUserAgent() string {
	return util.FixedUserAgent
}

// createBaseHeader 创建基础 header（无 state）。
func createBaseHeader() httpclient.AuroraHeaders {
	return createBaseHeaderForState(nil)
}

// createBaseHeaderForState 创建带 state 的基础 header。
func createBaseHeaderForState(state *ChatClientState) httpclient.AuroraHeaders {
	conversationID := ""
	deviceID := oaiDeviceID
	sessionID := oaiSessionID
	ua := ""
	if state != nil {
		if state.ConversationID != "" {
			conversationID = state.ConversationID
		}
		if state.DeviceID != "" {
			deviceID = state.DeviceID
		}
		if state.SessionID != "" {
			sessionID = state.SessionID
		}
		if state.UserAgent != "" {
			ua = state.UserAgent
		}
	}
	return headerbuilder.NewBaseHeaderWithState(conversationID, deviceID, sessionID, ua)
}

// readResponseSnippet 读取响应体的前 limit 字节用于错误报告。
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

// setTeamAccountHeader 设置 ChatGPT-Account-Id header（如果有 team account）。
func setTeamAccountHeader(header httpclient.AuroraHeaders, account *accounts.Account) {
	if account != nil && strings.TrimSpace(account.TeamUserID) != "" {
		header.Set("Chatgpt-Account-Id", strings.TrimSpace(account.TeamUserID))
	}
}

// getURLAttribution 获取 URL 的 attribution。
func getURLAttribution(client httpclient.AuroraHttpClient, account *accounts.Account, url string) string {
	requestURL := BaseURL + "/attributions"
	payload := bytes.NewBuffer([]byte(`{"urls":["` + url + `"]}`))
	header := createBaseHeader()
	if account != nil && account.PUID != "" {
		header.Set("Cookie", "_puid="+account.PUID+";")
	}
	header.Set("Content-Type", "application/json")
	if account != nil && account.Token != "" {
		header.Set("Authorization", "Bearer "+account.Token)
	}
	setTeamAccountHeader(header, account)
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

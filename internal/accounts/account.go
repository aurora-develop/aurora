package accounts

import (
	"fmt"
	"time"

	"aurora/httpclient"
	"aurora/httpclient/bogdanfinn"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// AccountType 账号类型
type AccountType int

const (
	TypeNoAuth AccountType = iota // 匿名设备 UUID
	TypeFree                      // ChatGPT 免费登录账号
	TypePUID                      // ChatGPT 付费/PRO 账号
)

func (t AccountType) String() string {
	switch t {
	case TypeNoAuth:
		return "noauth"
	case TypeFree:
		return "free"
	case TypePUID:
		return "puid"
	}
	return fmt.Sprintf("unknown(%d)", t)
}

// AccountStatus 账号生命周期状态
type AccountStatus int

const (
	StatusPending    AccountStatus = iota // 初始化中
	StatusActive                          // 正常
	StatusExpired                         // Token 过期，可续期
	StatusRateLimited                     // 被限流，等待冷却
	StatusDisabled                        // 手动停用
	StatusBanned                          // 被封禁
)

func (s AccountStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusActive:
		return "active"
	case StatusExpired:
		return "expired"
	case StatusRateLimited:
		return "rate_limited"
	case StatusDisabled:
		return "disabled"
	case StatusBanned:
		return "banned"
	}
	return fmt.Sprintf("unknown(%d)", s)
}

// Account 一个账号 = 完整隔离单元
// 每个账号拥有独立的 TLS Client、代理 IP、浏览器指纹和 WSS 连接
type Account struct {
	ID   string
	Type AccountType

	// 认证
	Token         string // access_token 或 UUID
	RefreshToken  string // 用于自动续期（/auth/refresh）
	SessionToken  string // 仅免费账号有，用于续期（/auth/session）

	// 身份
	PUID       string
	TeamUserID string

	// 隔离单元（每个账号独立）
	Client      httpclient.AuroraHttpClient // 专属 TLS Client
	Proxy       string                      // 专属代理 IP
	Fingerprint BrowserFingerprint          // 专属指纹

	// WSS (free/puid 有, noauth 无)
	WSSActor interface{} // *WSSActor — 避免 import 循环

	// 状态
	Status    AccountStatus
	ExpiresAt time.Time

	// 统计
	TotalCalls  int64
	FailedCalls int64
	LastUsed    time.Time
	LastChecked time.Time
	CreatedAt   time.Time
}

// BrowserFingerprint 浏览器指纹（所有参数自洽配套）
type BrowserFingerprint struct {
	OaiDeviceID         string
	OaiSessionID        string
	UserAgent           string
	ScreenWidth         int
	ScreenHeight        int
	HardwareConcurrency int
	Platform            string
	TLSProfileName      string // 对应 bogdanfinn profiles 名称
}

// NewAccount 创建新账号
func NewAccount(id string, acctType AccountType, token string) *Account {
	now := time.Now()
	return &Account{
		ID:        id,
		Type:      acctType,
		Token:     token,
		Status:    StatusPending,
		CreatedAt: now,
	}
}

// InitClient 创建专属 TLS Client
func (a *Account) InitClient() error {
	opts := []tls_client.HttpClientOption{
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
		tls_client.WithTimeoutSeconds(600),
	}

	// 绑定指纹画像
	profile := resolveTLSProfile(a.Fingerprint.TLSProfileName)
	opts = append(opts, tls_client.WithClientProfile(profile))

	// 创建底层 client
	baseClient, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil {
		return err
	}
	tlsClient := &bogdanfinn.TlsClient{Client: baseClient}

	// 设代理
	if a.Proxy != "" {
		if err := tlsClient.SetProxy(a.Proxy); err != nil {
			return err
		}
	}

	a.Client = tlsClient
	return nil
}

// resolveTLSProfile 映射指纹画像名到 tls-client profile
func resolveTLSProfile(name string) profiles.ClientProfile {
	switch name {
	case "chrome_146":
		return profiles.Chrome_146
	case "safari_16_0":
		return profiles.Safari_16_0
	case "safari_ios_18_5":
		return profiles.Safari_IOS_18_5
	case "safari_ios_17_0":
		return profiles.Safari_IOS_17_0
	case "safari_ipad_15_6":
		return profiles.Safari_Ipad_15_6
	default:
		return profiles.Chrome_146
	}
}

package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerHost         string
	ServerPort         string
	TLSCert            string
	TLSKey             string
	Authorization      string
	BaseURL            string
	APIReverseProxy    string
	FilesReverseProxy  string
	StreamMode         bool
	MaxContinueCount   int
	EnableHistory      bool
	EnableExternalToken bool  // 是否接受外部传入的 accessToken
	ToolCallingEnabled bool
	RefusalRetries     int
	DebugToolLog       string
	FreeAccounts       bool
	FreeAccountsNum    int
	ProxyURL           string
	HTTPProxy          string
	DebugSentinel      bool
}

func Load() Config {
	return Config{
		ServerHost:         getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:         getEnvWithFallback("SERVER_PORT", "PORT", "8080"),
		TLSCert:            os.Getenv("TLS_CERT"),
		TLSKey:             os.Getenv("TLS_KEY"),
		Authorization:      os.Getenv("Authorization"),
		BaseURL:            getEnv("BASE_URL", "https://chatgpt.com/backend-api"),
		APIReverseProxy:    os.Getenv("API_REVERSE_PROXY"),
		FilesReverseProxy:  os.Getenv("FILES_REVERSE_PROXY"),
		StreamMode:         getBoolEnv("STREAM_MODE", true),
		MaxContinueCount:   getIntEnv("MAX_CONTINUE_COUNT", 3),
		EnableHistory:      getBoolEnv("ENABLE_HISTORY", false),
		EnableExternalToken: getBoolEnv("ENABLE_EXTERNAL_TOKEN", false),
		ToolCallingEnabled: getBoolEnv("TOOL_CALLING_ENABLED", true),
		RefusalRetries:     getIntEnv("REFUSAL_RETRIES", 3),
		DebugToolLog:       os.Getenv("DEBUG_TOOL_LOG"),
		FreeAccounts:       getBoolEnv("FREE_ACCOUNTS", false),
		FreeAccountsNum:    getIntEnv("FREE_ACCOUNTS_NUM", 1024),
		ProxyURL:           os.Getenv("PROXY_URL"),
		HTTPProxy:          os.Getenv("http_proxy"),
		DebugSentinel:      getBoolEnv("DEBUG_SENTINEL", false),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvWithFallback(key, fallbackKey, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if v := os.Getenv(fallbackKey); v != "" {
		return v
	}
	return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

func getIntEnv(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}

package accounts

// FingerprintProfile 一个自洽的指纹画像
type FingerprintProfile struct {
	Name                string
	TLSProfileName      string
	UserAgent           string
	ScreenWidth         int
	ScreenHeight        int
	HardwareConcurrency int
	Platform            string
}

// DefaultProfiles 预定义的 8 个自洽指纹画像
// TLS 指纹、UA、视窗各维度绑定成一套
var DefaultProfiles = []FingerprintProfile{
	{
		Name: "chrome_win_high", TLSProfileName: "chrome_146",
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
		ScreenWidth: 2560, ScreenHeight: 1440, HardwareConcurrency: 16, Platform: "Win32",
	},
	{
		Name: "chrome_win_medium", TLSProfileName: "chrome_146",
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
		ScreenWidth: 1920, ScreenHeight: 1080, HardwareConcurrency: 8, Platform: "Win32",
	},
	{
		Name: "chrome_win_low", TLSProfileName: "chrome_146",
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
		ScreenWidth: 1366, ScreenHeight: 768, HardwareConcurrency: 4, Platform: "Win32",
	},
	{
		Name: "chrome_mac", TLSProfileName: "chrome_146",
		UserAgent:           "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
		ScreenWidth: 3024, ScreenHeight: 1964, HardwareConcurrency: 12, Platform: "MacIntel",
	},
	{
		Name: "safari_mac", TLSProfileName: "safari_16_0",
		UserAgent:           "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
		ScreenWidth: 3024, ScreenHeight: 1964, HardwareConcurrency: 10, Platform: "MacIntel",
	},
	{
		Name: "safari_iphone_pro", TLSProfileName: "safari_ios_18_5",
		UserAgent:           "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1",
		ScreenWidth: 393, ScreenHeight: 852, HardwareConcurrency: 6, Platform: "iPhone",
	},
	{
		Name: "safari_iphone", TLSProfileName: "safari_ios_17_0",
		UserAgent:           "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		ScreenWidth: 390, ScreenHeight: 844, HardwareConcurrency: 6, Platform: "iPhone",
	},
	{
		Name: "safari_ipad", TLSProfileName: "safari_ipad_15_6",
		UserAgent:           "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		ScreenWidth: 1024, ScreenHeight: 1366, HardwareConcurrency: 8, Platform: "iPad",
	},
}

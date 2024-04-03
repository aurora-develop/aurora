package funcaptcha

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"io"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GetTokenOptions struct {
	PKey     string            `json:"pkey"`
	SURL     string            `json:"surl,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Site     string            `json:"site,omitempty"`
	Location string            `json:"location,omitempty"`
	Proxy    string            `json:"proxy,omitempty"`
}

type GetTokenResult struct {
	ChallengeURL          string `json:"challenge_url"`
	ChallengeURLCDN       string `json:"challenge_url_cdn"`
	ChallengeURLCDNSRI    string `json:"challenge_url_cdn_sri"`
	DisableDefaultStyling bool   `json:"disable_default_styling"`
	IFrameHeight          int    `json:"iframe_height"`
	IFrameWidth           int    `json:"iframe_width"`
	KBio                  bool   `json:"kbio"`
	MBio                  bool   `json:"mbio"`
	NoScript              string `json:"noscript"`
	TBio                  bool   `json:"tbio"`
	Token                 string `json:"token"`
}

type OpenAiRequest struct {
	Request *http.Request
	Client  *tls_client.HttpClient
}

var (
	mu      sync.Mutex
	jar     = tls_client.NewCookieJar()
	options = []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(360),
		tls_client.WithClientProfile(profiles.Chrome_112),
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar),
		tls_client.WithInsecureSkipVerify(),
	}
	client, _ = tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
)

func init() {
	proxy := os.Getenv("http_proxy")
	if proxy != "" {
		client.SetProxy(proxy)
	}
}

func (r *OpenAiRequest) GetToken() (string, error) {
	resp, err := (*r.Client).Do(r.Request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result GetTokenResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}
	return result.Token, nil
}

func NewOpenAiRequestV1(public_key string) (*OpenAiRequest, error) {
	// generate timestamp in 1687790752 format
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1000000000)
	bx := fmt.Sprintf(`[{"key":"api_type","value":"js"},{"key":"p","value":1},{"key":"f","value":"5474099d8b4c4b6902203a929ffc0bec"},{"key":"n","value":"%s"},{"key":"wh","value":"c625241942b18a7e6bf5c7dae8bd566b|72627afbfd19a741c7da1732218301ac"},{"key":"enhanced_fp","value":[{"key":"webgl_extensions","value":null},{"key":"webgl_extensions_hash","value":null},{"key":"webgl_renderer","value":null},{"key":"webgl_vendor","value":null},{"key":"webgl_version","value":null},{"key":"webgl_shading_language_version","value":null},{"key":"webgl_aliased_line_width_range","value":null},{"key":"webgl_aliased_point_size_range","value":null},{"key":"webgl_antialiasing","value":null},{"key":"webgl_bits","value":null},{"key":"webgl_max_params","value":null},{"key":"webgl_max_viewport_dims","value":null},{"key":"webgl_unmasked_vendor","value":null},{"key":"webgl_unmasked_renderer","value":null},{"key":"webgl_vsf_params","value":null},{"key":"webgl_vsi_params","value":null},{"key":"webgl_fsf_params","value":null},{"key":"webgl_fsi_params","value":null},{"key":"webgl_hash_webgl","value":null},{"key":"user_agent_data_brands","value":null},{"key":"user_agent_data_mobile","value":null},{"key":"navigator_connection_downlink","value":null},{"key":"navigator_connection_downlink_max","value":null},{"key":"network_info_rtt","value":null},{"key":"network_info_save_data","value":null},{"key":"network_info_rtt_type","value":null},{"key":"screen_pixel_depth","value":24},{"key":"navigator_device_memory","value":null},{"key":"navigator_languages","value":"en-US,en"},{"key":"window_inner_width","value":0},{"key":"window_inner_height","value":0},{"key":"window_outer_width","value":0},{"key":"window_outer_height","value":0},{"key":"browser_detection_firefox","value":true},{"key":"browser_detection_brave","value":false},{"key":"audio_codecs","value":"{\"ogg\":\"probably\",\"mp3\":\"maybe\",\"wav\":\"probably\",\"m4a\":\"maybe\",\"aac\":\"maybe\"}"},{"key":"video_codecs","value":"{\"ogg\":\"probably\",\"h264\":\"probably\",\"webm\":\"probably\",\"mpeg4v\":\"\",\"mpeg4a\":\"\",\"theora\":\"\"}"},{"key":"media_query_dark_mode","value":false},{"key":"headless_browser_phantom","value":false},{"key":"headless_browser_selenium","value":false},{"key":"headless_browser_nightmare_js","value":false},{"key":"document__referrer","value":""},{"key":"window__ancestor_origins","value":null},{"key":"window__tree_index","value":[1]},{"key":"window__tree_structure","value":"[[],[]]"},{"key":"window__location_href","value":"https://tcr9i.openai.com/v2/2.4.4/enforcement.f73f1debe050b423e0e5cd1845b2430a.html"},{"key":"client_config__sitedata_location_href","value":"https://auth0.openai.com/u/login/password"},{"key":"client_config__surl","value":"https://tcr9i.openai.com"},{"key":"mobile_sdk__is_sdk"},{"key":"client_config__language","value":null},{"key":"audio_fingerprint","value":"35.73833402246237"}]},{"key":"fe","value":["DNT:1","L:en-US","D:24","PR:1","S:0,0","AS:false","TO:0","SS:true","LS:true","IDB:true","B:false","ODB:false","CPUC:unknown","PK:Linux x86_64","CFP:330110783","FR:false","FOS:false","FB:false","JSF:Arial,Arial Narrow,Bitstream Vera Sans Mono,Bookman Old Style,Century Schoolbook,Courier,Courier New,Helvetica,MS Gothic,MS PGothic,Palatino,Palatino Linotype,Times,Times New Roman","P:Chrome PDF Viewer,Chromium PDF Viewer,Microsoft Edge PDF Viewer,PDF Viewer,WebKit built-in PDF","T:0,false,false","H:2","SWF:false"]},{"key":"ife_hash","value":"168007f8bc78f4e6048a29a16ab25b07"},{"key":"cs","value":1},{"key":"jsbd","value":"{\"HL\":2,\"NCE\":true,\"DT\":\"\",\"NWD\":\"false\",\"DOTO\":1,\"DMTO\":1}"}]`,
		base64.StdEncoding.EncodeToString([]byte(timestamp)))
	// var bt = new Date() ['getTime']() / 1000
	bt := time.Now().UnixMicro() / 1000000
	// bw = Math.round(bt - (bt % 21600)
	bw := strconv.FormatInt(bt-(bt%21600), 10)
	bv := "Mozilla/5.0 (Windows NT 10.0; rv:114.0) Gecko/20100101 Firefox/114.0"
	bda := Encrypt(bx, bv+bw)
	bda = base64.StdEncoding.EncodeToString([]byte(bda))
	blob := map[string]string{
		"blob": "eyJrZXkiOiJhcGlfdHlwZSIsInAiOjEsImYiOiI1NDc0MDk5ZDhiNGM0YjY5MDIyMDNhOTI5ZmZjMGJlYyIsIm4iOiI2NzIyN2FmYmZkMTlhNzQxYzdkYTE3MzIyMTgzMDFhYyIsIndoIjoiODg5ZTMzNzE5YmU4NmM0NTRlZTg3YmIwN2FjY2QxOTMiLCJlbmhhbmNlZF9mcCI6W3sia2V5Ijoid2Via2dsX2V4dGVuc2lvbnMiLCJ2YWx1ZSI6bnVsbCwia2V5Ijoic2V0dXAiLCJ2YWx1ZSI6bnVsbCwia2V5Ijoid2Via2dsX3JlbmRlciIsInZhbHVlIjpudWxsLCJrZXkiOiJ3ZWJnbF92ZW5kb3IiLCJ2YWx1ZSI6bnVsbCwia2V5Ijoid2Via2dsX3ZlcnNpb24iLCJ2YWx1ZSI6bnVsbCwia2V5Ijoid2Via2dsX3NoYWRpbmdfbGFuZ3VsYXNlX3ZlbnRvcCIsInZhbHVlIjpudWxsLCJrZXkiOiJ3ZWJnbF9iaXRzIiwidmFsdWUiOm51bGwsImtleSI6IndlYmdsX2JpdHNfIiwidmFsdWUiOm51bGwsImtleSI6IndlYmdsX2FudGllbGFzcyIsInZhbHVlIjpudWxsLCJrZXkiOiJ3ZWJnbF9iaXRzIiwidmFsdWUiOiJ3ZWJnbF9iaXRzIiwia2V5Ijoid2Via",
	}
	// 将结构体实例序列化为 JSON 字符串
	jsonData, err := json.Marshal(blob)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return nil, nil
	}
	form := url.Values{
		"bda":          {bda},
		"public_key":   {public_key},
		"site":         {"https://chat.openai.com"},
		"userbrowser":  {bv},
		"capi_version": {"2.4.4"},
		"capi_mode":    {"lightbox"},
		"style_theme":  {"default"},
		"rnd":          {strconv.FormatFloat(rand.Float64(), 'f', -1, 64)},
		"data":         {string(jsonData)},
	}
	req, _ := http.NewRequest(http.MethodPost, "https://tcr9i.openai.com/fc/gt2/public_key/"+public_key, strings.NewReader(form.Encode()))
	req.Header.Set("Host", "tcr9i.openai.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:114.0) Gecko/20100101 Firefox/114.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", "https://tcr9i.openai.com")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://tcr9i.openai.com/v2/2.4.4/enforcement.f73f1debe050b423e0e5cd1845b2430a.html")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("TE", "trailers")
	return &OpenAiRequest{
		Request: req,
		Client:  &client,
	}, nil
}

func NewOpenAiRequestV3(public_key string) (*OpenAiRequest, error) {
	// generate timestamp in 1687790752 format
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()/1000000000)
	bx := fmt.Sprintf(`[{"key":"api_type","value":"js"},{"key":"p","value":1},{"key":"f","value":"5474099d8b4c4b6902203a929ffc0bec"},{"key":"n","value":"%s"},{"key":"wh","value":"c625241942b18a7e6bf5c7dae8bd566b|72627afbfd19a741c7da1732218301ac"},{"key":"enhanced_fp","value":[{"key":"webgl_extensions","value":null},{"key":"webgl_extensions_hash","value":null},{"key":"webgl_renderer","value":null},{"key":"webgl_vendor","value":null},{"key":"webgl_version","value":null},{"key":"webgl_shading_language_version","value":null},{"key":"webgl_aliased_line_width_range","value":null},{"key":"webgl_aliased_point_size_range","value":null},{"key":"webgl_antialiasing","value":null},{"key":"webgl_bits","value":null},{"key":"webgl_max_params","value":null},{"key":"webgl_max_viewport_dims","value":null},{"key":"webgl_unmasked_vendor","value":null},{"key":"webgl_unmasked_renderer","value":null},{"key":"webgl_vsf_params","value":null},{"key":"webgl_vsi_params","value":null},{"key":"webgl_fsf_params","value":null},{"key":"webgl_fsi_params","value":null},{"key":"webgl_hash_webgl","value":null},{"key":"user_agent_data_brands","value":null},{"key":"user_agent_data_mobile","value":null},{"key":"navigator_connection_downlink","value":null},{"key":"navigator_connection_downlink_max","value":null},{"key":"network_info_rtt","value":null},{"key":"network_info_save_data","value":null},{"key":"network_info_rtt_type","value":null},{"key":"screen_pixel_depth","value":24},{"key":"navigator_device_memory","value":null},{"key":"navigator_languages","value":"en-US,en"},{"key":"window_inner_width","value":0},{"key":"window_inner_height","value":0},{"key":"window_outer_width","value":0},{"key":"window_outer_height","value":0},{"key":"browser_detection_firefox","value":true},{"key":"browser_detection_brave","value":false},{"key":"audio_codecs","value":"{\"ogg\":\"probably\",\"mp3\":\"maybe\",\"wav\":\"probably\",\"m4a\":\"maybe\",\"aac\":\"maybe\"}"},{"key":"video_codecs","value":"{\"ogg\":\"probably\",\"h264\":\"probably\",\"webm\":\"probably\",\"mpeg4v\":\"\",\"mpeg4a\":\"\",\"theora\":\"\"}"},{"key":"media_query_dark_mode","value":false},{"key":"headless_browser_phantom","value":false},{"key":"headless_browser_selenium","value":false},{"key":"headless_browser_nightmare_js","value":false},{"key":"document__referrer","value":""},{"key":"window__ancestor_origins","value":null},{"key":"window__tree_index","value":[1]},{"key":"window__tree_structure","value":"[[],[]]"},{"key":"window__location_href","value":"https://tcr9i.openai.com/v2/2.4.4/enforcement.f73f1debe050b423e0e5cd1845b2430a.html"},{"key":"client_config__sitedata_location_href","value":"https://auth0.openai.com/u/login/password"},{"key":"client_config__surl","value":"https://tcr9i.openai.com"},{"key":"mobile_sdk__is_sdk"},{"key":"client_config__language","value":null},{"key":"audio_fingerprint","value":"35.73833402246237"}]},{"key":"fe","value":["DNT:1","L:en-US","D:24","PR:1","S:0,0","AS:false","TO:0","SS:true","LS:true","IDB:true","B:false","ODB:false","CPUC:unknown","PK:Linux x86_64","CFP:330110783","FR:false","FOS:false","FB:false","JSF:Arial,Arial Narrow,Bitstream Vera Sans Mono,Bookman Old Style,Century Schoolbook,Courier,Courier New,Helvetica,MS Gothic,MS PGothic,Palatino,Palatino Linotype,Times,Times New Roman","P:Chrome PDF Viewer,Chromium PDF Viewer,Microsoft Edge PDF Viewer,PDF Viewer,WebKit built-in PDF","T:0,false,false","H:2","SWF:false"]},{"key":"ife_hash","value":"168007f8bc78f4e6048a29a16ab25b07"},{"key":"cs","value":1},{"key":"jsbd","value":"{\"HL\":2,\"NCE\":true,\"DT\":\"\",\"NWD\":\"false\",\"DOTO\":1,\"DMTO\":1}"}]`,
		base64.StdEncoding.EncodeToString([]byte(timestamp)))
	// var bt = new Date() ['getTime']() / 1000
	bt := time.Now().UnixMicro() / 1000000
	// bw = Math.round(bt - (bt % 21600)
	bw := strconv.FormatInt(bt-(bt%21600), 10)
	bv := "Mozilla/5.0 (Windows NT 10.0; rv:114.0) Gecko/20100101 Firefox/114.0"
	bda := Encrypt(bx, bv+bw)
	bda = base64.StdEncoding.EncodeToString([]byte(bda))
	form := url.Values{
		"bda":          {bda},
		"public_key":   {public_key},
		"site":         {"https://auth0.openai.com"},
		"userbrowser":  {bv},
		"capi_version": {"2.4.4"},
		"capi_mode":    {"lightbox"},
		"style_theme":  {"default"},
		"rnd":          {strconv.FormatFloat(rand.Float64(), 'f', -1, 64)},
	}
	req, _ := http.NewRequest(http.MethodPost, "https://tcr9i.openai.com/fc/gt2/public_key/"+public_key, strings.NewReader(form.Encode()))
	req.Header.Set("Host", "tcr9i.openai.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:114.0) Gecko/20100101 Firefox/114.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Origin", "https://tcr9i.openai.com")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://tcr9i.openai.com/v2/2.4.4/enforcement.f73f1debe050b423e0e5cd1845b2430a.html")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("cookie", "cf_clearance=D.EBZ2jX13NDDpM10s4wESoPaskAS_fFv_oW_LdQk_w-1711383352-1.0.1.1-Xr90T32l_psnb6yuImeuQOo17Ly5WaJuSxEHxHxjk2VZ_gu3VBd47epUngs.qBOf.nDkQQVn.TeSuXTAGgOv5g;")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("TE", "trailers")
	return &OpenAiRequest{
		Request: req,
		Client:  &client,
	}, nil
}

func SendRequest(ipv6 string, mtype string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if mtype != "login" {
		return msgToken(ipv6)
	}
	return loginToken(ipv6)
}

func msgToken(ipv6 string) (string, error) {
	if ipv6 != "" {
		client.SetProxy(ipv6)
	}
	req, err := NewOpenAiRequestV1("35536E1E-65B4-4D96-9D97-6ADB7EFF8147")
	if err != nil {
		return "", err
	}
	token, err := req.GetToken()
	if err != nil {
		return "", err
	}
	return token, nil
}

func loginToken(ipv6 string) (string, error) {
	if ipv6 != "" {
		client.SetProxy(ipv6)
	}
	req, err := NewOpenAiRequestV3("0A1D34FC-659D-4E23-B17B-694DCFCF6A6C")
	if err != nil {
		return "", err
	}
	return req.GetToken()
}

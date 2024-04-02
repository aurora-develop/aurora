package bard

// By @mosajjal at https://github.com/mosajjal/bard-cli/blob/main/bard/bard.go
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/util/rand"

	nhttp "net/http"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	resty "github.com/go-resty/resty/v2"
)

var (
	jar     = tls_client.NewCookieJar()
	options = []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(360),
		tls_client.WithClientProfile(tls_client.Safari_IOS_15_5),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar), // create cookieJar instance and pass it as argument
		// Disable SSL verification
		tls_client.WithInsecureSkipVerify(),
	}
	client, _    = tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	http_proxy   = os.Getenv("http_proxy")
	resty_client = resty.New()
)

var headers map[string]string = map[string]string{
	"Host":          "bard.google.com",
	"X-Same-Domain": "1",
	"User-Agent":    "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.4472.114 Safari/537.36",
	"Content-Type":  "application/x-www-form-urlencoded;charset=UTF-8",
	"Origin":        "https://bard.google.com",
	"Referer":       "https://bard.google.com/",
}

func init() {
	if http_proxy != "" {
		client.SetProxy(http_proxy)
	}
}

const bardURL string = "https://bard.google.com/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate"

type Answer struct {
	Content string `json:"content"`
	// FactualityQueries []string `json:"factualityQueries"`
	Choices []string `json:"choices"`
}

// Bard is the main struct for the Bard AI
type Bard struct {
	Cookie              string
	ChoiceID            string
	ConversationID      string
	ResponseID          string
	SNlM0e              string
	LastInteractionTime time.Time
}

// New creates a new Bard AI instance. Cookie is the __Secure-1PSID cookie from Google
func New(cookie string) (*Bard, error) {
	b := &Bard{
		Cookie: cookie,
	}
	err := b.getSNlM0e()
	return b, err
}

func (b *Bard) getSNlM0e() error {
	req, _ := http.NewRequest("GET", "https://bard.google.com/", nil)
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	req.AddCookie(&http.Cookie{
		Name:  "__Secure-1PSID",
		Value: b.Cookie,
	})
	// in response text, the value shows. in python:
	r := regexp.MustCompile(`SNlM0e\":\"(.*?)\"`)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	tmpValues := r.FindStringSubmatch(string(body))
	if len(tmpValues) < 2 {
		return fmt.Errorf("failed to find snim0e value. possibly misconfigured cookies?")
	}
	b.SNlM0e = tmpValues[1]
	return nil
}

// Ask generates a Bard AI response and returns it to the user
func (b *Bard) Ask(prompt string) (*Answer, error) {
	b.LastInteractionTime = time.Now()

	// req paramters for the actual request
	reqParams := map[string]string{
		"bl":     "boq_assistant-bard-web-server_20230606.12_p0",
		"_reqid": fmt.Sprintf("%d", rand.IntnRange(100000, 999999)),
		"rt":     "c",
	}

	req := fmt.Sprintf(`[null, "[[\"%s\"], null, [\"%s\", \"%s\", \"%s\"]]"]`,
		//prompt, b.answer.ConversationID, b.answer.ResponseID, b.answer.ChoiceID)
		prompt, b.ConversationID, b.ResponseID, b.ChoiceID)

	reqData := map[string]string{
		"f.req": string(req),
		"at":    b.SNlM0e,
	}
	resty_client.SetHeaders(headers)
	resty_client.SetCookie(&nhttp.Cookie{
		Name:  "__Secure-1PSID",
		Value: b.Cookie,
	})
	resty_client.SetBaseURL(bardURL)
	resty_client.SetTimeout(60 * time.Second)
	resty_client.SetFormData(reqData)
	resty_client.SetQueryParams(reqParams)
	resty_client.SetDoNotParseResponse(true)
	resp, err := resty_client.R().Post("")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		// curl, _ := http2curl.GetCurlCommand(resp.Request.EnableTrace().RawRequest)
		// fmt.Println(curl)
		return nil, fmt.Errorf("status code is not 200: %d", resp.StatusCode())
	}

	// this is the Go version
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.RawResponse.Body)

	respLines := strings.Split(buf.String(), "\n")
	respJSON := respLines[3]

	var fullRes [][]interface{}
	err = json.Unmarshal([]byte(respJSON), &fullRes)
	if err != nil {
		return nil, err
	}

	// get the main answer
	res, ok := fullRes[0][2].(string)
	if !ok {
		return nil, fmt.Errorf("failed to get answer from bard")
	}

	answer := Answer{}

	answer.Content = gjson.Get(res, "0.0").String()
	b.ConversationID = gjson.Get(res, "1.0").String()
	b.ResponseID = gjson.Get(res, "1.1").String()
	choices := gjson.Get(res, "4").Array()
	answer.Choices = make([]string, len(choices))
	for i, choice := range choices {
		answer.Choices[i] = choice.Array()[0].String()
	}
	b.ChoiceID = choices[0].Array()[0].String()

	return &answer, nil
}

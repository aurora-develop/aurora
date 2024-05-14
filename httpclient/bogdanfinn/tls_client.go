package bogdanfinn

import (
	"aurora/httpclient"
	"io"
	"net/http"
	"net/url"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type TlsClient struct {
	Client    tls_client.HttpClient
	ReqBefore handler
}

type handler func(r *fhttp.Request) error

func NewStdClient() *TlsClient {
	client, _ := tls_client.NewHttpClient(tls_client.NewNoopLogger(), []tls_client.HttpClientOption{
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
		tls_client.WithTimeoutSeconds(600),
		tls_client.WithClientProfile(profiles.Chrome_117),
	}...)

	stdClient := &TlsClient{Client: client}
	return stdClient
}

func convertResponse(resp *fhttp.Response) *http.Response {
	response := &http.Response{
		Status:           resp.Status,
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           http.Header(resp.Header),
		Body:             resp.Body,
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          http.Header(resp.Trailer),
	}
	return response
}

func (t *TlsClient) handleHeaders(req *fhttp.Request, headers httpclient.AuroraHeaders) {
	if headers == nil {
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func (t *TlsClient) handleCookies(req *fhttp.Request, cookies []*http.Cookie) {
	if cookies == nil {
		return
	}
	for _, c := range cookies {
		req.AddCookie(&fhttp.Cookie{
			Name:       c.Name,
			Value:      c.Value,
			Path:       c.Path,
			Domain:     c.Domain,
			Expires:    c.Expires,
			RawExpires: c.RawExpires,
			MaxAge:     c.MaxAge,
			Secure:     c.Secure,
			HttpOnly:   c.HttpOnly,
			SameSite:   fhttp.SameSite(c.SameSite),
			Raw:        c.Raw,
			Unparsed:   c.Unparsed,
		})
	}
}

func (t *TlsClient) Request(method httpclient.HttpMethod, url string, headers httpclient.AuroraHeaders, cookies []*http.Cookie, body io.Reader) (*http.Response, error) {
	req, err := fhttp.NewRequest(string(method), url, body)
	if err != nil {
		return nil, err
	}
	t.handleHeaders(req, headers)
	t.handleCookies(req, cookies)
	if t.ReqBefore != nil {
		if err := t.ReqBefore(req); err != nil {
			return nil, err
		}
	}
	do, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	return convertResponse(do), nil
}

func (t *TlsClient) SetProxy(url string) error {
	return t.Client.SetProxy(url)
}

func (t *TlsClient) SetCookies(rawUrl string, cookies []*http.Cookie) {
	if cookies == nil {
		return
	}
	u, err := url.Parse(rawUrl)
	if err != nil {
		return
	}
	var fcookies []*fhttp.Cookie
	for _, c := range cookies {
		fcookies = append(fcookies, &fhttp.Cookie{
			Name:       c.Name,
			Value:      c.Value,
			Path:       c.Path,
			Domain:     c.Domain,
			Expires:    c.Expires,
			RawExpires: c.RawExpires,
			MaxAge:     c.MaxAge,
			Secure:     c.Secure,
			HttpOnly:   c.HttpOnly,
			SameSite:   fhttp.SameSite(c.SameSite),
			Raw:        c.Raw,
			Unparsed:   c.Unparsed,
		})
	}
	t.Client.GetCookieJar().SetCookies(u, fcookies)
}

func (t *TlsClient) GetCookies(rawUrl string) []*http.Cookie {
	currUrl, err := url.Parse(rawUrl)
	if err != nil {
		return nil
	}

	var cookies []*http.Cookie
	for _, c := range t.Client.GetCookies(currUrl) {
		cookies = append(cookies, &http.Cookie{
			Name:       c.Name,
			Value:      c.Value,
			Path:       c.Path,
			Domain:     c.Domain,
			Expires:    c.Expires,
			RawExpires: c.RawExpires,
			MaxAge:     c.MaxAge,
			Secure:     c.Secure,
			HttpOnly:   c.HttpOnly,
			SameSite:   http.SameSite(c.SameSite),
		})
	}
	return cookies
}


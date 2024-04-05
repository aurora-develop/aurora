package httpclient

import (
	"io"
	"net/http"
)

type AuroraHttpClient interface {
	Request(method HttpMethod, url string, headers AuroraHeaders, cookies []*http.Cookie, body io.Reader) (*http.Response, error)
	SetProxy(url string) error
}

type HttpMethod string

const (
	GET     HttpMethod = "GET"
	POST    HttpMethod = "POST"
	PUT     HttpMethod = "PUT"
	HEAD    HttpMethod = "HEAD"
	DELETE  HttpMethod = "DELETE"
	OPTIONS HttpMethod = "OPTIONS"
)

type AuroraHeaders map[string]string

func (a AuroraHeaders) Set(key, value string) {
	a[key] = value
}

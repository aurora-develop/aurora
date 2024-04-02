package httpclient

import (
	"crypto/tls"
	"net/http"
	"time"
)

type StdClient struct {
	httpClient *http.Client
}

func NewStdClient() *StdClient {
	return &StdClient{
		httpClient: &http.Client{
			Timeout: time.Second * 10,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (c *StdClient) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

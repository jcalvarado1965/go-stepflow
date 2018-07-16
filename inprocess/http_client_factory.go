package inprocess

import (
	"context"
	"crypto/tls"
	"net/http"

	stepflow "github.com/jcalvarado1965/go-stepflow"
)

type httpClientFactory struct{}

// NewHTTPClientFactory creates an http client factory
func NewHTTPClientFactory() stepflow.HTTPClientFactory {
	return &httpClientFactory{}
}

func (h *httpClientFactory) GetHTTPClient(ctx context.Context, disableSSLValidation bool) *http.Client {
	if disableSSLValidation {
		transport := http.DefaultTransport.(*http.Transport)
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		return &http.Client{Transport: transport}
	} else {
		return &http.Client{}
	}
}

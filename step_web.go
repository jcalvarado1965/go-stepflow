package stepflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

var validMethods = map[string]struct{}{
	http.MethodGet:    struct{}{},
	http.MethodPut:    struct{}{},
	http.MethodPost:   struct{}{},
	http.MethodDelete: struct{}{},
}

// WebMethodStep describes a step that makes an HTTP request, and sends
// the response to the Next step
type WebMethodStep struct {
	BaseStep
	Method string `json:"method,omitempty"`
	URL    string `json:"url,omitempty"`
}

// PrepareMarshal sets the step type
func (s *WebMethodStep) PrepareMarshal() {
	s.BaseStep.PrepareMarshal()
	s.Type = TypeWebMethod
}

// Validate checks the method and URL are OK
func (s *WebMethodStep) Validate() []error {
	errs := []error{}
	if _, ok := validMethods[s.Method]; !ok {
		errs = append(errs, fmt.Errorf("%s is not a valid method", s.Method))
	}

	if u, err := url.Parse(s.URL); err != nil {
		errs = append(errs, fmt.Errorf("%s is not a valid URL: %s", s.URL, err.Error()))
	} else if !u.IsAbs() {
		errs = append(errs, fmt.Errorf("%s is not an absolute URL", s.URL))
	}
	return errs
}

// Do implements DoerStep interface
func (s *WebMethodStep) Do(ctx context.Context, exec Executor, flow *Flow) error {
	client := exec.GetHTTPClientFactory().GetHTTPClient(ctx, false)

	var body io.Reader
	// if PUT or POST, figure content type and body
	if s.Method == http.MethodPut || s.Method == http.MethodPost {
		switch data := flow.Data.(type) {
		case string:
			body = strings.NewReader(data)
		case []byte:
			body = bytes.NewReader(data)
		case json.RawMessage:
			body = bytes.NewReader([]byte(data))
		default:
			// try to serialize to JSON
			jsonBytes, err := json.Marshal(data)
			if err != nil {
				return errors.New("Unable to marshal data for web method")
			}
			body = bytes.NewBuffer(jsonBytes)
		}
	}
	req, err := http.NewRequest(s.Method, s.URL, body)
	if err != nil {
		return err
	}

	if flow.ContentType != "" {
		req.Header.Add("Content-Type", flow.ContentType)
	}

	exec.GetLogger().Debugf(ctx, "Calling web URL %s %s", s.Method, s.URL)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", resp.Status, string(bodyBytes))
	}

	if flow.ContentType == "" {
		flow.ContentType = resp.Header.Get("Content-Type")
	}

	if strings.HasPrefix(strings.ToLower(flow.ContentType), "text/") {
		flow.Data = string(bodyBytes)
	} else if flow.ContentType == "application/json" {
		flow.Data = json.RawMessage(bodyBytes)
	} else {
		flow.Data = bodyBytes
	}

	exec.GetLogger().Debugf(ctx, "Got web data %s", flow.Data)

	return err
}

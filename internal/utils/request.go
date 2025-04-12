package utils

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/ruizlenato/smudgelord/internal/config"
)

type HTTPCaller struct {
	Client *http.Client
}

var DefaultHTTPCaller = &HTTPCaller{
	Client: &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: 1024,
		},
	},
}

func (a HTTPCaller) Call(url string, params RequestParams) (*http.Response, error) {
	req, err := http.NewRequest(params.Method, url, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range params.Headers {
		req.Header.Set(key, value)
	}

	switch params.Method {
	case http.MethodGet, http.MethodOptions:
		q := req.URL.Query()
		for key, value := range params.Query {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	case http.MethodPost:
		body := strings.Join(params.BodyString, "&")
		req.Body = io.NopCloser(strings.NewReader(body))
	}

	if params.Proxy {
		req.URL.Scheme = "http"
		req.URL.Host = config.Socks5Proxy
	}

	var resp *http.Response
	if params.Redirects > 0 {
		client := *a.Client
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= params.Redirects {
				return http.ErrUseLastResponse
			}
			return nil
		}
		resp, err = client.Do(req)
	} else {
		resp, err = a.Client.Do(req)
	}

	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return resp, nil
		}
		return nil, fmt.Errorf("request error: %w", err)
	}

	return resp, nil
}

type RequestParams struct {
	Method     string
	Redirects  int
	Proxy      bool
	Headers    map[string]string
	Query      map[string]string
	BodyString []string
}

func Request(url string, params RequestParams) (*http.Response, error) {
	resp, err := DefaultHTTPCaller.Call(url, params)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type RetryCaller struct {
	Caller       *HTTPCaller
	MaxAttempts  int
	ExponentBase float64
	StartDelay   time.Duration
	MaxDelay     time.Duration
}

var ErrMaxRetryAttempts = errors.New("max retry attempts reached")

func (r *RetryCaller) Request(url string, params RequestParams) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i < r.MaxAttempts; i++ {
		resp, err = r.Caller.Call(url, params)
		if err == nil {
			return resp, nil
		}

		if i == r.MaxAttempts-1 {
			break
		}

		delay := time.Duration(math.Pow(r.ExponentBase, float64(i))) * r.StartDelay
		if delay > r.MaxDelay {
			delay = r.MaxDelay
		}
		time.Sleep(delay)
	}

	return nil, errors.Join(err, ErrMaxRetryAttempts)
}

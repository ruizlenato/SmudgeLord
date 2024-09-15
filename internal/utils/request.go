package utils

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

type FastHTTPCaller struct {
	Client *fasthttp.Client
}

var DefaultFastHTTPCaller = &FastHTTPCaller{
	Client: &fasthttp.Client{
		ReadBufferSize:  16 * 1024,
		MaxConnsPerHost: 1024,
	},
}

func (a FastHTTPCaller) Call(url string, params RequestParams) (*fasthttp.Request, *fasthttp.Response, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	req.Header.SetMethod(params.Method)
	for key, value := range params.Headers {
		req.Header.Set(key, value)
	}

	switch params.Method {
	case fasthttp.MethodGet, fasthttp.MethodOptions:
		req.SetRequestURI(url)
		for key, value := range params.Query {
			req.URI().QueryArgs().Add(key, value)
		}
	case fasthttp.MethodPost:
		req.SetBodyString(strings.Join(params.BodyString, "&"))
		req.SetRequestURI(url)
	default:
		return nil, nil, fmt.Errorf("unsupported method: %s", params.Method)
	}

	var err error
	if params.Redirects > 0 {
		err = a.Client.DoRedirects(req, resp, params.Redirects)
	} else {
		err = a.Client.Do(req, resp)
	}

	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return req, resp, nil
		}
		return nil, nil, fmt.Errorf("request error: %w", err)
	}

	return req, resp, nil
}

type RequestParams struct {
	Method     string            // "GET", "OPTIONS" or "POST"
	Redirects  int               // Number of redirects to follow
	Proxy      bool              // Use proxy for the request
	Headers    map[string]string // Common headers for both GET and POST
	Query      map[string]string // Query parameters for GET
	BodyString []string          // Body of the request for POST
}

func Request(Link string, params RequestParams) (*fasthttp.Request, *fasthttp.Response, error) {
	if params.Proxy {
		DefaultFastHTTPCaller.Client.Dial = fasthttpproxy.FasthttpSocksDialer(config.Socks5Proxy)
	}

	req, resp, err := DefaultFastHTTPCaller.Call(Link, params)
	if err != nil {
		return nil, nil, errors.Join(err)
	}

	return req, resp, nil
}

type RetryCaller struct {
	Caller       *FastHTTPCaller
	MaxAttempts  int
	ExponentBase float64
	StartDelay   time.Duration
	MaxDelay     time.Duration
}

var ErrMaxRetryAttempts = errors.New("max retry attempts reached")

func (r *RetryCaller) Request(url string, params RequestParams) (*fasthttp.Request, *fasthttp.Response, error) {
	var req *fasthttp.Request
	var resp *fasthttp.Response
	var err error

	for i := 0; i < r.MaxAttempts; i++ {
		req, resp, err = r.Caller.Call(url, params)
		if err == nil {
			return req, resp, nil
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

	return nil, nil, errors.Join(err, ErrMaxRetryAttempts)
}

func ReleaseRequestResources(request *fasthttp.Request, response *fasthttp.Response) {
	if request != nil {
		defer fasthttp.ReleaseRequest(request)
	}
	if response != nil {
		defer fasthttp.ReleaseResponse(response)
	}
}

package utils

import (
	"log"
	"strings"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

type RequestParams struct {
	Method     string            // "GET", "OPTIONS" or "POST"
	Redirects  int               // Number of redirects to follow
	Proxy      bool              // Use proxy for the request
	Headers    map[string]string // Common headers for both GET and POST
	Query      map[string]string // Query parameters for GET
	BodyString []string          // Body of the request for POST
}

// Request sends a GET, OPTIONS or POST request to the specified link with the provided parameters and returns the response.
// The Link specifies the URL to send the request to.
// The params contain additional parameters for the request, such as headers, query parameters, and body.
// The Method field in params should be "GET" or "POST" to indicate the type of request.
//
// Example usage:
//
//	response := Request("https://api.example.com/users", RequestParams{
//		Method: "GET",
//		Headers: map[string]string{
//			"Authorization": "Bearer your-token",
//		},
//		Query: map[string]string{
//			"page":  "1",
//			"limit": "10",
//		},
//	})
//
//	response := Request("https://example.com/api", RequestParams{
//		Method: "POST",
//		Headers: map[string]string{
//			"Content-Type": "application/json",
//		},
//		BodyString: []string{
//			"param1=value1",
//			"param2=value2",
//		},
//	})
func Request(Link string, params RequestParams) (*fasthttp.Request, *fasthttp.Response) {
	request := fasthttp.AcquireRequest()
	response := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(response)

	client := &fasthttp.Client{
		ReadBufferSize:  16 * 1024,
		MaxConnsPerHost: 1024,
	}
	if params.Proxy {
		client.Dial = fasthttpproxy.FasthttpSocksDialer(config.Socks5Proxy)
	}

	request.Header.SetMethod(params.Method)
	for key, value := range params.Headers {
		request.Header.Set(key, value)
	}

	if params.Method == fasthttp.MethodGet {
		request.SetRequestURI(Link)
		for key, value := range params.Query {
			request.URI().QueryArgs().Add(key, value)
		}
	} else if params.Method == fasthttp.MethodOptions {
		request.SetRequestURI(Link)
		for key, value := range params.Query {
			request.URI().QueryArgs().Add(key, value)
		}
	} else if params.Method == fasthttp.MethodPost {
		request.SetBodyString(strings.Join(params.BodyString, "&"))
		request.SetRequestURI(Link)
	} else {
		log.Print("[request/Request] Error: Unsupported method ", params.Method)
		return request, response
	}

	var err error
	if params.Redirects > 0 {
		err = client.DoRedirects(request, response, 10)
	} else {
		err = client.Do(request, response)
	}
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return request, response
		}
		log.Print("[request/Request] Error: ", err)
	}
	return request, response
}

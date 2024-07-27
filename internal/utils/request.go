package utils

import (
	"log"
	"strings"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

type RequestParams struct {
	Method     string            // "GET", "OPTIONS" or "POST"
	Proxy      string            // Proxy URL
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
func Request(Link string, params RequestParams) *fasthttp.Response {
	request := fasthttp.AcquireRequest()
	response := fasthttp.AcquireResponse()

	client := &fasthttp.Client{
		ReadBufferSize:  16 * 1024,
		MaxConnsPerHost: 1024,
	}
	if params.Proxy != "" {
		client.Dial = fasthttpproxy.FasthttpSocksDialer(params.Proxy)
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
		return response
	}

	err := client.Do(request, response)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return response
		}
		log.Print("[request/Request] Error: ", err)
	}
	return response
}

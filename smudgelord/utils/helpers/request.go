package helpers

import (
	"log"
	"strings"

	"github.com/valyala/fasthttp"
)

type RequestGETParams struct {
	Headers map[string]string
	Query   map[string]string
}

type RequestPOSTParams struct {
	Headers    map[string]string
	BodyString []string
}

// RequestGET sends a GET request to the specified link with the provided parameters and returns the response.
// The Link parameter specifies the URL to send the request to.
// The params parameter contains additional parameters for the request, such as headers and query parameters.
// The function returns a pointer to a fasthttp.Response object representing the response received from the server.
//
// Exemple usage:
//
//	response := helpers.RequestGET("https://api.example.com/users", helpers.RequestGETParams{
//		Headers: map[string]string{
//			"Authorization": "Bearer your-token",
//		},
//		Query: map[string]string{
//			"page":  "1",
//			"limit": "10",
//		},
//	})
func RequestGET(Link string, params RequestGETParams) *fasthttp.Response {
	request := fasthttp.AcquireRequest()
	response := fasthttp.AcquireResponse()

	client := &fasthttp.Client{ReadBufferSize: 16 * 1024}

	request.Header.SetMethod(fasthttp.MethodGet)
	for key, value := range params.Headers {
		request.Header.Set(key, value)
	}

	request.SetRequestURI(Link)
	for key, value := range params.Query {
		request.URI().QueryArgs().Add(key, value)
	}

	err := client.Do(request, response)
	if err != nil {
		log.Println(err)
	}

	return response
}

// RequestPOST sends a POST request to the specified link with the given parameters and returns the response.
// It takes a `Link` string parameter representing the URL to send the request to, and a `params` parameter of type `RequestPOSTParams`
// which contains the headers and body string for the request.
// The function returns a pointer to a `fasthttp.Response` object representing the response received from the server.
//
// Example usage:
//
//	response := RequestPOST("https://example.com/api", RequestPOSTParams{
//	  Headers: map[string]string{
//	    "Content-Type": "application/json",
//	  },
//	  BodyString: []string{
//	    "param1=value1",
//	    "param2=value2",
//	  },
//	})
func RequestPOST(Link string, params RequestPOSTParams) *fasthttp.Response {
	request := fasthttp.AcquireRequest()
	response := fasthttp.AcquireResponse()

	client := &fasthttp.Client{ReadBufferSize: 16 * 1024,
		MaxConnsPerHost: 1024}

	request.Header.SetMethod(fasthttp.MethodPost)
	for key, value := range params.Headers {
		request.Header.Set(key, value)
	}

	request.SetBodyString(strings.Join(params.BodyString, "&"))
	request.SetRequestURI(Link)

	err := client.Do(request, response)
	if err != nil {
		log.Println(err)
	}

	return response
}

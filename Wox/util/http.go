package util

import (
	"errors"
	"fmt"
	"github.com/levigross/grequests"
)

type RequestMethod string

const (
	GET    RequestMethod = "GET"
	POST   RequestMethod = "POST"
	DELETE RequestMethod = "DELETE"
	PUT    RequestMethod = "PUT"
)

func Get(url string) (response []byte, err error) {
	return getRawResponse(GET, url, nil)
}

func Post(url string) (response []byte, err error) {
	return getRawResponse(POST, url, nil)
}

func getRawResponse(method RequestMethod, url string, requestOptions *grequests.RequestOptions) (response []byte, err error) {
	var httpResponse *grequests.Response
	httpResponse, err = doHttpRequest(method, url, requestOptions)
	if err != nil {
		return
	}

	if httpResponse.RawResponse != nil {
		defer httpResponse.Close()
	}
	if (httpResponse.StatusCode >= 200 && httpResponse.StatusCode <= 204) || (httpResponse.StatusCode >= 400 && httpResponse.StatusCode <= 600) {
		response = httpResponse.Bytes()
	} else {
		err = errors.New(fmt.Sprintf("http status code: %d, response: %s", httpResponse.StatusCode, string(httpResponse.Bytes())))
	}

	return
}

func doHttpRequest(method RequestMethod, url string, requestOptions *grequests.RequestOptions) (httpResponse *grequests.Response, err error) {
	switch method {
	case GET:
		httpResponse, err = grequests.Get(url, requestOptions)
	case POST:
		httpResponse, err = grequests.Post(url, requestOptions)
	case DELETE:
		httpResponse, err = grequests.Delete(url, requestOptions)
	case PUT:
		httpResponse, err = grequests.Put(url, requestOptions)
	default:
		err = errors.New(fmt.Sprintf("unknown method: %s", method))
	}

	return
}

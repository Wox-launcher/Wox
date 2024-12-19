package util

import (
	"context"

	"github.com/go-resty/resty/v2"
)

var client *resty.Client

func HttpGet(ctx context.Context, url string) ([]byte, error) {
	resp, err := getClient().R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, resp.Error().(error)
	}

	return resp.Body(), nil
}

func HttpPost(ctx context.Context, url string, body any) ([]byte, error) {
	resp, err := getClient().R().SetBody(body).Post(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, resp.Error().(error)
	}

	return resp.Body(), nil
}

func HttpDownload(ctx context.Context, url string, dest string) error {
	resp, err := getClient().R().SetOutput(dest).Get(url)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return resp.Error().(error)
	}

	return nil
}

func getClient() *resty.Client {
	if client == nil {
		client = resty.New()
		client.SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	}

	return client
}

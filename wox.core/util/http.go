package util

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
)

var client *resty.Client

func HttpGet(ctx context.Context, url string) ([]byte, error) {
	resp, err := getClient().R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("http get %s failed, status code: %d, error: %s", url, resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

func HttpGetWithHeaders(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	resp, err := getClient().R().SetHeaders(headers).Get(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("http get %s failed, status code: %d, error: %s", url, resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

func HttpPost(ctx context.Context, url string, body any) ([]byte, error) {
	resp, err := getClient().R().SetBody(body).Post(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("http post %s failed, status code: %d, error: %s", url, resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}

func HttpDownload(ctx context.Context, url string, dest string) error {
	resp, err := getClient().R().SetOutput(dest).Get(url)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("http download %s failed, status code: %d, error: %s", url, resp.StatusCode(), resp.String())
	}

	return nil
}

func getClient() *resty.Client {
	if client == nil {
		client = resty.New()
		client.SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36")
	}

	return client
}

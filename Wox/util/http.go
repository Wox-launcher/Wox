package util

import (
	"context"
	"github.com/go-resty/resty/v2"
)

func HttpGet(ctx context.Context, url string) ([]byte, error) {
	client := resty.New()
	resp, err := client.R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, resp.Error().(error)
	}

	return resp.Body(), nil
}

func HttpDownload(ctx context.Context, url string, dest string) error {
	client := resty.New()
	resp, err := client.R().SetOutput(dest).Get(url)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return resp.Error().(error)
	}

	return nil
}

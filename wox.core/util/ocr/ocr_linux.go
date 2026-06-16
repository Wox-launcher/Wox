//go:build linux

package ocr

import "context"

func recognizePlatform(ctx context.Context, request Request) (Result, error) {
	return Result{}, ErrUnsupported
}

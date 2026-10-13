//go:build !windows

package filesearchservice

import (
	"context"
	"fmt"
)

func Serve(context.Context, string, RequestHandler) error {
	return fmt.Errorf("file index service is only available on Windows")
}

func Health(context.Context) (Response, error) {
	return Response{}, fmt.Errorf("file index service is only available on Windows")
}

func Update(context.Context, string, string, string) error {
	return fmt.Errorf("file index service is only available on Windows")
}

package util

import (
	"context"
	"github.com/saracen/fastzip"
)

func Unzip(source, destination string) error {
	e, err := fastzip.NewExtractor(source, destination)
	if err != nil {
		return err
	}
	defer e.Close()

	if err = e.Extract(context.Background()); err != nil {
		return err
	}

	return nil
}

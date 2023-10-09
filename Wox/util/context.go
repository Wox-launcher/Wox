package util

import "context"
import "github.com/google/uuid"

func NewTraceContext() context.Context {
	return context.WithValue(context.Background(), "trace", uuid.NewString())
}
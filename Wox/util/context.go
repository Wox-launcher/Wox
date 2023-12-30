package util

import "context"
import "github.com/google/uuid"

func NewTraceContext() context.Context {
	return NewTraceContextWith(uuid.NewString())
}

func NewTraceContextWith(traceId string) context.Context {
	return context.WithValue(context.Background(), "trace", traceId)
}

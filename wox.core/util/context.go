package util

import "context"
import "github.com/google/uuid"

const (
	ContextKeyTraceId       = "trace"
	ContextKeyComponentName = "component"
)

func NewTraceContext() context.Context {
	return NewTraceContextWith(uuid.NewString())
}

func NewComponentContext(ctx context.Context, componentName string) context.Context {
	return context.WithValue(ctx, ContextKeyComponentName, componentName)
}

func NewTraceContextWith(traceId string) context.Context {
	return context.WithValue(context.Background(), ContextKeyTraceId, traceId)
}

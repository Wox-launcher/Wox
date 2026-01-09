package util

import (
	"context"

	"github.com/google/uuid"
)

const (
	ContextKeyTraceId       = "trace"
	ContextKeyComponentName = "component"
	ContextKeySessionId     = "session"
	ContextKeyQueryId       = "query"
)

func NewTraceContext() context.Context {
	return NewTraceContextWith(uuid.NewString())
}

func NewTraceContextWith(traceId string) context.Context {
	return context.WithValue(context.Background(), ContextKeyTraceId, traceId)
}

func WithComponentContext(ctx context.Context, componentName string) context.Context {
	return context.WithValue(ctx, ContextKeyComponentName, componentName)
}

func WithSessionContext(ctx context.Context, sessionId string) context.Context {
	return context.WithValue(ctx, ContextKeySessionId, sessionId)
}

func WithQueryIdContext(ctx context.Context, queryId string) context.Context {
	return context.WithValue(ctx, ContextKeyQueryId, queryId)
}

func GetContextSessionId(ctx context.Context) string {
	if sessionId, ok := ctx.Value(ContextKeySessionId).(string); ok {
		return sessionId
	}
	return ""
}

func GetContextQueryId(ctx context.Context) string {
	if queryId, ok := ctx.Value(ContextKeyQueryId).(string); ok {
		return queryId
	}
	return ""
}

func GetContextTraceId(ctx context.Context) string {
	if traceId, ok := ctx.Value(ContextKeyTraceId).(string); ok {
		return traceId
	}

	return ""
}

func GetContextComponentName(ctx context.Context) string {
	if componentName, ok := ctx.Value(ContextKeyComponentName).(string); ok {
		return componentName
	}

	return "Wox"
}

package util

import (
	"context"
	"fmt"
	"runtime/debug"
)

func Go(ctx context.Context, message string, jobTodo func(), recoverFunc ...func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				msg := fmt.Sprintf("%s panic，err: %s, stack: %s", message, err, debug.Stack())
				GetLogger().Error(ctx, msg)
				if len(recoverFunc) > 0 {
					recoverFunc[0]()
				}
			}
		}()
		jobTodo()
	}()
}

func GoRecover(ctx context.Context, message string) {
	if err := recover(); err != nil {
		msg := fmt.Sprintf("%s panic，err: %s, stack: %s", message, err, debug.Stack())
		GetLogger().Debug(ctx, msg)
	}
}

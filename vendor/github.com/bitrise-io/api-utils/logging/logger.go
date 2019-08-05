package logging

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type contextKey string

const loggerKey contextKey = "ctx-logger"

var logger *zap.Logger

func init() {
	newLogger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %s", err)
	}
	logger = newLogger
}

// NewContext ...
func NewContext(ctx context.Context, fields ...zap.Field) context.Context {
	return context.WithValue(ctx, loggerKey, WithContext(ctx).With(fields...))
}

// WithContext ...
func WithContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return logger
	}
	if ctxLogger, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return ctxLogger
	}
	return logger
}

// Sync ...
func Sync(logger *zap.Logger) {
	err := logger.Sync()
	if err != nil {
		if !strings.Contains(err.Error(), "invalid argument") {
			fmt.Printf("Failed to sync logger: %s", errors.WithStack(err))
		}
	}
}

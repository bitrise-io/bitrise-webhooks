package logging

import (
	"fmt"

	"github.com/blendle/zapdriver"
	"go.uber.org/zap"
)

var stackDriverLogger *zap.Logger

func init() {
	var err error

	config := zapdriver.NewProductionConfig()
	// remove plain caller from the output
	config.EncoderConfig.CallerKey = ""
	logger, err = config.Build(zapdriver.WrapCore())

	if err != nil {
		fmt.Printf("Failed to initialize stackdriver logger: %s", err)
	}

	stackDriverLogger = logger
}

// StackDriverLogger Builds new StackDriver logger
func StackDriverLogger(fields ...zap.Field) *zap.Logger {
	return stackDriverLogger.With(fields...)
}

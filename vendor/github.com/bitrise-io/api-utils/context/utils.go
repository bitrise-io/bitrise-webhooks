package context

import (
	"context"
	"errors"

	"github.com/bitrise-io/api-utils/providers"
)

// RequestContextKey ...
type RequestContextKey string

const (
	// --- Providers

	// ContextKeyRequestParamProvider ...
	ContextKeyRequestParamProvider RequestContextKey = "rck-request-param-provider"
)

// RequestParamProviderFromContext ...
func RequestParamProviderFromContext(ctx context.Context) (providers.RequestParamsInterface, error) {
	requestParamProvider, ok := ctx.Value(ContextKeyRequestParamProvider).(providers.RequestParamsInterface)
	if !ok {
		return requestParamProvider, errors.New("Request params provider not found in Context")
	}
	return requestParamProvider, nil
}

// WithRequestParamProvider ...
func WithRequestParamProvider(ctx context.Context, requestParamProvider providers.RequestParamsInterface) context.Context {
	return context.WithValue(ctx, ContextKeyRequestParamProvider, requestParamProvider)
}

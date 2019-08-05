package middleware

import (
	"net/http"

	"github.com/bitrise-io/api-utils/context"
	"github.com/bitrise-io/api-utils/httpresponse"
	"github.com/bitrise-io/api-utils/logging"
	"github.com/bitrise-io/api-utils/providers"
	"github.com/justinas/alice"
	"github.com/rs/cors"
	"go.uber.org/zap"
)

// CommonMiddleware ...
func CommonMiddleware() alice.Chain {
	return alice.New(
		cors.AllowAll().Handler,
	)
}

// CreateRedirectToHTTPSMiddleware ...
func CreateRedirectToHTTPSMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme := r.Header.Get("X-Forwarded-Proto")
			if scheme != "" && scheme != "https" {
				target := "https://" + r.Host + r.URL.Path
				if len(r.URL.RawQuery) > 0 {
					target += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, target, http.StatusPermanentRedirect)
				return
			}

			h.ServeHTTP(w, r)
		})
	}
}

// CreateOptionsRequestTerminatorMiddleware ...
func CreateOptionsRequestTerminatorMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "OPTIONS" {
				httpresponse.RespondWithJSONNoErr(w, 200, nil)
			} else {
				h.ServeHTTP(w, r)
			}
		})
	}
}

// CreateSetRequestParamProviderMiddleware ...
func CreateSetRequestParamProviderMiddleware(requestParamProvider providers.RequestParamsInterface) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithRequestParamProvider(r.Context(), requestParamProvider)
			h.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AddLoggerToContextMiddleware ...
func AddLoggerToContextMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := logging.NewContext(r.Context(), zap.String("request_id", r.Header.Get("X-Request-ID")))
			h.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

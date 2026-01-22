package telemetry

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
	written    bool
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{ResponseWriter: w, StatusCode: http.StatusOK}
}

func (r *ResponseRecorder) WriteHeader(code int) {
	if !r.written {
		r.StatusCode = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

func LoggingMiddleware(cfg *LoggingConfig) func(http.Handler) http.Handler {
	parsedCfg := parseLoggingConfig(cfg)
	excludeSet := parsedCfg.excludePaths

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if excludeSet[path] {
				next.ServeHTTP(w, r)
				return
			}

			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = r.Header.Get("X-Correlation-ID")
			}

			var logger *RequestLogger
			if requestID != "" {
				logger = NewRequestLoggerWithID("api", path, requestID, cfg)
			} else {
				logger = NewRequestLogger("api", path, cfg)
			}

			logger.AddFields(
				zap.String("method", r.Method),
				zap.String("user_agent", r.UserAgent()),
				zap.String("remote_addr", r.RemoteAddr),
			)

			w.Header().Set("X-Request-ID", logger.RequestID())

			ctx := ContextWithLogger(r.Context(), logger)
			recorder := NewResponseRecorder(w)

			defer func() {
				if rec := recover(); rec != nil {
					logger.AddFields(zap.Any("panic", rec))
					logger.Finalize(Outcome{
						Level:      zap.ErrorLevel,
						StatusCode: http.StatusInternalServerError,
						Message:    "request panicked",
					})
					panic(rec)
				}
			}()

			next.ServeHTTP(recorder, r.WithContext(ctx))

			if outcome := OutcomeFromContext(ctx); outcome != nil {
				logger.Finalize(*outcome)
			} else {
				logger.Finalize(Outcome{
					Level:      LevelFromStatusCode(recorder.StatusCode),
					StatusCode: recorder.StatusCode,
					Message:    "request completed",
				})
			}
		})
	}
}

type outcomeKeyType struct{}

var outcomeKey = outcomeKeyType{}

func SetOutcome(ctx context.Context, outcome Outcome) context.Context {
	return context.WithValue(ctx, outcomeKey, &outcome)
}

func OutcomeFromContext(ctx context.Context) *Outcome {
	if outcome, ok := ctx.Value(outcomeKey).(*Outcome); ok {
		return outcome
	}
	return nil
}

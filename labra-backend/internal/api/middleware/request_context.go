package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// using a typed string so we don't accidentally clash with other context keys
type contextKey string

const (
	requestIDContextKey     contextKey = "request_id"
	correlationIDContextKey contextKey = "correlation_id"
)

// RequestIDFromContext pulls the request id out of context - returns empty string if not set
func RequestIDFromContext(ctx context.Context) string {
	requestIDValue, _ := ctx.Value(requestIDContextKey).(string)
	return requestIDValue
}

// CorrelationIDFromContext pulls the correlation id out of context
func CorrelationIDFromContext(ctx context.Context) string {
	correlationIDValue, _ := ctx.Value(correlationIDContextKey).(string)
	return correlationIDValue
}

// RequestContext is a middleware that assigns request and correlation ids to each request
// also logs every request with method, path, status, and duration
func RequestContext(logger *slog.Logger) func(http.Handler) http.Handler {
	// use the default logger if none provided
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestStartTime := time.Now()

			// use incoming request id if provided, otherwise generate one
			incomingRequestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
			if incomingRequestID == "" {
				incomingRequestID = randomID()
			}

			// correlation id follows a request through multiple services
			incomingCorrelationID := strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
			if incomingCorrelationID == "" {
				// if no correlation id, use the request id as a fallback
				incomingCorrelationID = incomingRequestID
			}

			// stash both ids in context so handlers can use them
			requestCtx := context.WithValue(r.Context(), requestIDContextKey, incomingRequestID)
			requestCtx = context.WithValue(requestCtx, correlationIDContextKey, incomingCorrelationID)
			r = r.WithContext(requestCtx)

			// echo them back in response headers so clients can correlate
			w.Header().Set("X-Request-ID", incomingRequestID)
			w.Header().Set("X-Correlation-ID", incomingCorrelationID)

			// wrap the response writer to capture the status code for logging
			statusCapture := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(statusCapture, r)

			// log the completed request with all the useful info
			logger.LogAttrs(r.Context(), slog.LevelInfo,
				"request completed",
				slog.String("request_id", incomingRequestID),
				slog.String("correlation_id", incomingCorrelationID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", statusCapture.statusCode),
				slog.Int64("duration_ms", time.Since(requestStartTime).Milliseconds()),
			)
		})
	}
}

// statusRecorder wraps a ResponseWriter and captures the status code that was written
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (s *statusRecorder) WriteHeader(statusCode int) {
	s.statusCode = statusCode
	s.ResponseWriter.WriteHeader(statusCode)
}

// randomID generates a random hex string for use as request ids
func randomID() string {
	randomBytes := make([]byte, 12)
	if _, err := rand.Read(randomBytes); err != nil {
		// fall back to a static string if random fails - better than crashing
		return "generated-request-id"
	}
	return hex.EncodeToString(randomBytes)
}

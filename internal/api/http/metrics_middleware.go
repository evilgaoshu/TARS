package httpapi

import (
	"net/http"
	"strings"
	"time"

	"tars/internal/foundation/observability"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func instrumentHandler(deps Dependencies, route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Metrics == nil {
			next(w, r)
			return
		}

		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}
		next(recorder, r)
		deps.Metrics.IncHTTPRequest(route, r.Method, recorder.status)
		if deps.Observability != nil && !strings.HasPrefix(route, "/metrics") {
			_ = deps.Observability.AppendEvent(observability.SignalRecord{
				Timestamp: timeNowUTC(),
				Component: "http_api",
				Message:   "http request completed",
				Route:     route,
				Metadata: map[string]any{
					"method": r.Method,
					"code":   recorder.status,
				},
			})
		}
	}
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

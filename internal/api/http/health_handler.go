package httpapi

import (
	"bytes"
	"net/http"

	"tars/internal/api/dto"
)

func healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		writeJSON(w, http.StatusOK, dto.HealthResponse{Status: "ok"})
	}
}

func readyz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		writeJSON(w, http.StatusOK, dto.ReadyResponse{Status: "ready", Degraded: false})
	}
}

func metricsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if deps.Metrics == nil {
			_, _ = w.Write([]byte("# metrics unavailable\n"))
			return
		}

		var buf bytes.Buffer
		if err := deps.Metrics.WritePrometheus(&buf); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		_, _ = w.Write(buf.Bytes())
	}
}

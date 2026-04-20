package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func webUIHandlerWithDir(distDir string) http.HandlerFunc {
	distDir = strings.TrimSpace(distDir)

	fileServer := http.FileServer(http.Dir(distDir))
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			http.NotFound(w, r)
			return
		}
		if distDir == "" {
			writeError(w, http.StatusServiceUnavailable, "web_ui_unavailable", "web ui assets are not configured")
			return
		}

		cleanPath := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if cleanPath == "." {
			cleanPath = ""
		}

		if cleanPath == "" || !strings.Contains(path.Base(cleanPath), ".") {
			indexPath := filepath.Join(distDir, "index.html")
			if _, err := os.Stat(indexPath); err != nil {
				writeError(w, http.StatusServiceUnavailable, "web_ui_unavailable", "web ui index is not available")
				return
			}
			http.ServeFile(w, r, indexPath)
			return
		}

		assetPath := filepath.Join(distDir, filepath.FromSlash(cleanPath))
		if _, err := os.Stat(assetPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

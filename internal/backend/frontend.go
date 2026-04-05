package backend

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	webui "github.com/fvrv17/mvp/web"
)

func (a *App) mountFrontend(r chi.Router) {
	r.Get("/", a.serveFrontendAsset("index.html"))
	r.Get("/styles.css", a.serveFrontendAsset("styles.css"))
	r.Get("/app.js", a.serveFrontendAsset("app.js"))
}

func (a *App) serveFrontendAsset(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if name == "index.html" {
			if redirectURL := strings.TrimSpace(os.Getenv("FRONTEND_REDIRECT_URL")); redirectURL != "" {
				http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
				return
			}
		}
		payload, err := webui.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if name == "index.html" {
			w.Header().Set("Cache-Control", "no-store")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=300")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}
}

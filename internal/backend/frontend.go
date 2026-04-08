package backend

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *App) mountFrontend(r chi.Router) {
	r.Get("/", a.handleFrontendRoot)
}

func (a *App) handleFrontendRoot(w http.ResponseWriter, r *http.Request) {
	if redirectURL := strings.TrimSpace(os.Getenv("FRONTEND_REDIRECT_URL")); redirectURL != "" {
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>SkillRoom Backend</title>
  </head>
  <body>
    <main>
      <h1>SkillRoom backend is running</h1>
      <p>The active browser client is the Next.js frontend served separately.</p>
      <p>Set FRONTEND_REDIRECT_URL to redirect this root route to the frontend.</p>
    </main>
  </body>
</html>`))
}

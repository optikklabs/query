package app

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/httputil"
)

// health answers liveness and readiness probes. Dependencies are dialled
// during startup, so a serving process is ready by construction.
func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

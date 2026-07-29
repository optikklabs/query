package app

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/httputil"
)

func (a *App) healthLive(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) healthReady(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

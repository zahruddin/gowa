package api

import (
	"net/http"
	"gowa/internal/database"
)

func (h *APIHandler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-API-Token")
		if token == "" {
			http.Error(w, "Unauthorized: Missing API Token", http.StatusUnauthorized)
			return
		}

		var exists bool
		err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM sessions WHERE api_token = ?)", token).Scan(&exists)
		if err != nil || !exists {
			http.Error(w, "Unauthorized: Invalid API Token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

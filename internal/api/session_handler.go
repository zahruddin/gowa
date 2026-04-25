package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"gowa/internal/database"
)

// --- SESSION MANAGEMENT ---

func (h *APIHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, COALESCE(api_token,''), COALESCE(webhook_url,''), COALESCE(status,'created'), COALESCE(created_at,'') FROM sessions ORDER BY created_at DESC")
	if err != nil {
		jsonError(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type sessionInfo struct {
		ID          string `json:"id"`
		APIToken    string `json:"api_token"`
		WebhookURL  string `json:"webhook_url"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		IsConnected bool   `json:"is_connected"`
		DeviceJID   string `json:"device_jid"`
		PushName    string `json:"push_name"`
	}

	var sessions []sessionInfo
	for rows.Next() {
		var s sessionInfo
		err := rows.Scan(&s.ID, &s.APIToken, &s.WebhookURL, &s.Status, &s.CreatedAt)
		if err != nil {
			continue
		}
		s.IsConnected = h.BotManager.IsConnected(s.ID)
		s.DeviceJID = h.BotManager.GetDeviceJID(s.ID)
		s.PushName = h.BotManager.GetPushName(s.ID)

		if s.IsConnected && s.Status != "connected" {
			s.Status = "connected"
			database.DB.Exec("UPDATE sessions SET status = 'connected' WHERE id = ?", s.ID)
		} else if !s.IsConnected && s.Status == "connected" {
			s.Status = "disconnected"
			database.DB.Exec("UPDATE sessions SET status = 'disconnected' WHERE id = ?", s.ID)
		}

		sessions = append(sessions, s)
	}

	if sessions == nil {
		sessions = []sessionInfo{}
	}
	jsonResponse(w, sessions)
}

func (h *APIHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		jsonError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Generate unique API token
	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	apiToken := "gowa_" + hex.EncodeToString(tokenBytes)

	// Insert into database
	_, err := database.DB.Exec(
		"INSERT OR IGNORE INTO sessions (id, api_token, status) VALUES (?, ?, 'created')",
		req.ID, apiToken,
	)
	if err != nil {
		jsonError(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Start the WhatsApp client in background (will generate QR)
	go h.BotManager.InitClient(req.ID)

	jsonResponse(w, map[string]string{
		"status":    "success",
		"id":        req.ID,
		"api_token": apiToken,
		"message":   "Session created. Poll /api/sessions/" + req.ID + "/qr for QR code.",
	})
}

func (h *APIHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.ID == "" {
		jsonError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Logout & remove from memory
	h.BotManager.Logout(req.ID)

	// Remove from database
	database.DB.Exec("DELETE FROM sessions WHERE id = ?", req.ID)

	jsonResponse(w, map[string]string{"status": "success", "message": "Session deleted"})
}

func (h *APIHandler) ConnectSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.ID == "" {
		jsonError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Check if client already exists
	_, exists := h.BotManager.GetClient(req.ID)
	if exists {
		// Try to connect
		err := h.BotManager.ConnectClient(req.ID)
		if err != nil {
			jsonError(w, "Failed to connect: "+err.Error(), http.StatusInternalServerError)
			return
		}
		database.DB.Exec("UPDATE sessions SET status = 'connected' WHERE id = ?", req.ID)
	} else {
		// Re-init from scratch
		go h.BotManager.InitClient(req.ID)
	}

	jsonResponse(w, map[string]string{"status": "success", "message": "Connecting..."})
}

func (h *APIHandler) DisconnectSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.ID == "" {
		jsonError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	h.BotManager.DisconnectClient(req.ID)
	database.DB.Exec("UPDATE sessions SET status = 'disconnected' WHERE id = ?", req.ID)

	jsonResponse(w, map[string]string{"status": "success", "message": "Session disconnected"})
}

func (h *APIHandler) GetSessionQR(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from query
	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		jsonError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	qr, ok := h.BotManager.GetQR(sessionID)
	if !ok || qr == "" {
		jsonResponse(w, map[string]interface{}{
			"has_qr": false,
			"qr":     "",
		})
		return
	}

	jsonResponse(w, map[string]interface{}{
		"has_qr": true,
		"qr":     qr,
	})
}

func (h *APIHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID         string `json:"id"`
		WebhookURL string `json:"webhook_url"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	database.DB.Exec("UPDATE sessions SET webhook_url = ? WHERE id = ?", req.WebhookURL, req.ID)
	jsonResponse(w, map[string]string{"status": "success"})
}

// --- KATALOG ---

func (h *APIHandler) ListKatalog(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		jsonError(w, "session_id is required", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query("SELECT keyword, details FROM katalog WHERE session_id = ? ORDER BY keyword ASC", sessionID)
	if err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type katalogItem struct {
		Keyword string `json:"keyword"`
		Details string `json:"details"`
	}

	var items []katalogItem
	for rows.Next() {
		var item katalogItem
		rows.Scan(&item.Keyword, &item.Details)
		items = append(items, item)
	}
	if items == nil {
		items = []katalogItem{}
	}
	jsonResponse(w, items)
}

func (h *APIHandler) SaveKatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Keyword   string `json:"keyword"`
		Details   string `json:"details"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	req.Keyword = strings.ToLower(strings.TrimSpace(req.Keyword))
	if req.Keyword == "" || req.SessionID == "" {
		jsonError(w, "session_id and keyword are required", http.StatusBadRequest)
		return
	}

	database.DB.Exec("REPLACE INTO katalog (session_id, keyword, details) VALUES (?, ?, ?)", req.SessionID, req.Keyword, req.Details)
	jsonResponse(w, map[string]string{"status": "success"})
}

func (h *APIHandler) DeleteKatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Keyword   string `json:"keyword"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	database.DB.Exec("DELETE FROM katalog WHERE session_id = ? AND keyword = ?", req.SessionID, req.Keyword)
	jsonResponse(w, map[string]string{"status": "success"})
}

// --- WHITELIST ---

func (h *APIHandler) ListWhitelist(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		jsonError(w, "session_id is required", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query("SELECT number, COALESCE(name,'') FROM whitelist WHERE session_id = ? ORDER BY name ASC", sessionID)
	if err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type whitelistItem struct {
		Number string `json:"number"`
		Name   string `json:"name"`
	}

	var items []whitelistItem
	for rows.Next() {
		var item whitelistItem
		rows.Scan(&item.Number, &item.Name)
		items = append(items, item)
	}
	if items == nil {
		items = []whitelistItem{}
	}
	jsonResponse(w, items)
}

func (h *APIHandler) SaveWhitelist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Number    string `json:"number"`
		Name      string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	req.Number = strings.TrimSpace(req.Number)
	if req.Number == "" || req.SessionID == "" {
		jsonError(w, "session_id and number are required", http.StatusBadRequest)
		return
	}

	database.DB.Exec("REPLACE INTO whitelist (session_id, number, name) VALUES (?, ?, ?)", req.SessionID, req.Number, req.Name)
	jsonResponse(w, map[string]string{"status": "success"})
}

func (h *APIHandler) DeleteWhitelist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Number    string `json:"number"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	database.DB.Exec("DELETE FROM whitelist WHERE session_id = ? AND number = ?", req.SessionID, req.Number)
	jsonResponse(w, map[string]string{"status": "success"})
}

// --- HELPERS ---

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

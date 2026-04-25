package api

import (
	"encoding/json"
	"net/http"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

func (h *APIHandler) SendChatMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		To        string `json:"to"`
		Message   string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, ok := h.BotManager.GetClient(req.SessionID)
	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	targetJID := types.NewJID(req.To, types.DefaultUserServer)
	
	_, err := client.SendMessage(r.Context(), targetJID, &waE2E.Message{
		Conversation: &req.Message,
	})
	if err != nil {
		http.Error(w, "Failed to send message: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Message sent"})
}
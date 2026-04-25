package api

import (
	"gowa/internal/bot"
)

type APIHandler struct {
	BotManager *bot.Manager
}

func NewAPIHandler(bm *bot.Manager) *APIHandler {
	return &APIHandler{BotManager: bm}
}

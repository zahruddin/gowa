package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gowa/internal/database"
)

func (m *Manager) GetHandler(sessionID string) func(interface{}) {
	return func(evt interface{}) {
		client, ok := m.GetClient(sessionID)
		if !ok {
			return
		}

		switch v := evt.(type) {
		case *events.Message:
			if v.Info.IsFromMe {
				return
			}

			msgText := v.Message.GetConversation()
			if msgText == "" && v.Message.GetExtendedTextMessage() != nil {
				msgText = v.Message.GetExtendedTextMessage().GetText()
			}

			rawMsg := strings.TrimSpace(msgText)
			if rawMsg == "" {
				return
			}
			lowerMsg := strings.ToLower(rawMsg)

			// 1. Check Whitelist for Admin Commands
			// Real phone number is in SenderAlt (e.g. 62895631337014@s.whatsapp.net)
			// Sender field only has LID (Linked Identity)
			senderNumber := v.Info.Sender.User // fallback: LID
			if v.Info.MessageSource.SenderAlt.User != "" {
				senderNumber = v.Info.MessageSource.SenderAlt.User // real phone number
			}

			isAdmin := m.IsAdmin(sessionID, senderNumber)
			// fmt.Printf("[%s] From: %s (LID: %s) | Admin: %v | %s\n",
			// 	sessionID, senderNumber, v.Info.Sender.User, isAdmin, rawMsg)

			if isAdmin {
				if strings.HasPrefix(lowerMsg, "#setwebhook ") {
					webhook := strings.TrimSpace(rawMsg[12:])
					database.DB.Exec("UPDATE sessions SET webhook_url = ? WHERE id = ?", webhook, sessionID)
					m.SendMessage(client, v.Info.Chat, "✅ Webhook updated.")
					return
				}
				if strings.HasPrefix(lowerMsg, "#addkatalog ") {
					parts := strings.SplitN(rawMsg[12:], "@", 2)
					if len(parts) == 2 {
						keyword := strings.ToLower(strings.TrimSpace(parts[0]))
						detail := strings.TrimSpace(parts[1])
						_, dbErr := database.DB.Exec("REPLACE INTO katalog (session_id, keyword, details) VALUES (?, ?, ?)",
							sessionID, keyword, detail)
						if dbErr != nil {
							fmt.Printf("[%s] DB Error saving katalog: %v\n", sessionID, dbErr)
							m.SendMessage(client, v.Info.Chat, "❌ Error: "+dbErr.Error())
						} else {
							fmt.Printf("[%s] Katalog saved: %s -> %s\n", sessionID, keyword, detail)
							m.SendMessage(client, v.Info.Chat, "✅ Katalog '"+keyword+"' updated.")
						}
					} else {
						m.SendMessage(client, v.Info.Chat, "⚠️ Format: #addkatalog keyword@pesan")
					}
					return
				}
				if lowerMsg == "#status" {
					m.SendMessage(client, v.Info.Chat, "🤖 System: Online\nSession: "+sessionID)
					return
				}
			}

			// --- Public commands (no whitelist needed) ---
			if lowerMsg == "#myid" {
				phoneNum := v.Info.MessageSource.SenderAlt.User
				if phoneNum == "" {
					phoneNum = v.Info.Sender.User
				}
				info := fmt.Sprintf("📋 *Your Info*\n\n📱 Phone: %s\n🔗 LID: %s\n\n💡 Add this to whitelist:\n*%s*",
					phoneNum, v.Info.Sender.User, phoneNum)
				m.SendMessage(client, v.Info.Chat, info)
				return
			}

			// 2. Katalog / Auto-Response
			var details string
			err := database.DB.QueryRow("SELECT details FROM katalog WHERE session_id = ? AND keyword = ?", sessionID, lowerMsg).Scan(&details)
			if err == nil {
				m.SendMessage(client, v.Info.Chat, details)
				return
			}

			// 3. List command
			if lowerMsg == "list" {
				m.SendList(client, sessionID, v.Info.Chat)
				return
			}

			// 4. Webhook Forwarding
			go m.ForwardToWebhook(sessionID, v)
		}
	}
}

// PERBAIKAN: Tambahkan parameter sessionID
func (m *Manager) IsAdmin(sessionID string, number string) bool {
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM whitelist WHERE session_id = ? AND number = ?)", sessionID, number).Scan(&exists)
	return err == nil && exists
}

func (m *Manager) SendMessage(client *whatsmeow.Client, target types.JID, text string) {
	if client == nil {
		return
	}
	client.SendMessage(context.Background(), target, &waE2E.Message{Conversation: proto.String(text)})
}

// PERBAIKAN: Tambahkan parameter sessionID
func (m *Manager) SendList(client *whatsmeow.Client, sessionID string, target types.JID) {
	if client == nil {
		return
	}
	// PERBAIKAN: Ambil katalog khusus untuk session ini saja
	rows, _ := database.DB.Query("SELECT keyword FROM katalog WHERE session_id = ? ORDER BY keyword ASC", sessionID)
	defer rows.Close()
	
	var listBuilder strings.Builder
	listBuilder.WriteString("🏪 *KATALOG KAMI*\n\n")
	for rows.Next() {
		var kw string
		rows.Scan(&kw)
		listBuilder.WriteString(fmt.Sprintf("▪️ %s\n", kw))
	}
	
	items := listBuilder.String()
	if items == "🏪 *KATALOG KAMI*\n\n" {
		items += "(Kosong)"
	}
	
	m.SendMessage(client, target, items)
}

func (m *Manager) ForwardToWebhook(sessionID string, v *events.Message) {
	var webhookURL string
	err := database.DB.QueryRow("SELECT webhook_url FROM sessions WHERE id = ?", sessionID).Scan(&webhookURL)
	if err != nil || webhookURL == "" {
		return
	}

	payload := map[string]interface{}{
		"session_id": sessionID,
		"sender":     v.Info.Sender.User,
		"message":    v.Message.GetConversation(),
		"timestamp":  v.Info.Timestamp,
	}

	data, _ := json.Marshal(payload)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
	if err == nil {
		resp.Body.Close()
	}
}
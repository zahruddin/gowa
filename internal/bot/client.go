package bot

import (
	"context"
	"fmt"

	"gowa/internal/database"
)

// InitClient connects a session. If not yet paired, starts QR flow and stores
// the QR string so the web panel can poll it via /api/sessions/qr?id=xxx.
func (m *Manager) InitClient(id string) error {
	client, err := m.AddClient(id)
	if err != nil {
		return err
	}

	if client.Store.ID == nil {
		// New device — need QR pairing
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			return err
		}

		database.DB.Exec("UPDATE sessions SET status = 'waiting_qr' WHERE id = ?", id)
		fmt.Printf("[%s] Waiting for QR scan (check web panel)...\n", id)

		for evt := range qrChan {
			if evt.Event == "code" {
				m.SetQR(id, evt.Code)
				fmt.Printf("[%s] QR code ready — scan from web panel\n", id)
			} else if evt.Event == "success" {
				m.ClearQR(id)
				// Save the device JID so we can find this device on next restart
				if client.Store.ID != nil {
					database.DB.Exec("UPDATE sessions SET device_jid = ?, status = 'connected' WHERE id = ?",
						client.Store.ID.String(), id)
				} else {
					database.DB.Exec("UPDATE sessions SET status = 'connected' WHERE id = ?", id)
				}
				fmt.Printf("[%s] Connected!\n", id)
			} else if evt.Event == "timeout" {
				m.ClearQR(id)
				database.DB.Exec("UPDATE sessions SET status = 'timeout' WHERE id = ?", id)
				fmt.Printf("[%s] QR timeout\n", id)
			}
		}
	} else {
		// Already paired — just connect
		err = client.Connect()
		if err != nil {
			database.DB.Exec("UPDATE sessions SET status = 'error' WHERE id = ?", id)
			return err
		}
		// Update JID in case it wasn't saved before
		database.DB.Exec("UPDATE sessions SET device_jid = ?, status = 'connected' WHERE id = ?",
			client.Store.ID.String(), id)
		fmt.Printf("[%s] Reconnected (already paired)\n", id)
	}

	return nil
}

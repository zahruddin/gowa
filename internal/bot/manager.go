package bot

import (
	"context"
	"fmt"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"gowa/internal/database"
)

type Manager struct {
	Clients        map[string]*whatsmeow.Client
	StoreContainer *sqlstore.Container
	QRChannels     map[string]string // sessionID -> latest QR code string
	mu             sync.RWMutex
}

func NewManager(dbPath string) (*Manager, error) {
	container, err := sqlstore.New(context.Background(), "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath), waLog.Stdout("Database", "WARN", true))
	if err != nil {
		return nil, err
	}

	return &Manager{
		Clients:        make(map[string]*whatsmeow.Client),
		StoreContainer: container,
		QRChannels:     make(map[string]string),
	}, nil
}

// AddClient creates or retrieves an existing whatsmeow client for the given session ID.
// It checks for an existing device in the whatsmeow store (via the JID saved in our DB)
// so that sessions survive app restarts without needing to re-scan QR.
func (m *Manager) AddClient(id string) (*whatsmeow.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Already loaded in memory
	if client, ok := m.Clients[id]; ok {
		return client, nil
	}

	// Try to find an existing device from the whatsmeow store
	// by looking up the JID we saved in our sessions table
	var device *store.Device

	var jidStr string
	err := database.DB.QueryRow("SELECT COALESCE(device_jid,'') FROM sessions WHERE id = ?", id).Scan(&jidStr)
	if err == nil && jidStr != "" {
		// We have a saved JID — try to find that device in the whatsmeow store
		jid, parseErr := types.ParseJID(jidStr)
		if parseErr == nil {
			device, _ = m.StoreContainer.GetDevice(context.Background(), jid)
		}
	}

	// If no saved device found, also scan all devices in the store
	// (handles first-run migration or if device_jid column was empty)
	if device == nil {
		allDevices, _ := m.StoreContainer.GetAllDevices(context.Background())
		if len(allDevices) > 0 {
			// Check if any device is unclaimed by another session
			for _, d := range allDevices {
				if d.ID != nil {
					// Check if this device is already used by another session in our map
					alreadyUsed := false
					for _, existingClient := range m.Clients {
						if existingClient.Store.ID != nil && existingClient.Store.ID.String() == d.ID.String() {
							alreadyUsed = true
							break
						}
					}
					if !alreadyUsed {
						device = d
						// Save the JID mapping for next restart
						database.DB.Exec("UPDATE sessions SET device_jid = ? WHERE id = ?", d.ID.String(), id)
						break
					}
				}
			}
		}
	}

	// Still no device? Create a brand new one (will need QR pairing)
	if device == nil {
		device = m.StoreContainer.NewDevice()
	}

	client := whatsmeow.NewClient(device, waLog.Stdout("Client-"+id, "WARN", true))
	m.Clients[id] = client

	client.AddEventHandler(m.GetHandler(id))

	return client, nil
}

func (m *Manager) GetClient(id string) (*whatsmeow.Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.Clients[id]
	return client, ok
}

func (m *Manager) RemoveClient(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if client, ok := m.Clients[id]; ok {
		client.Disconnect()
		delete(m.Clients, id)
	}
	delete(m.QRChannels, id)
}

// SetQR stores the latest QR code string for a session so the web UI can poll it.
func (m *Manager) SetQR(id string, qr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.QRChannels[id] = qr
}

// GetQR retrieves the latest QR code string for a session.
func (m *Manager) GetQR(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	qr, ok := m.QRChannels[id]
	return qr, ok
}

// ClearQR removes the QR code for a session (after successful pairing).
func (m *Manager) ClearQR(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.QRChannels, id)
}

// IsConnected checks if a client exists and is connected.
func (m *Manager) IsConnected(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.Clients[id]
	if !ok {
		return false
	}
	return client.IsConnected()
}

// GetAllClientIDs returns all active session IDs.
func (m *Manager) GetAllClientIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.Clients))
	for id := range m.Clients {
		ids = append(ids, id)
	}
	return ids
}

// GetDeviceJID returns the JID string of a connected device, or empty string.
func (m *Manager) GetDeviceJID(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.Clients[id]
	if !ok || client.Store.ID == nil {
		return ""
	}
	return client.Store.ID.User
}

// GetPushName returns the push name (display name) of a connected device.
func (m *Manager) GetPushName(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.Clients[id]
	if !ok || client.Store.ID == nil {
		return ""
	}
	return client.Store.PushName
}

// DisconnectClient disconnects a specific session without removing it.
func (m *Manager) DisconnectClient(id string) {
	m.mu.RLock()
	client, ok := m.Clients[id]
	m.mu.RUnlock()
	if ok {
		client.Disconnect()
	}
}

// ConnectClient connects a previously disconnected session.
func (m *Manager) ConnectClient(id string) error {
	client, ok := m.GetClient(id)
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	return client.Connect()
}

// Logout logs out a session and removes the device store.
func (m *Manager) Logout(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	client, ok := m.Clients[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	err := client.Logout(context.Background())
	client.Disconnect()
	delete(m.Clients, id)
	delete(m.QRChannels, id)
	return err
}

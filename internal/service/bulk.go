package service

import (
	"context"
	// "encoding/json"
	"fmt"
	// "log"
	// "math/rand"
	// "strings"
	"time"

	"gowa/internal/bot"
	"gowa/internal/database"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type BulkService struct {
	BotManager *bot.Manager
}

func NewBulkService(bm *bot.Manager) *BulkService {
	return &BulkService{BotManager: bm}
}

// StartWorker menjalankan background process untuk mengirim pesan bulk
func (s *BulkService) StartWorker() {
	go func() {
		for {
			s.processNextJob()
			time.Sleep(5 * time.Second)
		}
	}()
}

func (s *BulkService) processNextJob() {
	var jobID int
	var sessionID, status string
	err := database.DB.QueryRow("SELECT id, session_id, status FROM bulk_jobs WHERE status IN ('pending', 'processing') ORDER BY id ASC LIMIT 1").Scan(&jobID, &sessionID, &status)
	if err != nil {
		return
	}

	// Update status ke processing
	if status == "pending" {
		database.DB.Exec("UPDATE bulk_jobs SET status = 'processing' WHERE id = ?", jobID)
	}

	client, ok := s.BotManager.GetClient(sessionID)
	if !ok || !client.IsConnected() {
		return
	}

	// Get total for progress
	var total int
	database.DB.QueryRow("SELECT total FROM bulk_jobs WHERE id = ?", jobID).Scan(&total)

	// Since we don't have a 'bulk_items' table yet, for now we assume 
	// the data is passed once. In a real scenario, we'd loop through items.
	// Let's refine the database to include bulk_items if needed, 
	// but for this MVP, we'll mark it as completed after one run.
	
	// database.DB.Exec("UPDATE bulk_jobs SET status = 'completed' WHERE id = ?", jobID)
}

func (s *BulkService) CreateJob(sessionID string, total int) (int64, error) {
	res, err := database.DB.Exec("INSERT INTO bulk_jobs (session_id, total, success, failed, status) VALUES (?, ?, 0, 0, 'pending')", sessionID, total)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *BulkService) SendMessage(sessionID string, to string, message string) error {
	client, ok := s.BotManager.GetClient(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	targetJID := types.NewJID(to, types.DefaultUserServer)
	_, err := client.SendMessage(context.Background(), targetJID, &waE2E.Message{
		Conversation: proto.String(message),
	})
	return err
}


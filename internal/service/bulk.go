package service

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
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

func (s *BulkService) CreateJob(sessionID string, total int, minDelay int, maxDelay int) (int64, error) {
	res, err := database.DB.Exec("INSERT INTO bulk_jobs (session_id, total, success, failed, status, min_delay, max_delay) VALUES (?, ?, 0, 0, 'pending', ?, ?)", sessionID, total, minDelay, maxDelay)
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

	to = strings.TrimSpace(to)
	to = strings.TrimPrefix(to, "+")
	if !strings.Contains(to, "@s.whatsapp.net") {
		to = to + "@s.whatsapp.net"
	}

	targetJID := types.NewJID(strings.Split(to, "@")[0], types.DefaultUserServer)
	
	_, err := client.SendMessage(context.Background(), targetJID, &waE2E.Message{
		Conversation: proto.String(message),
	})
	return err
}

func (s *BulkService) processSpintext(text string) string {
	re := regexp.MustCompile(`\{([^{}]*)\}`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		inner := match[1 : len(match)-1]
		parts := strings.Split(inner, "|")
		if len(parts) == 0 {
			return ""
		}
		return parts[rand.Intn(len(parts))]
	})
}

// ProcessJob sekarang menggunakan metode Queue Locking (Anti-Double)
func (s *BulkService) ProcessJob(jobID int64) {
	// 1. Kunci status job utama
	_, _ = database.DB.Exec("UPDATE bulk_jobs SET status = 'running' WHERE id = ? AND status = 'pending'", jobID)

	var sessionID string
	var minDelay, maxDelay int
	err := database.DB.QueryRow("SELECT session_id, min_delay, max_delay FROM bulk_jobs WHERE id = ?", jobID).Scan(&sessionID, &minDelay, &maxDelay)
	if err != nil {
		return
	}

	client, ok := s.BotManager.GetClient(sessionID)
	if !ok || !client.IsConnected() {
		database.DB.Exec("UPDATE bulk_jobs SET status = 'failed' WHERE id = ?", jobID)
		database.DB.Exec("UPDATE bulk_items SET status = 'failed' WHERE job_id = ? AND status = 'pending'", jobID)
		return
	}

	rand.Seed(time.Now().UnixNano())

	// 2. Loop satu per satu, BUKAN borongan
	for {
		if !client.IsConnected() {
			break
		}

		// Cari 1 pesan yang masih pending
		var itemID int
		var target, message string
		err := database.DB.QueryRow("SELECT id, target, message FROM bulk_items WHERE job_id = ? AND status = 'pending' ORDER BY id ASC LIMIT 1", jobID).Scan(&itemID, &target, &message)
		
		if err != nil {
			// Jika error (tidak ada lagi yang pending), keluar dari loop
			break 
		}

		// PROTEKSI ANTI-DOUBLE: Coba kunci baris ini. Jika RowsAffected 0, berarti sudah diambil proses lain.
		res, _ := database.DB.Exec("UPDATE bulk_items SET status = 'processing' WHERE id = ? AND status = 'pending'", itemID)
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			continue // Lewati jika sudah keduluan dieksekusi
		}

		// Proses Spintext dan Kirim
		finalMsg := s.processSpintext(message)
		errSend := s.SendMessage(sessionID, target, finalMsg)

		if errSend != nil {
			database.DB.Exec("UPDATE bulk_items SET status = 'failed' WHERE id = ?", itemID)
			database.DB.Exec("UPDATE bulk_jobs SET failed = failed + 1 WHERE id = ?", jobID)
		} else {
			database.DB.Exec("UPDATE bulk_items SET status = 'success' WHERE id = ?", itemID)
			database.DB.Exec("UPDATE bulk_jobs SET success = success + 1 WHERE id = ?", jobID)
		}

		delay := minDelay
		if maxDelay > minDelay {
			delay = rand.Intn(maxDelay-minDelay+1) + minDelay
		}
		
		time.Sleep(time.Duration(delay) * time.Second)
	}

	// 3. Pastikan job ditandai completed BILA semua item sudah tidak ada yang pending/processing
	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM bulk_items WHERE job_id = ? AND status IN ('pending', 'processing')", jobID).Scan(&count)
	if count == 0 {
		database.DB.Exec("UPDATE bulk_jobs SET status = 'completed' WHERE id = ?", jobID)
	}
}

// StartWorker HANYA meresume job yang 'pending' saat server baru menyala.
// Tidak boleh meresume job 'running' agar tidak tabrakan (double send).
func (s *BulkService) StartWorker() {
	go func() {
		for {
			var jobID int64
			err := database.DB.QueryRow("SELECT id FROM bulk_jobs WHERE status = 'pending' ORDER BY id ASC LIMIT 1").Scan(&jobID)
			if err == nil {
				s.ProcessJob(jobID)
			}
			time.Sleep(15 * time.Second)
		}
	}()
}
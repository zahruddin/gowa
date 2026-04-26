package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gowa/internal/database"
	"gowa/internal/service"
)

type BulkHandler struct {
	BulkService *service.BulkService
}

func NewBulkHandler(bs *service.BulkService) *BulkHandler {
	return &BulkHandler{BulkService: bs}
}

type BulkCampaignRequest struct {
	SessionID string `json:"session_id"`
	MinDelay  int    `json:"min_delay"`
	MaxDelay  int    `json:"max_delay"`
	Items     []struct {
		Target  string `json:"target"`
		Message string `json:"message"`
	} `json:"items"`
}

func (h *BulkHandler) StartCampaign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BulkCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.SessionID == "" || len(req.Items) == 0 {
		http.Error(w, "Session ID and Items are required", http.StatusBadRequest)
		return
	}

	if req.MinDelay <= 0 {
		req.MinDelay = 3
	}
	if req.MaxDelay < req.MinDelay {
		req.MaxDelay = req.MinDelay
	}

	// 1. Create a bulk job
	jobID, err := h.BulkService.CreateJob(req.SessionID, len(req.Items), req.MinDelay, req.MaxDelay)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Insert items
	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// PENTING: Mencegah data tersangkut jika terjadi error sebelum commit
	defer tx.Rollback()

	// PENTING: Tambahkan status default 'pending' agar bisa dieksekusi oleh ProcessJob
	stmt, err := tx.Prepare("INSERT INTO bulk_items (job_id, target, message, status) VALUES (?, ?, ?, 'pending')")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	for _, item := range req.Items {
		// Validasi error saat exec untuk rollback
		if _, err := stmt.Exec(jobID, item.Target, item.Message); err != nil {
			http.Error(w, "Failed to insert item: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Jalankan pengiriman di background (Goroutine)
	go h.BulkService.ProcessJob(jobID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"job_id":  jobID,
		"message": fmt.Sprintf("Bulk campaign started with %d targets", len(req.Items)),
	})
}

func (h *BulkHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	query := "SELECT id, session_id, total, success, failed, status, created_at FROM bulk_jobs"
	var args []interface{}
	
	if sessionID != "" {
		query += " WHERE session_id = ?"
		args = append(args, sessionID)
	}
	query += " ORDER BY id DESC LIMIT 50"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var jobs []map[string]interface{}
	for rows.Next() {
		var id, total, success, failed int
		var sid, status, created string
		rows.Scan(&id, &sid, &total, &success, &failed, &status, &created)
		jobs = append(jobs, map[string]interface{}{
			"id":         id,
			"session_id": sid,
			"total":      total,
			"success":    success,
			"failed":     failed,
			"status":     status,
			"created_at": created,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (h *BulkHandler) VerifyNumbers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string   `json:"session_id"`
		Numbers   []string `json:"numbers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, ok := h.BulkService.BotManager.GetClient(req.SessionID)
	if !ok || !client.IsConnected() {
		http.Error(w, "Session not found or not connected", http.StatusNotFound)
		return
	}

	// 1. Inisialisasi semua nomor dengan status false (Invalid) dari awal.
	results := make(map[string]bool)
	for _, num := range req.Numbers {
		results[num] = false
	}

	// 2. Proses dalam chunk (kelompok) untuk mencegah banned dan timeout
	chunkSize := 5
	for i := 0; i < len(req.Numbers); i += chunkSize {
		end := i + chunkSize
		if end > len(req.Numbers) {
			end = len(req.Numbers)
		}
		
		chunk := req.Numbers[i:end]
		
		formattedChunk := make([]string, len(chunk))
		queryToOriginal := make(map[string]string)
		
		for j, num := range chunk {
			formatted := strings.TrimSpace(num)
			formatted = strings.TrimPrefix(formatted, "+")
			
			if strings.HasPrefix(formatted, "0") {
				formatted = "62" + formatted[1:]
			}
			
			queryWithPlus := "+" + formatted
			formattedChunk[j] = queryWithPlus
			queryToOriginal[queryWithPlus] = num
		}

		// 3. Panggil API pengecekan WA
		resp, err := client.IsOnWhatsApp(context.Background(), formattedChunk)
		
		if err == nil {
			for _, res := range resp {
				if orig, exists := queryToOriginal[res.Query]; exists {
					results[orig] = res.IsIn
				}
			}
		} else {
			fmt.Printf("Warning: Failed to verify chunk %v: %v\n", formattedChunk, err)
		}

		// 4. Jeda antar chunk (1.5 detik)
		if end < len(req.Numbers) {
			time.Sleep(1500 * time.Millisecond)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *BulkHandler) GetJobDetails(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}

	query := "SELECT id, target, status FROM bulk_items WHERE job_id = ? ORDER BY id ASC"
	rows, err := database.DB.Query(query, jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int
		var target, status string
		rows.Scan(&id, &target, &status) 
		
		items = append(items, map[string]interface{}{
			"id":     id,
			"target": target,
			"status": status,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}


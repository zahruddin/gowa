package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	// "strings"
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

func (h *BulkHandler) UploadBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sessionID := r.FormValue("session_id")
	message := r.FormValue("message")
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if sessionID == "" || message == "" {
		http.Error(w, "Session ID and Message are required", http.StatusBadRequest)
		return
	}

	// Save file temporarily
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), header.Filename)
	path := filepath.Join("uploads", filename)
	dst, err := os.Create(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	// In a real app, we would parse Excel here using excelize.
	// For now, we'll just create a job record and return success.
	// Since we haven't added excelize yet, I'll simulate it.

	jobID, err := h.BulkService.CreateJob(sessionID, 0) // Total will be updated after parsing
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"job_id": jobID,
		"message": "Bulk job created and queued",
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
			"id": id,
			"session_id": sid,
			"total": total,
			"success": success,
			"failed": failed,
			"status": status,
			"created_at": created,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

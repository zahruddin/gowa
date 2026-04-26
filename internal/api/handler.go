package api

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"gowa/internal/bot"
	"gowa/internal/database"
	"net/http"
	"strings"
)

type APIHandler struct {
	BotManager *bot.Manager
}

func NewAPIHandler(bm *bot.Manager) *APIHandler {
	return &APIHandler{BotManager: bm}
}

func (h *APIHandler) ExportKatalogCSV(w http.ResponseWriter, r *http.Request) {
    sessionID := r.URL.Query().Get("session_id")

    // --- BARIS DEBUGGING (Tambahkan ini) ---
    fmt.Printf("[CSV EXPORT] Mencari data untuk Session ID: '%s'\n", sessionID)

    if sessionID == "" {
        http.Error(w, "Session ID wajib diisi", http.StatusBadRequest)
        return
    }

    rows, err := database.DB.Query("SELECT keyword, details FROM katalog WHERE session_id = ?", sessionID)
    if err != nil {
        http.Error(w, "Gagal mengambil data: "+err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    w.Header().Set("Content-Type", "text/csv; charset=utf-8")
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=katalog_%s.csv", sessionID))

    // Tulis UTF-8 BOM agar rapi
    w.Write([]byte{0xEF, 0xBB, 0xBF})

    writer := csv.NewWriter(w)
    writer.Write([]string{"Keyword", "Details"})

    count := 0 // Hitung jumlah data
    for rows.Next() {
        var keyword, details string
        if err := rows.Scan(&keyword, &details); err == nil {
            cleanDetails := strings.ReplaceAll(details, "\n", "\\n")
            cleanDetails = strings.ReplaceAll(cleanDetails, "\r", "")
            writer.Write([]string{keyword, cleanDetails})
            count++
        }
    }
    writer.Flush()

    fmt.Printf("[CSV EXPORT] Berhasil mengekspor %d baris\n", count)
}

func (h *APIHandler) ImportKatalogCSV(w http.ResponseWriter, r *http.Request) {
    sendError := func(msg string, code int) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(code)
        json.NewEncoder(w).Encode(map[string]string{"error": msg})
    }

    if r.Method != http.MethodPost {
        sendError("Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    err := r.ParseMultipartForm(10 << 20) // 10 MB limit
    if err != nil {
        sendError("Error parsing form", http.StatusBadRequest)
        return
    }

    sessionID := r.FormValue("session_id")
    if sessionID == "" {
        sendError("Session ID wajib diisi", http.StatusBadRequest)
        return
    }

    file, _, err := r.FormFile("file")
    if err != nil {
        sendError("Error retrieving the file", http.StatusBadRequest)
        return
    }
    defer file.Close()

    reader := csv.NewReader(file)
    // Read header
    _, err = reader.Read()
    if err != nil {
        sendError("Error reading CSV header", http.StatusBadRequest)
        return
    }

    records, err := reader.ReadAll()
    if err != nil {
        sendError("Error reading CSV records", http.StatusBadRequest)
        return
    }

    tx, err := database.DB.Begin()
    if err != nil {
        sendError("Database error", http.StatusInternalServerError)
        return
    }
    
    checkStmt, err := tx.Prepare("SELECT 1 FROM katalog WHERE session_id = ? AND keyword = ?")
    if err != nil {
        tx.Rollback()
        sendError("Database prepare error", http.StatusInternalServerError)
        return
    }
    defer checkStmt.Close()

    insertStmt, err := tx.Prepare("INSERT INTO katalog (session_id, keyword, details) VALUES (?, ?, ?)")
    if err != nil {
        tx.Rollback()
        sendError("Database prepare error", http.StatusInternalServerError)
        return
    }
    defer insertStmt.Close()

    updateStmt, err := tx.Prepare("UPDATE katalog SET details = ? WHERE session_id = ? AND keyword = ?")
    if err != nil {
        tx.Rollback()
        sendError("Database prepare error", http.StatusInternalServerError)
        return
    }
    defer updateStmt.Close()

    inserted := 0
    updated := 0

    for _, record := range records {
        if len(record) < 2 {
            continue
        }
        keyword := record[0]
        details := record[1]
        
        details = strings.ReplaceAll(details, "\\n", "\n")

        var exists int
        err := checkStmt.QueryRow(sessionID, keyword).Scan(&exists)
        if err != nil && err != sql.ErrNoRows {
            tx.Rollback()
            sendError("Error checking data: "+err.Error(), http.StatusInternalServerError)
            return
        }

        if err == sql.ErrNoRows {
            _, err = insertStmt.Exec(sessionID, keyword, details)
            if err != nil {
                tx.Rollback()
                sendError("Error inserting data: "+err.Error(), http.StatusInternalServerError)
                return
            }
            inserted++
        } else {
            _, err = updateStmt.Exec(details, sessionID, keyword)
            if err != nil {
                tx.Rollback()
                sendError("Error updating data: "+err.Error(), http.StatusInternalServerError)
                return
            }
            updated++
        }
    }
    
    err = tx.Commit()
    if err != nil {
        sendError("Error committing transaction", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "success",
        "message": fmt.Sprintf("Berhasil mengimpor: %d data baru ditambahkan, %d data lama ditimpa", inserted, updated),
        "inserted": inserted,
        "updated": updated,
    })
}
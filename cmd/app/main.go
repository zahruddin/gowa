package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gowa/internal/api"
	"gowa/internal/bot"
	"gowa/internal/database"
	"gowa/internal/service"
)

func main() {
	// 1. Initialize Database
	db := database.InitDB("./gowa_data.db")
	defer db.Close()

	os.MkdirAll("./uploads", 0755)

	// 2. Initialize Bot Manager
	botManager, err := bot.NewManager("examplestore.db")
	if err != nil {
		log.Fatalf("Failed to initialize Bot Manager: %v", err)
	}

	// 3. Initialize Services
	bulkService := service.NewBulkService(botManager)
	bulkService.StartWorker()

	// 4. Load existing sessions from DB and start them
	rows, err := db.Query("SELECT id FROM sessions WHERE status != 'deleted'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			rows.Scan(&id)
			go botManager.InitClient(id)
		}
	}

	// 5. Initialize API Handlers
	apiHandler := api.NewAPIHandler(botManager)
	bulkHandler := api.NewBulkHandler(bulkService)

	// 6. Setup Routes
	mux := http.NewServeMux()

	// --- Session Management ---
	mux.HandleFunc("/api/sessions", apiHandler.ListSessions)
	mux.HandleFunc("/api/sessions/create", apiHandler.CreateSession)
	mux.HandleFunc("/api/sessions/delete", apiHandler.DeleteSession)
	mux.HandleFunc("/api/sessions/connect", apiHandler.ConnectSession)
	mux.HandleFunc("/api/sessions/disconnect", apiHandler.DisconnectSession)
	mux.HandleFunc("/api/sessions/qr", apiHandler.GetSessionQR)
	mux.HandleFunc("/api/sessions/webhook", apiHandler.UpdateWebhook)

	// --- Katalog ---
	mux.HandleFunc("/api/katalog", apiHandler.ListKatalog)
	mux.HandleFunc("/api/katalog/save", apiHandler.SaveKatalog)
	mux.HandleFunc("/api/katalog/delete", apiHandler.DeleteKatalog)
	mux.HandleFunc("/api/katalog/export", apiHandler.ExportKatalogCSV)
	mux.HandleFunc("/api/katalog/import", apiHandler.ImportKatalogCSV)

	// --- Whitelist ---
	mux.HandleFunc("/api/whitelist", apiHandler.ListWhitelist)
	mux.HandleFunc("/api/whitelist/save", apiHandler.SaveWhitelist)
	mux.HandleFunc("/api/whitelist/delete", apiHandler.DeleteWhitelist)

	// --- Bulk Sender ---
	mux.HandleFunc("/api/bulk/start", bulkHandler.StartCampaign)
	mux.HandleFunc("/api/bulk/jobs", bulkHandler.ListJobs)
	mux.HandleFunc("/api/bulk/verify", bulkHandler.VerifyNumbers)
	mux.HandleFunc("/api/bulk/job-details", bulkHandler.GetJobDetails)

	// --- External API (Token Auth) ---
	mux.HandleFunc("/api/send-text", apiHandler.AuthMiddleware(apiHandler.SendChatMessage))

	// --- Monitoring ---
	mux.HandleFunc("/api/system/stats", apiHandler.GetSystemStats)

	// --- Web Dashboard ---
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/index.html")
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/templates/about.html")
	})

	// Static Assets
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// 7. Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		fmt.Printf("🚀 GOWA Enterprise is running on http://localhost:%s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen: %s\n", err)
		}
	}()

	// 8. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	fmt.Println("\nShutting down GOWA...")
}
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

var adminNumbers = map[string]bool{
	"6287895496320": true,
}

var client *whatsmeow.Client
var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./gowa_katalog.db?_journal_mode=WAL")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Tabel Katalog & Tabel Settings untuk Payment
	query := `
	CREATE TABLE IF NOT EXISTS katalog (keyword TEXT UNIQUE, details TEXT);
	CREATE TABLE IF NOT EXISTS settings (key TEXT UNIQUE, value TEXT);
	`
	db.Exec(query)
	
	// Template Default
	if _, err := os.Stat("template_list.txt"); os.IsNotExist(err) {
		defaultTemplate := "🏪 *KATALOG KAMI*\n⌚ {time} | 📅 {date}\n\n{list_items}\n\n💡 Ketik nama produk untuk cek harga."
		os.WriteFile("template_list.txt", []byte(defaultTemplate), 0644)
	}
}

// Fungsi kirim teks
func sendMessage(target types.JID, text string) {
	client.SendMessage(context.Background(), target, &waE2E.Message{Conversation: proto.String(text)})
}

// Fungsi kirim gambar (QRIS) - PERBAIKAN HURUF KAPITAL (URL & SHA256)
func sendImage(target types.JID, path string, caption string) {
	data, err := os.ReadFile(path)
	if err != nil { return }

	resp, err := client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil { return }

	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(http.DetectContentType(data)),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
		},
	}
	client.SendMessage(context.Background(), target, msg)
}

func isAllowed(v *events.Message) bool {
	sender := v.Info.Sender.User
	if adminNumbers[sender] { return true }
	if v.Info.IsGroup {
		groupInfo, err := client.GetGroupInfo(context.Background(), v.Info.Chat)
		if err != nil { return false }
		for _, participant := range groupInfo.Participants {
			if participant.JID.User == sender {
				return participant.IsAdmin || participant.IsSuperAdmin
			}
		}
	}
	return false
}

func getTemplate() string {
	content, _ := os.ReadFile("template_list.txt")
	return string(content)
}

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsFromMe { return }

		msgText := v.Message.GetConversation()
		if msgText == "" && v.Message.GetExtendedTextMessage() != nil {
			msgText = v.Message.GetExtendedTextMessage().GetText()
		}

		rawMsg := strings.TrimSpace(msgText)
		if rawMsg == "" { return }
		lowerMsg := strings.ToLower(rawMsg)

		// 1. FITUR PAYMENT (QRIS)
		if lowerMsg == "payment" || lowerMsg == ".payment" {
			var caption string
			db.QueryRow("SELECT value FROM settings WHERE key = 'payment_caption'").Scan(&caption)
			if caption == "" { caption = "Silakan lakukan pembayaran melalui QRIS di atas." }
			
			if _, err := os.Stat("./uploads/qris.png"); err == nil {
				sendImage(v.Info.Chat, "./uploads/qris.png", caption)
			} else {
				sendMessage(v.Info.Chat, "⚠️ Gambar QRIS belum dikonfigurasi oleh Admin. Silakan atur melalui Web Panel.")
			}
			return
		}

		// 2. Perintah Admin Chat: addlist & dellist
		if strings.HasPrefix(lowerMsg, "addlist ") {
			if !isAllowed(v) { return }
			parts := strings.SplitN(rawMsg[8:], "@", 2)
			if len(parts) == 2 {
				db.Exec("REPLACE INTO katalog (keyword, details) VALUES (?, ?)", strings.ToLower(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1]))
				sendMessage(v.Info.Chat, "✅ Berhasil disimpan.")
			}
			return
		}

		if strings.HasPrefix(lowerMsg, "dellist ") {
			if !isAllowed(v) { return }
			db.Exec("DELETE FROM katalog WHERE keyword = ?", strings.TrimSpace(lowerMsg[8:]))
			sendMessage(v.Info.Chat, "🗑️ Berhasil dihapus.")
			return
		}

		// 3. Perintah List
		if lowerMsg == "list" {
			loc, _ := time.LoadLocation("Asia/Jakarta")
			now := time.Now().In(loc)
			timeStr := now.Format("15:04:05 WIB")
			dateStr := fmt.Sprintf("%02d-%02d-%d", now.Day(), now.Month(), now.Year())

			rows, _ := db.Query("SELECT keyword FROM katalog ORDER BY keyword ASC")
			defer rows.Close()
			var listBuilder strings.Builder
			for rows.Next() {
				var kw string
				rows.Scan(&kw)
				listBuilder.WriteString(fmt.Sprintf("▪️ %s\n", kw))
			}
			
			items := listBuilder.String()
			if items == "" { items = "(Kosong)\n" }

			finalMsg := getTemplate()
			finalMsg = strings.ReplaceAll(finalMsg, "{time}", timeStr)
			finalMsg = strings.ReplaceAll(finalMsg, "{date}", dateStr)
			finalMsg = strings.ReplaceAll(finalMsg, "{list_items}", items)

			sendMessage(v.Info.Chat, finalMsg)
			return
		}

		// 4. Auto-Response
		var details string
		err := db.QueryRow("SELECT details FROM katalog WHERE keyword = ?", lowerMsg).Scan(&details)
		if err == nil { sendMessage(v.Info.Chat, details) }
	}
}

// --- WEB SERVER ---
func webServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rows, _ := db.Query("SELECT keyword, details FROM katalog")
		var tableRows strings.Builder
		for rows.Next() {
			var kw, dt string
			rows.Scan(&kw, &dt)
			tableRows.WriteString(fmt.Sprintf(`<tr class="border-b"><td class="p-2 font-bold">%s</td><td class="p-2 text-sm whitespace-pre-wrap">%s</td><td class="p-2 text-center"><a href="/delete?kw=%s" class="text-red-500 font-bold">Hapus</a></td></tr>`, kw, dt, kw))
		}
		rows.Close()

		var currentCaption string
		db.QueryRow("SELECT value FROM settings WHERE key = 'payment_caption'").Scan(&currentCaption)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
		<!DOCTYPE html><html><head><meta charset="UTF-8"><script src="https://cdn.tailwindcss.com"></script><title>Bot Panel</title></head>
		<body class="bg-gray-100 p-4 md:p-8 font-sans">
			<div class="max-w-5xl mx-auto space-y-6">
				<h1 class="text-2xl font-bold bg-white p-4 rounded shadow text-center text-blue-600">🤖 GOWA Bot Panel</h1>
				
				<div class="grid grid-cols-1 md:grid-cols-3 gap-6">
					<!-- Form Katalog -->
					<div class="bg-white p-6 rounded shadow border-t-4 border-blue-500">
						<h2 class="font-bold mb-4">➕ Katalog Produk</h2>
						<form action="/add" method="POST" class="space-y-3">
							<input name="kw" placeholder="Keyword" class="w-full p-2 border rounded" required>
							<textarea name="dt" placeholder="Isi Pesan" class="w-full p-2 border rounded h-24" required></textarea>
							<button class="w-full bg-blue-500 text-white py-2 rounded font-bold hover:bg-blue-600">Simpan</button>
						</form>
					</div>

					<!-- Form Template -->
					<div class="bg-white p-6 rounded shadow border-t-4 border-green-500">
						<h2 class="font-bold mb-4">📝 Template List</h2>
						<form action="/update-template" method="POST" class="space-y-3">
							<textarea name="template" class="w-full p-2 border rounded h-32 text-xs font-mono" required>%s</textarea>
							<button class="w-full bg-green-500 text-white py-2 rounded font-bold hover:bg-green-600">Update Template</button>
						</form>
					</div>

					<!-- Form Payment (QRIS) -->
					<div class="bg-white p-6 rounded shadow border-t-4 border-purple-500">
						<h2 class="font-bold mb-4">💳 Set QRIS Payment</h2>
						<form action="/upload-qris" method="POST" enctype="multipart/form-data" class="space-y-3">
							<input type="file" name="qris" accept="image/*" class="w-full text-xs p-1 border rounded" required>
							<textarea name="caption" placeholder="Pesan Payment (Contoh: Dana 0812xxx)" class="w-full p-2 border rounded text-xs h-16">%s</textarea>
							<button class="w-full bg-purple-500 text-white py-2 rounded font-bold hover:bg-purple-600">Upload QRIS</button>
						</form>
						<p class="text-[10px] text-gray-500 mt-2">*User dapat memanggil QRIS dengan mengetik "payment" di chat.</p>
					</div>
				</div>

				<div class="bg-white p-6 rounded shadow">
					<table class="w-full text-left">
						<tr class="bg-gray-200"><th class="p-2">Keyword</th><th class="p-2">Isi Pesan</th><th class="p-2 text-center">Aksi</th></tr>
						%s
					</table>
				</div>
			</div>
		</body></html>`, getTemplate(), currentCaption, tableRows.String())
	})

	// Handler Upload QRIS
	http.HandleFunc("/upload-qris", func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("qris")
		if err == nil {
			defer file.Close()
			out, _ := os.Create("./uploads/qris.png")
			defer out.Close()
			io.Copy(out, file)
		}
		
		caption := r.FormValue("caption")
		db.Exec("REPLACE INTO settings (key, value) VALUES ('payment_caption', ?)", caption)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		db.Exec("REPLACE INTO katalog (keyword, details) VALUES (?, ?)", strings.ToLower(r.FormValue("kw")), r.FormValue("dt"))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		db.Exec("DELETE FROM katalog WHERE keyword = ?", r.URL.Query().Get("kw"))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/update-template", func(w http.ResponseWriter, r *http.Request) {
		os.WriteFile("template_list.txt", []byte(r.FormValue("template")), 0644)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.ListenAndServe(":8080", nil)
}

func main() {
	initDB()
	os.MkdirAll("./uploads", 0755) // Pastikan folder uploads otomatis terbuat
	container, _ := sqlstore.New(context.Background(), "sqlite3", "file:examplestore.db?_foreign_keys=on", waLog.Stdout("Database", "WARN", true))
	deviceStore, _ := container.GetFirstDevice(context.Background())
	client = whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "WARN", true))
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		client.Connect()
		for evt := range qrChan {
			if evt.Event == "code" { qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout) }
		}
	} else { client.Connect() }

	go webServer()
	fmt.Println("\nWeb Panel berjalan di: http://localhost:8080")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	client.Disconnect()
}
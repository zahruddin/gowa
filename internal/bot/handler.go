package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"os"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gowa/internal/database"
)

// 1. FUNGSI HELPER UNTUK MENDAPATKAN NOMOR (DRY - Don't Repeat Yourself)
func getSenderNumber(v *events.Message) string {
	id1 := v.Info.Sender.User
	id2 := ""
	if v.Info.MessageSource.SenderAlt.User != "" {
		id2 = v.Info.MessageSource.SenderAlt.User
	}

	senderNumber := id1
	if id2 != "" {
		if strings.HasPrefix(id1, "62") {
			senderNumber = id1
		} else if strings.HasPrefix(id2, "62") {
			senderNumber = id2
		} else {
			if len(id2) < len(id1) {
				senderNumber = id2
			} else {
				senderNumber = id1
			}
		}
	}

	if strings.Contains(senderNumber, "@") {
		senderNumber = strings.Split(senderNumber, "@")[0]
	}
	return senderNumber
}

// 2. HANDLER UTAMA YANG SUDAH DIBERSIHKAN
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

			// Ekstrak pesan
			// --- EKSTRAKSI TEKS (Teks Biasa, Teks Format, atau Caption Gambar) ---
			msgText := v.Message.GetConversation()

			if msgText == "" && v.Message.GetExtendedTextMessage() != nil {
				msgText = v.Message.GetExtendedTextMessage().GetText()
			}

			// TAMBAHKAN INI: Ambil caption jika pesan berupa Gambar
			if msgText == "" && v.Message.GetImageMessage() != nil {
				msgText = v.Message.GetImageMessage().GetCaption()
			}

			rawMsg := strings.TrimSpace(msgText)
			if rawMsg == "" {
				// Jika benar-benar tidak ada teks (misal cuma kirim gambar tanpa caption), abaikan
				return
			}
			lowerMsg := strings.ToLower(rawMsg)

			// Dapatkan nomor dengan 1 baris kode (memanggil helper)
			senderNumber := getSenderNumber(v)

			// --- RUTING PESAN (ROUTING) ---
			// Jika pesan diawali '#', lempar ke router Admin
			if strings.HasPrefix(rawMsg, ".") {
				m.handleAdminCommands(sessionID, client, v, senderNumber, lowerMsg, rawMsg)
				return // Hentikan eksekusi di sini jika ini adalah perintah '.'
			}

			// Jika bukan '.', lempar ke router Publik
			m.handlePublicCommands(sessionID, client, v, senderNumber, lowerMsg)
		}
	}
}

// 3. ROUTER KHUSUS ADMIN (Middlewaring)
func (m *Manager) handleAdminCommands(sessionID string, client *whatsmeow.Client, v *events.Message, senderNumber, lowerMsg, rawMsg string) {
	// Middleware Cek Admin ada DI SINI
	if !m.IsAdmin(sessionID, senderNumber) {
		// (Opsional) Bisa beri pesan "Anda bukan admin" atau diam saja
		// m.SendMessage(client, v.Info.Chat, "❌ Akses ditolak. Anda bukan admin.")
		return
	}

	// Logika Perintah Admin
	if strings.HasPrefix(lowerMsg, ".setwebhook ") {
		webhook := strings.TrimSpace(rawMsg[12:])
		database.DB.Exec("UPDATE sessions SET webhook_url = ? WHERE id = ?", webhook, sessionID)
		m.SendMessage(client, v.Info.Chat, "✅ Webhook updated.")
		return
	}

	if strings.HasPrefix(lowerMsg, ".addkatalog ") {
		parts := strings.SplitN(rawMsg[12:], "@", 2)
		if len(parts) == 2 {
			keyword := strings.ToLower(strings.TrimSpace(parts[0]))
			detail := strings.TrimSpace(parts[1])
			database.DB.Exec("REPLACE INTO katalog (session_id, keyword, details) VALUES (?, ?, ?)", sessionID, keyword, detail)
			m.SendMessage(client, v.Info.Chat, "✅ Katalog '"+keyword+"' updated.")
		}
		return
	}

	if strings.HasPrefix(lowerMsg, ".rmkatalog ") {
		keyword := strings.ToLower(strings.TrimSpace(rawMsg[11:]))
		if keyword != "" {
			database.DB.Exec("DELETE FROM katalog WHERE session_id = ? AND keyword = ?", sessionID, keyword)
			m.SendMessage(client, v.Info.Chat, "✅ Katalog dihapus (jika ada).")
		}
		return
	}

	if strings.HasPrefix(lowerMsg, ".setqris") {
        // fmt.Printf("[%s] Perintah #setqris terdeteksi!\n", sessionID)
        
		img := v.Message.GetImageMessage()
		if img == nil {
			fmt.Printf("[%s] Error: Pesan bukan ImageMessage\n", sessionID)
			m.SendMessage(client, v.Info.Chat, "⚠️ Kirim gambar QRIS dengan caption #setqris")
			return
		}

		// 1. Download gambar dari WhatsApp
		// Tambahkan context.Background() sebagai argumen pertama
		data, err := client.Download(context.Background(), img)
		if err != nil {
			m.SendMessage(client, v.Info.Chat, "❌ Gagal mengunduh gambar.")
			return
		}

		// 2. Simpan gambar secara lokal di folder /uploads (buat folder ini manual)
		fileName := fmt.Sprintf("uploads/qris_%s.jpg", sessionID)
		os.WriteFile(fileName, data, 0644)

		// 3. Simpan path dan caption ke database
		caption := strings.TrimSpace(rawMsg[8:]) // Mengambil teks setelah #setqris
		if caption == "" {
			caption = "Silahkan scan QRIS di atas untuk melakukan pembayaran."
		}

		_, dbErr := database.DB.Exec(`
			INSERT INTO payment_settings (session_id, qris_data, caption) 
			VALUES (?, ?, ?) 
			ON CONFLICT(session_id) DO UPDATE SET qris_data=excluded.qris_data, caption=excluded.caption`,
			sessionID, fileName, caption)

		if dbErr != nil {
			m.SendMessage(client, v.Info.Chat, "❌ Gagal menyimpan ke database.")
		} else {
			m.SendMessage(client, v.Info.Chat, "✅ QRIS berhasil diperbarui!")
		}
		return
	}

	if lowerMsg == ".status" {
		m.SendMessage(client, v.Info.Chat, "🤖 System: Online\nStatus: Admin")
		return
	}
    
	if lowerMsg == ".myid" {
		m.SendMessage(client, v.Info.Chat, fmt.Sprintf("📋 ID Anda: %s", senderNumber))
		return
	}

	if lowerMsg == ".menu" {
        adminMenu := "🛠️ *ADMIN DASHBOARD MENU*\n\n" +
            " Berikut adalah daftar perintah konfigurasi:\n\n" +
            "📝 *KATALOG*\n" +
            "• `#addkatalog keyword@pesan` : Menambah/update katalog.\n" +
            "• `#rmkatalog keyword` : Menghapus item dari katalog.\n\n" +
            "💳 *PAYMENT (QRIS)*\n" +
            "• `#setqris [caption]` : Kirim gambar QRIS dengan caption ini untuk mengatur metode pembayaran.\n\n" +
            "⚙️ *SISTEM*\n" +
            "• `#setwebhook [URL]` : Mengatur URL tujuan webhook.\n" +
            "• `#status` : Cek status bot dan session.\n" +
            "• `#myid` : Cek nomor identitas WhatsApp Anda.\n\n" +
            "• `#all [pesan]` : Membuat pengumuman ke semua anggota grup.\n\n" +
            "💡 _Gunakan perintah di atas persis seperti format yang tertera._"

        m.SendMessage(client, v.Info.Chat, adminMenu)
        return
    }

	if strings.HasPrefix(lowerMsg, ".all") {
        if !v.Info.IsGroup {
            m.SendMessage(client, v.Info.Chat, "❌ Gunakan di grup!")
            return
        }

        // 1. Ambil isi pesan secara aman
        // Kita cari posisi spasi pertama. Jika tidak ada spasi, berarti pesan kustom kosong.
        isiPesan := ""
        if firstSpace := strings.Index(rawMsg, " "); firstSpace != -1 {
            isiPesan = strings.TrimSpace(rawMsg[firstSpace:])
        }
        
        if isiPesan == "" {
            isiPesan = "Halo semuanya, ada informasi penting!"
        }

        // 2. Ambil JID anggota dengan penanganan error
        groupInfo, err := client.GetGroupInfo(context.Background(), v.Info.Chat)
        if err != nil {
            fmt.Printf("Error GetGroupInfo: %v\n", err)
            return
        }

        var mentionedJIDs []string
        for _, participant := range groupInfo.Participants {
            mentionedJIDs = append(mentionedJIDs, participant.JID.String())
        }

        // 3. Kirim pesan Ghost Mention (Tag Tersembunyi)
        client.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
            ExtendedTextMessage: &waE2E.ExtendedTextMessage{
                Text: proto.String("📢 *PENGUMUMAN:* \n\n" + isiPesan),
                ContextInfo: &waE2E.ContextInfo{
                    MentionedJID: mentionedJIDs,
                },
            },
        })
        return
    }

	// --- FITUR BUKA GRUP ---
    if lowerMsg == ".open" {
        if !v.Info.IsGroup {
            m.SendMessage(client, v.Info.Chat, "❌ Perintah ini hanya berlaku di dalam grup.")
            return
        }

        // false = Membuka agar semua orang bisa chat
        err := client.SetGroupAnnounce(context.Background(), v.Info.Chat, false)
        if err != nil {
            fmt.Printf("[%s] Error SetGroupAnnounce: %v\n", sessionID, err)
            m.SendMessage(client, v.Info.Chat, "❌ Gagal membuka grup. Pastikan Bot adalah Admin!")
        } else {
            m.SendMessage(client, v.Info.Chat, "🔓 *Grup Berhasil Dibuka.*\nSekarang semua peserta dapat mengirim pesan.")
        }
        return
    }

    // --- FITUR TUTUP GRUP ---
    if lowerMsg == ".close" {
        if !v.Info.IsGroup {
            m.SendMessage(client, v.Info.Chat, "❌ Perintah ini hanya berlaku di dalam grup.")
            return
        }

        // true = Menutup agar hanya Admin yang bisa chat
        err := client.SetGroupAnnounce(context.Background(), v.Info.Chat, true)
        if err != nil {
            fmt.Printf("[%s] Error SetGroupAnnounce: %v\n", sessionID, err)
            m.SendMessage(client, v.Info.Chat, "❌ Gagal menutup grup. Pastikan Bot adalah Admin!")
        } else {
            m.SendMessage(client, v.Info.Chat, "🔒 *Grup Berhasil Ditutup.*\nSekarang hanya Admin yang dapat mengirim pesan.")
        }
        return
    }

	// --- FITUR AFK ---
    if strings.HasPrefix(lowerMsg, ".afk") {
        reason := "Tidak Ada"
        if idx := strings.Index(rawMsg, " "); idx != -1 {
            reason = rawMsg[idx+1:]
        }

        _, err := database.DB.Exec(`
            INSERT INTO admin_status (session_id, number, status, reason, since) 
            VALUES (?, ?, 'afk', ?, CURRENT_TIMESTAMP)
            ON CONFLICT(session_id, number) DO UPDATE SET status='afk', reason=excluded.reason, since=CURRENT_TIMESTAMP`,
            sessionID, senderNumber, reason)

        if err == nil {
            m.SendMessage(client, v.Info.Chat, fmt.Sprintf("💤 Admin @%s sekarang AFK.\nAlasan: %s", senderNumber, reason))
        }
        return
    }

    // --- FITUR READY ---
    if lowerMsg == ".ready" {
        var status, reason string
        var since time.Time
        
        // Ambil data AFK sebelumnya untuk hitung durasi
        err := database.DB.QueryRow("SELECT status, reason, since FROM admin_status WHERE session_id = ? AND number = ?", 
            sessionID, senderNumber).Scan(&status, &reason, &since)

        durasiStr := "00:00:00"
        if err == nil && status == "afk" {
            durasiStr = formatDuration(time.Since(since))
        }

        // Update jadi ready
        database.DB.Exec("UPDATE admin_status SET status='ready', since=CURRENT_TIMESTAMP WHERE session_id = ? AND number = ?", 
            sessionID, senderNumber)

        resMsg := fmt.Sprintf("✅ *Kembali Dari Afk*\nDengan Alasan : %s\nSelama : %s", reason, durasiStr)
        m.SendMessage(client, v.Info.Chat, resMsg)
        return
    }
}

// 4. ROUTER KHUSUS PUBLIK (Katalog, Webhook)
func (m *Manager) handlePublicCommands(sessionID string, client *whatsmeow.Client, v *events.Message, senderNumber, lowerMsg string) {
	
	// Cek Katalog
	var details string
	err := database.DB.QueryRow("SELECT details FROM katalog WHERE session_id = ? AND keyword = ?", sessionID, lowerMsg).Scan(&details)
	if err == nil && details != "" {
		m.SendMessage(client, v.Info.Chat, details)
		return
	}

	// Cek List
	if lowerMsg == "list" {
		m.SendList(client, sessionID, v.Info.Chat)
		return
	}

	// Di dalam handlePublicCommands
	if lowerMsg == "payment" {
		var qrisPath, caption string
		// 1. Ambil data dari database
		err := database.DB.QueryRow("SELECT qris_data, caption FROM payment_settings WHERE session_id = ?", sessionID).Scan(&qrisPath, &caption)
		
		if err != nil {
			// Jika tidak ada data di DB, kita diam saja atau beri pesan log
			return 
		}

		// 2. Cek apakah file benar-benar ada di folder uploads/
		if _, err := os.Stat(qrisPath); os.IsNotExist(err) {
			fmt.Printf("[%s] Error: File QRIS tidak ditemukan di %s\n", sessionID, qrisPath)
			m.SendMessage(client, v.Info.Chat, "⚠️ QRIS belum diatur oleh Admin atau file hilang.")
			return
		}

		// 3. Baca file
		fileData, err := os.ReadFile(qrisPath)
		if err != nil {
			m.SendMessage(client, v.Info.Chat, "❌ Gagal membaca file QRIS.")
			return
		}

		// 4. Upload ke server WA
		// Tips: Jika traffic sangat tinggi, simpan resp.URL dan resp.DirectPath di DB 
		// agar tidak perlu upload setiap saat.
		resp, err := client.Upload(context.Background(), fileData, whatsmeow.MediaImage)
		if err != nil {
			fmt.Printf("[%s] Upload Error: %v\n", sessionID, err)
			m.SendMessage(client, v.Info.Chat, "❌ Sistem gagal mengunggah QRIS ke WhatsApp.")
			return
		}

		// 5. Rakit Pesan
		imageMsg := &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String("image/jpeg"), // Pastikan sesuai format upload
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
		}

		// 6. Kirim
		_, err = client.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
			ImageMessage: imageMsg,
		})

		if err != nil {
			fmt.Printf("[%s] SendMessage Error: %v\n", sessionID, err)
		}
		return
	}

	// --- CEK STATUS ADMIN ---
    if lowerMsg == "admin" {
        rows, err := database.DB.Query("SELECT number FROM admin_status WHERE session_id = ? AND status = 'ready'", sessionID)
        if err != nil {
            return
        }
        defer rows.Close()

        var listReady []string
        for rows.Next() {
            var num string
            rows.Scan(&num)
            listReady = append(listReady, "@"+num)
        }

        if len(listReady) == 0 {
            m.SendMessage(client, v.Info.Chat, "🔴 Maaf, saat ini tidak ada Admin yang sedang Ready.")
        } else {
            resText := "🟢 *ADMIN READY:*\n\n" + strings.Join(listReady, "\n") + "\n\nSilahkan kirim pesan, admin akan segera membalas."
            m.SendMessage(client, v.Info.Chat, resText)
        }
        return
    }

	// Forward Webhook untuk pesan yang tidak masuk kategori apa pun
	go m.ForwardToWebhook(sessionID, v)
}

func (m *Manager) IsAdmin(sessionID string, number string) bool {
	cleanNumber := strings.TrimSpace(number)
	if cleanNumber == "" {
		return false
	}
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM whitelist WHERE session_id = ? AND number = ?)", sessionID, cleanNumber).Scan(&exists)
	return err == nil && exists
}

func (m *Manager) SendMessage(client *whatsmeow.Client, target types.JID, text string) {
	if client == nil {
		return
	}
	client.SendMessage(context.Background(), target, &waE2E.Message{Conversation: proto.String(text)})
}

func (m *Manager) SendList(client *whatsmeow.Client, sessionID string, target types.JID) {
	if client == nil {
		return
	}
	rows, _ := database.DB.Query("SELECT keyword FROM katalog WHERE session_id = ? ORDER BY keyword ASC", sessionID)
	defer rows.Close()
	
	var listBuilder strings.Builder
	listBuilder.WriteString("🏪 *KATALOG KAMI*\n\n")
	count := 0
	for rows.Next() {
		var kw string
		rows.Scan(&kw)
		listBuilder.WriteString(fmt.Sprintf("▪️ %s\n", kw))
		count++
	}
	
	if count == 0 {
		listBuilder.WriteString("(Kosong)")
	}
	
	m.SendMessage(client, target, listBuilder.String())
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

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
# 📂 Dokumentasi Struktur Folder GOWA

Proyek ini menggunakan standar **Clean Architecture** yang disesuaikan untuk kebutuhan Multi-Session WhatsApp API agar tetap ringan dijalankan di Android (Termux).

---

## 🏗️ Hirarki Direktori

### 1. `cmd/app/`
- **main.go**: Titik masuk utama aplikasi. Tugasnya hanya inisialisasi awal dan menjalankan server.

### 2. `internal/` (Core Logic)
Bagian ini bersifat privat (hanya bisa diakses di dalam proyek ini).
- **api/**: Berisi handler untuk setiap endpoint REST API (Kirim teks, media, manage session).
- **bot/**: Otak dari WhatsApp. 
    - `manager.go`: Mengelola banyak akun (Multi-session) dalam satu *Map*.
    - `client.go`: Logika per-individu akun WhatsApp.
    - `handler.go`: Menangani pesan masuk dan filter *whitelist*.
- **config/**: Tempat menyimpan pengaturan API Key, Port, dan Database path.
- **database/**: Pengaturan koneksi SQLite dan fungsi CRUD di folder `repository`.
- **middleware/**: Filter keamanan, seperti mengecek Token API sebelum mengizinkan pengiriman pesan.
- **model/**: Definisi struktur data (JSON response, tabel database).
- **service/**: Bisnis logika yang lebih kompleks, seperti sistem antrean untuk *Bulk Sender*.

### 3. `web/` (Frontend)
- **templates/**: File HTML (Dashboard Admin) menggunakan Tailwind/Bootstrap.
- **static/**: Aset pendukung seperti logo, file CSS, atau JavaScript kustom.

### 4. `uploads/`
- Tempat penyimpanan sementara file gambar, dokumen, atau QRIS sebelum dikirim/diproses.

### 5. `docs/`
- Tempat menyimpan dokumentasi API (Swagger/Markdown) dan catatan pengembangan.

---

## 🚀 Prinsip Kerja Multi-Session
Sistem menggunakan `map[string]*whatsmeow.Client` di dalam `internal/bot/manager.go`. 
1. Saat aplikasi nyala, ia membaca semua sesi di database.
2. Setiap sesi dijalankan di jalur terpisah (**Goroutine**).
3. API memanggil sesi berdasarkan ID atau Token yang dikirim di Header.
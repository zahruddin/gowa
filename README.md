🚀 GOWA - WhatsApp Store Manager BotBot pengelola toko WhatsApp otomatis yang dibangun dengan Golang dan whatsmeow. Dirancang untuk berjalan ringan di Termux (Android) maupun VPS.✨ Fitur Utama🛠️ Admin Dashboard (Chat-Based): Kelola bot langsung lewat chat.📦 Katalog Otomatis: Respon cepat untuk daftar harga layanan.💳 QRIS Payment: Kirim gambar QRIS otomatis ke pembeli.💤 AFK/Ready System: Pantau status admin dan durasi istirahat.📣 Ghost Mention: Tag semua anggota grup tanpa tumpukan nomor.🔒 Group Control: Buka/tutup grup hanya dengan satu perintah.🏢 Multi-Session: Mendukung pengelolaan banyak bot dalam satu database.📂 Struktur FolderPlaintextgowa/
├── cmd/
│   └── app/
│       └── main.go       # Entry point utama aplikasi
├── internal/
│   ├── bot/              # Logika handler dan perintah bot
│   └── database/         # Konfigurasi SQLite dan migrasi tabel
├── uploads/              # Penyimpanan gambar QRIS (.jpg)
├── go.mod                # Dependency manager
└── gowa.db               # Database SQLite (Generated otomatis)
📦 Panduan Instalasi (Termux)1. Update dan Persiapan LingkunganBuka Termux, lalu jalankan perintah ini untuk menginstal semua alat yang dibutuhkan:Bashpkg update && pkg upgrade -y
pkg install golang git sqlite build-essential -y
2. Clone Project & Setup FolderBashgit clone https://github.com/zahruddin/gowa.git
cd gowa
# Buat folder uploads manual agar tidak error saat simpan QRIS
mkdir uploads
3. Instalasi DependencyBashgo mod tidy
4. Menjalankan BotKarena struktur folder sudah standar, jalankan dari root folder menggunakan path ke main.go:Bashgo run cmd/app/main.go
Scan QR: Munculkan QR di terminal, lalu buka WhatsApp HP Anda > Perangkat Tertaut > Tautkan Perangkat.🛠️ Daftar Perintah (Prefix: .)👮 Khusus Admin (Whitelist)PerintahDeskripsi.menuMenampilkan daftar perintah admin..addkatalog key@msgMenambah/update item katalog..rmkatalog keyMenghapus item katalog..setqris [caption]Kirim gambar + caption ini untuk set QRIS..afk [alasan]Set status admin ke mode istirahat..readyKembali aktif dan tampilkan durasi AFK..all [pesan]Tag semua anggota grup (Hidden Tag)..openMembuka grup (Semua bisa chat)..closeMenutup grup (Hanya admin bisa chat)..statusCek status sistem dan session ID.👥 Perintah PublikPerintahDeskripsilistMenampilkan semua keyword katalog yang tersedia.paymentMenampilkan gambar QRIS dan cara bayar..adminCek daftar admin yang sedang Ready.#myidCek nomor WhatsApp Anda untuk didaftarkan whitelist.⚙️ Pengoperasian di Latar Belakang (Background)Agar bot tetap hidup meskipun aplikasi Termux ditutup:Aktifkan Wake Lock:Bashtermux-wake-lock
Jalankan dengan Nohup:Bashnohup go run cmd/app/main.go > bot.log 2>&1 &
Cek Log: tail -f bot.logMatikan Bot: pkill main📝 Catatan TambahanDatabase: Seluruh data sesi dan konfigurasi disimpan di gowa.db. Jika pindah perangkat, cukup bawa file ini dan folder uploads/.Whitelist: Untuk menambah admin baru, masukkan nomor mereka (format: 628xxx) ke tabel whitelist di database secara manual atau melalui panel database.
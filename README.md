📋 Panduan Instalasi di Termux (HP)
Ikuti langkah-langkah ini secara urut agar tidak ada error library:

1. Update dan Persiapan Lingkungan
Buka Termux, lalu jalankan perintah berikut untuk menginstal bahasa Go dan database SQLite:

Bash
pkg update && pkg upgrade -y
pkg install golang git sqlite -y
2. Clone Project dari GitHub
Gunakan alamat SSH atau HTTPS repository Anda (ganti username dengan username Anda):

Bash
git clone https://github.com/zahruddin/gowa.git
cd gowa
3. Instalasi Library (Dependencies)
Pastikan semua library yang dibutuhkan terunduh dengan benar:

Bash
go mod tidy
4. Menjalankan Bot
Untuk pertama kali, jalankan bot dengan perintah:

Bash
go run main.go
Scan QR: Akan muncul QR Code di terminal Termux. Gunakan HP yang ingin dijadikan bot untuk scan (pencet Settings > Linked Devices di WhatsApp).

Akses Web Panel: Setelah muncul pesan "Web Panel berjalan", buka browser di HP Anda dan ketik: http://localhost:8080

🛠️ Tips Penggunaan di Termux
Cara Menjalankan Bot di Latar Belakang
Agar bot tidak mati saat Anda menutup aplikasi Termux atau saat layar HP mati, gunakan nohup atau jalankan perintah ini:

Pasang termux-wake-lock (agar CPU HP tidak tidur):

Bash
termux-wake-lock
Jalankan bot di background:

Bash
nohup go run main.go > bot.log 2>&1 &
Bot akan tetap hidup meskipun Termux dikeluarkan. Log aktivitas bisa dicek di file bot.log.

Cara Mematikan Bot di Background
Bash
pkill main
📝 Catatan Penting
Upload QRIS: Karena kita mengabaikan folder uploads di Git, jangan lupa buka Web Panel di HP (localhost:8080), lalu upload ulang gambar QRIS Anda di menu yang tersedia.

Katalog: Isi kembali daftar layanan (Canva, Netflix, dll) melalui Web Panel atau gunakan fitur addlist di chat WhatsApp Admin.

Port: Jika port 8080 bentrok dengan aplikasi lain, Anda bisa mengubah angka :8080 di bagian paling bawah file main.go.
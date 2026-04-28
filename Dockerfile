# ==========================================
# STAGE 1: BUILDER (Kompilasi Aplikasi)
# ==========================================
# Menggunakan versi 1.25-alpine yang persis dengan versi Go di laptop kamu
FROM golang:1.25-alpine AS builder

# 1. Pasang Dependensi Wajib Sistem
# git & ca-certificates: Untuk mendownload library Go dari GitHub dengan aman
# gcc & musl-dev: Compiler C yang wajib ada untuk driver SQLite (CGO)
RUN apk add --no-cache git ca-certificates gcc musl-dev

# 2. Atur folder kerja di dalam container
WORKDIR /app

# 3. Cache Dependency (Langkah optimasi build)
# Copy go.mod dan go.sum duluan agar Docker bisa nge-cache hasil download
COPY go.mod go.sum ./
RUN go mod download -x

# 4. Copy seluruh source code project kamu
COPY . .

# 5. Build Binary Aplikasi
# CGO_ENABLED=1 wajib dinyalakan agar database SQLite (gowa_data.db) bisa terbaca
# Output file executable kita beri nama 'app-exe'
RUN CGO_ENABLED=1 GOOS=linux go build -o app-exe ./cmd/app/main.go


# ==========================================
# STAGE 2: RUNNER (Lingkungan Produksi)
# ==========================================
# Kita gunakan alpine murni agar ukuran image Docker hasil akhirnya sangat kecil
FROM alpine:latest

# 1. Pasang Dependensi Runtime
# libc6-compat: Dibutuhkan oleh file binary hasil compile CGO (SQLite)
# tzdata: Untuk mengatur zona waktu
# ffmpeg: Sangat direkomendasikan untuk bot WA jika ada fitur proses video/audio
# ca-certificates: Agar bot bisa melakukan request HTTPS/API ke luar
RUN apk --no-cache add ca-certificates ffmpeg tzdata libc6-compat

# 2. Atur Zona Waktu Server (WIB)
ENV TZ=Asia/Jakarta

WORKDIR /app

# 3. Salin file hasil build dari Stage 1 (Builder)
COPY --from=builder /app/app-exe .

# 4. Salin aset statis dan template HTML (Sesuai struktur tree kamu)
COPY --from=builder /app/web ./web

# 5. Buka port yang digunakan oleh aplikasi web kamu
EXPOSE 8080

# 6. Perintah yang dijalankan saat container dihidupkan
CMD ["./app-exe"]
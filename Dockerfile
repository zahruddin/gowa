# Stage 1: Build (Kompilasi)
# PENTING: Jika di file go.mod kamu tertulis 'go 1.23', ganti angka 1.22 di bawah ini menjadi 1.23
FROM golang:1.22-alpine AS builder

# 1. PASANG DEPENDENSI WAJIB
# git & ca-certificates: untuk mendownload library dari GitHub
# gcc & musl-dev: untuk driver SQLite (CGO)
RUN apk add --no-cache git ca-certificates gcc musl-dev

WORKDIR /app

# Copy dependency files terlebih dahulu agar proses build lebih cepat
COPY go.mod go.sum ./

# Jalankan download dengan flag -x agar log detail muncul jika terjadi masalah
RUN go mod download -x

# Copy seluruh source code
COPY . .

# 2. AKTIFKAN CGO_ENABLED=1
# Ini wajib agar driver SQLite (mattn/go-sqlite3) bisa di-compile
RUN CGO_ENABLED=1 GOOS=linux go build -o app-exe ./cmd/app/main.go


# Stage 2: Run (Lingkungan Eksekusi)
FROM alpine:latest

# Pasang library dasar untuk Alpine
# libc6-compat: dibutuhkan oleh binary hasil CGO
# ffmpeg: untuk pemrosesan video/audio (opsional sesuai kebutuhan bot)
# tzdata: untuk mengatur zona waktu server
RUN apk --no-cache add ca-certificates ffmpeg tzdata libc6-compat
ENV TZ=Asia/Jakarta

WORKDIR /app

# Salin binary hasil kompilasi dari stage builder
COPY --from=builder /app/app-exe .

# 3. SALIN FOLDER WEB
# Menyalin folder web agar halaman HTML bisa diakses
COPY --from=builder /app/web ./web

EXPOSE 8080

CMD ["./app-exe"]
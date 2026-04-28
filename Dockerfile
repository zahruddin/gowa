# Stage 1: Build (Kompilasi)
FROM golang:1.21-alpine AS builder

# 1. PASANG DEPENDENSI UNTUK SQLITE (CGO)
RUN apk add --no-cache gcc musl-dev git

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# 2. AKTIFKAN CGO_ENABLED=1
# Ini wajib agar driver SQLite (mattn/go-sqlite3) bisa di-compile
RUN CGO_ENABLED=1 GOOS=linux go build -o app-exe ./cmd/app/main.go

# Stage 2: Run (Lingkungan Eksekusi)
FROM alpine:latest

# Pasang library dasar (libc6-compat dibutuhkan oleh binary hasil CGO di Alpine)
RUN apk --no-cache add ca-certificates ffmpeg tzdata libc6-compat
ENV TZ=Asia/Jakarta

WORKDIR /app

# Salin binary dari stage builder
COPY --from=builder /app/app-exe .

# 3. SALIN FOLDER WEB (Sesuai output 'ls' kamu sebelumnya)
# Jika di dalam folder 'web' ada folder 'static' atau 'templates', 
# aplikasi Golang kamu harus diarahkan ke path '/app/web/...'
COPY --from=builder /app/web ./web

# Salin .env jika kamu memang menggunakannya
# COPY --from=builder /app/.env . 

EXPOSE 8080

CMD ["./app-exe"]
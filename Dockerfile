# Gunakan versi yang sesuai dengan go.mod kamu (misal 1.22 atau 1.23)
FROM golang:1.23-alpine AS builder

# PASANG git, gcc, DAN musl-dev (Wajib agar go mod download lancar)
RUN apk add --no-cache gcc musl-dev git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
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
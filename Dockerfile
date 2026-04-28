# Stage 1: Build
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app-exe ./cmd/app/main.go

# Stage 2: Run
FROM alpine:latest
RUN apk --no-cache add ca-certificates ffmpeg tzdata
ENV TZ=Asia/Jakarta
WORKDIR /root/
COPY --from=builder /app/app-exe .
COPY --from=builder /app/.env .
# Sesuaikan folder web di bawah ini jika namanya berbeda
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates

EXPOSE 8080
CMD ["./app-exe"]

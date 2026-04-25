package model

import "time"

type Session struct {
	ID         string    `json:"id" db:"id"`
	APIToken   string    `json:"api_token" db:"api_token"`
	WebhookURL string    `json:"webhook_url" db:"webhook_url"`
	Status     string    `json:"status" db:"status"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type Katalog struct {
	Keyword string `json:"keyword" db:"keyword"`
	Details string `json:"details" db:"details"`
}

type Whitelist struct {
	Number string `json:"number" db:"number"`
	Name   string `json:"name" db:"name"`
}

type BulkJob struct {
	ID        int       `json:"id" db:"id"`
	SessionID string    `json:"session_id" db:"session_id"`
	Total     int       `json:"total" db:"total"`
	Success   int       `json:"success" db:"success"`
	Failed    int       `json:"failed" db:"failed"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type User struct {
	ID       int    `json:"id" db:"id"`
	Username string `json:"username" db:"username"`
	Password string `json:"password" db:"password"` // In real app, use hash
}

package handlers

import "time"

// Request DTOs

type SignupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SendMessageRequest struct {
	Content string `json:"content" binding:"required,max=1024"`
}

// Response DTOs

type LoginResponse struct {
	Token string `json:"token"`
}

type MessageResponse struct {
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type APIResponse struct {
	Message string `json:"message"`
}

type HealthResponse struct {
	Status    string `json:"status"`
	MongoDB   string `json:"mongodb"`
	Messenger string `json:"messenger"`
}

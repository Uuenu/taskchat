package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Uuenu/taskchat/internal/errors"
	"github.com/Uuenu/taskchat/internal/messaging"
	"github.com/Uuenu/taskchat/internal/middleware"
	"github.com/Uuenu/taskchat/internal/repository"
	"github.com/Uuenu/taskchat/internal/service"
)

const requestTimeout = 10 * time.Second

// Handler holds all HTTP handlers and their dependencies
type Handler struct {
	authService  *service.AuthService
	chatService  *service.ChatService
	messenger    messaging.Messenger
	userRepo     repository.UserRepository
	mongoHealthy func() bool
}

// NewHandler creates a new handler with all dependencies
func NewHandler(
	authService *service.AuthService,
	chatService *service.ChatService,
	messenger messaging.Messenger,
	userRepo repository.UserRepository,
	mongoHealthy func() bool,
) *Handler {
	return &Handler{
		authService:  authService,
		chatService:  chatService,
		messenger:    messenger,
		userRepo:     userRepo,
		mongoHealthy: mongoHealthy,
	}
}

// Signup handles user registration
func (h *Handler) Signup(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	if err := h.authService.Signup(ctx, req.Email, req.Password); err != nil {
		switch err {
		case errors.ErrUserAlreadyExists:
			c.JSON(http.StatusConflict, errors.ErrorResponse{Error: "user already exists"})
		default:
			log.Printf("Signup error: %v", err)
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{Error: "registration failed"})
		}
		return
	}

	// Create account in messenger (best effort - don't fail signup if messenger is down)
	if h.messenger != nil && h.messenger.IsConnected() {
		go h.createMessengerAccount(req.Email, req.Password)
	}

	c.JSON(http.StatusCreated, APIResponse{Message: "User registered successfully"})
}

// createMessengerAccount creates a user account in the messenger and links it
func (h *Handler) createMessengerAccount(email, password string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	externalID, err := h.messenger.CreateUser(ctx, email, password)
	if err != nil {
		log.Printf("Messenger account creation failed: %v", err)
		return
	}

	if err := h.userRepo.LinkExternalAccount(ctx, email, h.messenger.Provider(), externalID); err != nil {
		log.Printf("Failed to link external account: %v", err)
	}
}

// Login handles user authentication
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	token, err := h.authService.Login(ctx, req.Email, req.Password)
	if err != nil {
		switch err {
		case errors.ErrInvalidCredentials, errors.ErrUserNotFound:
			c.JSON(http.StatusUnauthorized, errors.ErrorResponse{Error: "invalid email or password"})
		default:
			log.Printf("Login error: %v", err)
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{Error: "authentication failed"})
		}
		return
	}

	c.JSON(http.StatusOK, LoginResponse{Token: token})
}

// SendMessage handles sending a message to the general chat
func (h *Handler) SendMessage(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	userEmail := middleware.GetUserEmail(c)
	if userEmail == "" {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{Error: "user not authenticated"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	if err := h.chatService.SendMessage(ctx, userEmail, req.Content); err != nil {
		log.Printf("Send message error: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{Error: "failed to send message"})
		return
	}

	c.JSON(http.StatusOK, APIResponse{Message: "Message sent successfully"})
}

// GetMessages retrieves recent messages from the general chat
func (h *Handler) GetMessages(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	messages, err := h.chatService.GetRecentMessages(ctx)
	if err != nil {
		log.Printf("Get messages error: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{Error: "failed to retrieve messages"})
		return
	}

	response := make([]MessageResponse, len(messages))
	for i, msg := range messages {
		response[i] = MessageResponse{
			Author:    msg.Author,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		}
	}

	c.JSON(http.StatusOK, response)
}

// HealthCheck provides system health status
func (h *Handler) HealthCheck(c *gin.Context) {
	mongoStatus := "disconnected"
	if h.mongoHealthy != nil && h.mongoHealthy() {
		mongoStatus = "connected"
	}

	messengerStatus := "disconnected"
	if h.messenger != nil && h.messenger.IsConnected() {
		messengerStatus = "connected"
	}

	status := "healthy"
	if mongoStatus != "connected" && messengerStatus != "connected" {
		status = "unhealthy"
	} else if mongoStatus != "connected" || messengerStatus != "connected" {
		status = "degraded"
	}
	c.JSON(http.StatusOK, HealthResponse{
		Status:    status,
		MongoDB:   mongoStatus,
		Messenger: messengerStatus,
	})
}

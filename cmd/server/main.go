package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Uuenu/taskchat/internal/config"
	"github.com/Uuenu/taskchat/internal/handlers"
	"github.com/Uuenu/taskchat/internal/messaging"
	"github.com/Uuenu/taskchat/internal/messaging/tinode"
	"github.com/Uuenu/taskchat/internal/middleware"
	"github.com/Uuenu/taskchat/internal/repository"
	mongorepo "github.com/Uuenu/taskchat/internal/repository/mongo"
	"github.com/Uuenu/taskchat/internal/service"
)

func main() {
	cfg := config.Load()

	// Initialize MongoDB
	log.Println("Connecting to MongoDB...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	mongoClient, err := mongorepo.NewClient(ctx, cfg.MongoDB.URI, cfg.MongoDB.Database)
	cancel()
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	log.Println("Connected to MongoDB successfully")

	// Get repositories
	repos := mongoClient.Repositories()

	// Initialize services
	authService := service.NewAuthService(repos.Users, cfg.JWT.Secret, cfg.JWT.Expiration)

	// Initialize messenger
	messenger := tinode.NewTinode(&cfg.Tinode)
	messenger.SetServiceCredentials(cfg.Tinode.ServiceLogin, cfg.Tinode.ServicePassword)
	messenger.SetReconnectEnabled(true)

	chatService := service.NewChatService(repos.Messages, messenger)

	// Connect to messenger (non-blocking)
	go connectMessenger(messenger, chatService, repos.Users, cfg.Tinode.ServiceLogin, cfg.Tinode.ServicePassword)

	// Initialize handler
	handler := handlers.NewHandler(
		authService,
		chatService,
		messenger,
		repos.Users,
		func() bool {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return mongoClient.Ping(ctx) == nil
		},
	)

	// Setup router
	router := setupRouter(handler, authService)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	if err := messenger.Disconnect(); err != nil {
		log.Printf("Error shutting down messenger: %v", err)
	}

	if err := mongoClient.Close(ctx); err != nil {
		log.Printf("Error closing MongoDB connection: %v", err)
	}

	log.Println("Server exited")
}

// TinodeMessenger extends Messenger with Tinode-specific methods
type TinodeMessenger interface {
	messaging.Messenger
	SetServiceCredentials(email, password string)
	SetReconnectEnabled(enabled bool)
	EnsureGeneralTopic(ctx context.Context) error
	SubscribeToGeneral(ctx context.Context) error
	Login(ctx context.Context, email, password string) (string, error)
}

func connectMessenger(m TinodeMessenger, chatService *service.ChatService, users repository.UserRepository, serviceLogin, servicePassword string) {
	log.Println("Connecting to messenger...")
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := m.Connect(ctx)
		cancel()
		if err != nil {
			log.Printf("Messenger connection attempt %d failed: %v", i+1, err)
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			continue
		}
		log.Println("Connected to messenger successfully")

		// Authenticate service session if credentials provided
		if serviceLogin != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := m.Login(ctx, serviceLogin, servicePassword)
			cancel()
			if err != nil {
				log.Printf("Messenger login failed: %v", err)
			}
		}

		// Ensure and subscribe to general topic
		{
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := m.EnsureGeneralTopic(ctx); err != nil {
				log.Printf("Ensure general topic failed: %v", err)
			}
			cancel()
		}
		{
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := m.SubscribeToGeneral(ctx); err != nil {
				log.Printf("Subscribe to general failed: %v", err)
			}
			cancel()
		}

		// Map messenger author (external ID) to our email and persist messages
		m.OnMessage(func(author, content string, _ map[string]any) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			email := author
			if author != "" && users != nil {
				if u, err := users.FindByExternalID(ctx, m.Provider(), author); err == nil && u != nil && u.Email != "" {
					email = u.Email
				}
			}

			if err := chatService.SaveIncomingMessage(ctx, email, content); err != nil {
				log.Printf("Failed to persist real-time message: %v", err)
			}
		})

		// Reconnect handler: re-auth and re-subscribe
		m.OnReconnect(func() {
			log.Println("Messenger reconnected, performing re-auth and re-subscribe")
			if serviceLogin != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, err := m.Login(ctx, serviceLogin, servicePassword)
				cancel()
				if err != nil {
					log.Printf("Messenger re-login failed: %v", err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := m.SubscribeToGeneral(ctx); err != nil {
				log.Printf("Messenger re-subscribe failed: %v", err)
			}
			cancel()
		})

		return
	}
	log.Println("Warning: Could not connect to messenger - real-time features disabled")
}

func setupRouter(h *handlers.Handler, authService *service.AuthService) *gin.Engine {
	router := gin.Default()

	// CORS middleware
	router.Use(middleware.CORS())

	// Public routes
	router.GET("/health", h.HealthCheck)
	router.POST("/signup", h.Signup)
	router.POST("/login", h.Login)

	// Protected routes
	authorized := router.Group("/")
	authorized.Use(middleware.AuthMiddleware(authService))
	{
		authorized.POST("/message", h.SendMessage)
		authorized.GET("/messages", h.GetMessages)
	}

	return router
}

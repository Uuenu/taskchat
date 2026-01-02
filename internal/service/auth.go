package service

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/Uuenu/taskchat/internal/errors"
	"github.com/Uuenu/taskchat/internal/repository"
)

// AuthService handles authentication business logic
type AuthService struct {
	userRepo  repository.UserRepository
	jwtSecret []byte
	jwtExpiry time.Duration
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// NewAuthService creates a new authentication service
func NewAuthService(userRepo repository.UserRepository, jwtSecret string, jwtExpiry time.Duration) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: []byte(jwtSecret),
		jwtExpiry: jwtExpiry,
	}
}

// Signup registers a new user
func (s *AuthService) Signup(ctx context.Context, email, password string) error {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Create user (repository handles duplicate check)
	_, err = s.userRepo.Create(ctx, email, string(hashedPassword))
	return err
}

// Login authenticates user and returns JWT token
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	// Find user
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if err == errors.ErrUserNotFound {
			return "", errors.ErrInvalidCredentials
		}
		return "", err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.ErrInvalidCredentials
	}

	// Generate JWT
	return s.generateToken(email)
}

// ValidateToken validates JWT and returns the email claim
func (s *AuthService) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.ErrInvalidToken
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		if err == jwt.ErrTokenExpired {
			return "", errors.ErrTokenExpired
		}
		return "", errors.ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return "", errors.ErrInvalidToken
	}

	return claims.Email, nil
}

// generateToken creates a new JWT token
func (s *AuthService) generateToken(email string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   email,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

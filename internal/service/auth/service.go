package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gateway.example/go-gateway/internal/models"
	"gateway.example/go-gateway/internal/repository"
	"gateway.example/go-gateway/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
)

// AuthService 定义认证服务的接口
type AuthService interface {
	Login(ctx context.Context, username, password string) (string, error)
	ValidateToken(ctx context.Context, tokenString string) bool
	ValidateTokenWithClaims(ctx context.Context, tokenString string) (*jwt.RegisteredClaims, error)
	GenerateToken(ctx context.Context, user *models.User) (string, error)
}

// authService 是AuthService接口的具体实现
type authService struct {
	userRepo    repository.UserRepository
	jwtSecret   []byte
	jwtDuration time.Duration
	log         logger.Logger
}

// NewAuthService 创建一个新的认证服务实例
func NewAuthService(
	userRepo repository.UserRepository,
	jwtSecretKey string,
	jwtDurationMinutes int,
	log logger.Logger,
) (AuthService, error) {
	// 输入校验
	if userRepo == nil {
		return nil, errors.New("auth service: user repository cannot be nil")
	}
	if jwtSecretKey == "" {
		return nil, errors.New("auth service: jwt secret key cannot be empty")
	}
	if jwtDurationMinutes <= 0 {
		return nil, errors.New("auth service: jwt duration must be a positive number")
	}

	// 创建实例
	service := &authService{
		userRepo:    userRepo,
		jwtSecret:   []byte(jwtSecretKey),
		jwtDuration: time.Duration(jwtDurationMinutes) * time.Minute,
		log:         log,
	}

	log.Info(context.Background(), "Auth service initialized successfully",
		"jwt_duration_minutes", jwtDurationMinutes,
		"service", "auth")

	return service, nil
}

// Login 验证用户凭证并返回一个JWT
func (s *authService) Login(ctx context.Context, username, password string) (string, error) {
	s.log.Info(ctx, "User login attempt",
		"username", username,
		"service", "auth",
		"action", "login_attempt")

	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		s.log.Warn(ctx, "User not found or repository error",
			"username", username,
			"error", err.Error(),
			"service", "auth",
			"action", "login_failed")
		return "", errors.New("invalid username or password")
	}

	// 注意：在真实项目中，这里应该用 bcrypt.CompareHashAndPassword 来比较哈希后的密码
	if user.Password != password {
		s.log.Warn(ctx, "Invalid password for user",
			"username", username,
			"service", "auth",
			"action", "login_failed")
		return "", errors.New("invalid username or password")
	}

	token, err := s.GenerateToken(ctx, user)
	if err != nil {
		s.log.Error(ctx, "Failed to generate token for user",
			"username", username,
			"error", err.Error(),
			"service", "auth",
			"action", "token_generation_failed")
		return "", err
	}

	s.log.Info(ctx, "User login successful",
		"username", username,
		"user_id", user.ID,
		"service", "auth",
		"action", "login_success")

	return token, nil
}

// ValidateToken 验证JWT令牌的有效性
func (s *authService) ValidateToken(ctx context.Context, tokenString string) bool {
	s.log.Debug(ctx, "Token validation attempt",
		"service", "auth",
		"action", "token_validation_attempt")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			s.log.Warn(ctx, "Unexpected signing method",
				"method", token.Header["alg"],
				"service", "auth",
				"action", "token_validation_failed")
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		s.log.Warn(ctx, "Token parsing failed",
			"error", err.Error(),
			"service", "auth",
			"action", "token_validation_failed")
		return false
	}

	valid := token.Valid
	if valid {
		s.log.Debug(ctx, "Token validation successful",
			"service", "auth",
			"action", "token_validation_success")
	} else {
		s.log.Warn(ctx, "Token validation failed",
			"service", "auth",
			"action", "token_validation_failed")
	}

	return valid
}

// ValidateTokenWithClaims 验证JWT令牌并返回其声明
func (s *authService) ValidateTokenWithClaims(ctx context.Context, tokenString string) (*jwt.RegisteredClaims, error) {
	s.log.Debug(ctx, "Token validation with claims attempt",
		"service", "auth",
		"action", "token_claims_validation_attempt")

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			s.log.Warn(ctx, "Unexpected signing method",
				"method", token.Header["alg"],
				"service", "auth",
				"action", "token_claims_validation_failed")
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			s.log.Warn(ctx, "Token expired",
				"service", "auth",
				"action", "token_expired")
			return nil, errors.New("token expired")
		}
		s.log.Warn(ctx, "Token parsing with claims failed",
			"error", err.Error(),
			"service", "auth",
			"action", "token_claims_validation_failed")
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		s.log.Warn(ctx, "Token is not valid",
			"service", "auth",
			"action", "token_claims_validation_failed")
		return nil, errors.New("token is not valid")
	}

	s.log.Debug(ctx, "Token validation with claims successful",
		"subject", claims.Subject,
		"issuer", claims.Issuer,
		"service", "auth",
		"action", "token_claims_validation_success")

	return claims, nil
}

// GenerateToken 为用户生成JWT令牌
func (s *authService) GenerateToken(ctx context.Context, user *models.User) (string, error) {
	s.log.Debug(ctx, "Token generation attempt",
		"user_id", user.ID,
		"service", "auth",
		"action", "token_generation_attempt")

	claims := &jwt.RegisteredClaims{
		Issuer:    "auth-service",
		Subject:   user.ID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtDuration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		s.log.Error(ctx, "Failed to sign token",
			"user_id", user.ID,
			"error", err.Error(),
			"service", "auth",
			"action", "token_generation_failed")
		return "", err
	}

	s.log.Debug(ctx, "Token generation successful",
		"user_id", user.ID,
		"expires_at", claims.ExpiresAt,
		"service", "auth",
		"action", "token_generation_success")

	return tokenString, nil
}

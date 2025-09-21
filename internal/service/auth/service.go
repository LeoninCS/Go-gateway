package auth

import (
	"errors"
	"fmt"
	"time"

	"gateway.example/go-gateway/internal/models"
	"gateway.example/go-gateway/internal/repository"
	"github.com/golang-jwt/jwt/v5"
)

// AuthService 定义认证服务的接口
type AuthService interface {
	Login(username, password string) (string, error)
	ValidateToken(tokenString string) bool
	ValidateTokenWithClaims(tokenString string) (*jwt.RegisteredClaims, error)
	GenerateToken(user *models.User) (string, error)
}

// authService 是AuthService接口的具体实现
type authService struct {
	userRepo    repository.UserRepository
	jwtSecret   []byte
	jwtDuration time.Duration
}

// NewAuthService 创建一个新的认证服务实例
func NewAuthService(
	userRepo repository.UserRepository,
	jwtSecretKey string,
	jwtDurationMinutes int,
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
	}

	return service, nil
}

// Login 验证用户凭证并返回一个JWT
func (s *authService) Login(username, password string) (string, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return "", errors.New("invalid username or password")
	}

	// 注意：在真实项目中，这里应该用 bcrypt.CompareHashAndPassword 来比较哈希后的密码
	if user.Password != password {
		return "", errors.New("invalid username or password")
	}

	token, err := s.GenerateToken(user)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidateToken 验证JWT令牌的有效性
func (s *authService) ValidateToken(tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return false
	}

	return token.Valid
}

// ValidateTokenWithClaims 验证JWT令牌并返回其声明
func (s *authService) ValidateTokenWithClaims(tokenString string) (*jwt.RegisteredClaims, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token expired")
		}
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("token is not valid")
	}

	return claims, nil
}

// GenerateToken 为用户生成JWT令牌
func (s *authService) GenerateToken(user *models.User) (string, error) {
	claims := &jwt.RegisteredClaims{
		Issuer:    "auth-service",
		Subject:   user.ID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtDuration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

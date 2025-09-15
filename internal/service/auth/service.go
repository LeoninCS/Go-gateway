package auth

import (
	"errors"
	"fmt"
	"time"

	"gateway.example/go-gateway/internal/models"
	"gateway.example/go-gateway/internal/repository" // 同样，替换成你的模块名

	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	userRepo    repository.UserRepository
	jwtSecret   []byte
	jwtDuration time.Duration
}

func NewAuthService(userRepo repository.UserRepository, secret string, durationMinutes int) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		jwtSecret:   []byte(secret),
		jwtDuration: time.Duration(durationMinutes) * time.Minute,
	}
}

// Login 验证用户凭证并返回一个JWT
func (s *AuthService) Login(username, password string) (string, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return "", errors.New("invalid username or password")
	}

	if user.Password != password { // 在真实项目中，这里应该用 bcrypt.CompareHashAndPassword
		return "", errors.New("invalid username or password")
	}
	token, err := s.GenerateToken(user)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *AuthService) ValidateToken(tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil // 使用 AuthService 实例持有的 secretKey
	})
	if err != nil {
		// 可以选择在这里记录错误，以便调试
		// log.Printf("JWT token validation failed: %v", err)
		return false
	}
	return token.Valid
}

func (s *AuthService) ValidateTokenWithClaims(tokenString string) (*jwt.RegisteredClaims, error) {
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

// GenerateToken 生成 JWT token (示例)
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	claims := &jwt.RegisteredClaims{
		Issuer:    "auth-service",
		Subject:   user.ID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtDuration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

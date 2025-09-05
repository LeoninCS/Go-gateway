package auth

import (
	"errors"
	"time"

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

	claims := &jwt.RegisteredClaims{
		Issuer:    "auth-service",
		Subject:   user.ID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtDuration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

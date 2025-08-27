package auth

import (
	"errors"

	"gateway.example/go-gateway/internal/dao"
	"gateway.example/go-gateway/internal/models" // ★ 导入 models 包

	"golang.org/x/crypto/bcrypt"
)

// ★ 定义业务逻辑层相关的错误
var (
	Error = errors.New("invalid username or password")
)

// AuthService 封装了所有用户认证相关的业务逻辑
type AuthService struct {
	userDAO dao.UserDAO
}

// NewAuthService 是 AuthService 的构造函数
func NewAuthService(userDAO dao.UserDAO) *AuthService {
	return &AuthService{
		userDAO: userDAO,
	}
}

// RegisterUser 处理用户注册的业务逻辑
func (s *AuthService) RegisterUser(username, password, phone string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// ★ 使用 models.User
	newUser := models.User{
		Username:     username,
		Phone:        phone,
		PasswordHash: string(hashedPassword),
	}

	return s.userDAO.Create(&newUser)
}

// AuthenticateUser 处理用户登录验证的业务逻辑
func (s *AuthService) AuthenticateUser(username, password string) (*models.User, error) { // ★ 返回 models.User
	user, err := s.userDAO.FindByUsername(username)
	if err != nil {
		// ★ 从 dao 包检查错误
		if errors.Is(err, dao.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

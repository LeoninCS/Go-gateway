package auth

import (
	"context"
	"fmt"
	"log"
	"time"

	"gateway.example/go-gateway/internal/cache"
	"gateway.example/go-gateway/internal/models"
	"gateway.example/go-gateway/internal/repository"
	"gateway.example/go-gateway/pkg/jwt"
	"gateway.example/go-gateway/pkg/util"
)

type AuthService struct {
	userRepo    repository.UserRepository // 使用新的接口类型
	jwtSecret   string
	jwtDuration time.Duration
	cache       cache.Cache
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string, jwtDurationMinutes int) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		jwtSecret:   jwtSecret,
		jwtDuration: time.Duration(jwtDurationMinutes) * time.Minute,
	}
}

func (s *AuthService) Register(username, password, phone string) (*models.User, error) {
	// 检查用户是否已存在
	_, err := s.userRepo.FindByUsername(username)
	if err == nil {
		return nil, ErrUserExists
	}

	// 如果是"未找到"错误，可以继续注册流程
	if err != nil && err != repository.ErrNotFound {
		return nil, err // 其他错误（如数据库连接问题）
	}

	if phone == "" {
		return nil, ErrPhoneRequired
	}

	// 哈希密码
	hashedPassword, err := util.Encrypt(password)
	if err != nil {
		return nil, err
	}

	hashedPhone, err := util.Encrypt(phone)
	if err != nil {
		return nil, err
	}

	// 创建用户对象
	user := &models.User{
		Username:     username,
		Phone:        string(hashedPhone),
		PasswordHash: string(hashedPassword),
	}

	// 保存到数据库
	if err := s.userRepo.Create(user); err != nil {
		if err == repository.ErrDuplicate {
			return nil, ErrUserExists
		}
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(username, password string) (string, error) {
	// 查找用户
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		if err == repository.ErrNotFound {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	// 验证密码
	err = util.Compare(user.PasswordHash, password)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	// 生成 JWT Token
	token, err := jwt.GenerateToken(int64(user.ID), user.Username, []byte(s.jwtSecret), s.jwtDuration)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) ChangePassword(userName string, oldPassword, newPassword string) error {
	// 查找用户
	user, err := s.userRepo.FindByUsername(userName)
	if err != nil {
		if err == repository.ErrNotFound {
			return ErrUserNotFound
		}
		return err
	}
	// 验证旧密码
	err = util.Compare(user.PasswordHash, oldPassword)
	if err != nil {
		return ErrInvalidCredentials
	}

	// 哈希新密码
	hashedPassword, err := util.Encrypt(newPassword)
	if err != nil {
		return err
	}

	// 更新密码
	user.PasswordHash = string(hashedPassword)
	return s.userRepo.Update(user)
}

func (s *AuthService) ResetPassword(username, phone, verificationCode, newPassword string) error {
	// 查找用户
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		if err == repository.ErrNotFound {
			return ErrUserNotFound
		}
		return err
	}

	// 验证手机号是否匹配
	if user.Phone != phone {
		return ErrPhoneNotMatch
	}
	// 从缓存或数据库中获取存储的验证码
	storedCode, err := s.cache.Get("pwd_reset:" + phone)
	if err != nil {
		return fmt.Errorf("获取验证码失败: %w", err)
	}
	if storedCode == "" || storedCode != verificationCode {
		return ErrInvalidVerificationCode
	}

	// 哈希新密码
	hashedPassword, err := util.Encrypt(newPassword)
	if err != nil {
		return err
	}
	// 更新密码
	user.PasswordHash = string(hashedPassword)
	return s.userRepo.Update(user)
}

func (s *AuthService) Unregister(username, password string) error {
	// 查找用户
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		if err == repository.ErrNotFound {
			return ErrUserNotFound
		}
		return err
	}

	// 验证密码
	err = util.Compare(user.PasswordHash, password)
	if err != nil {
		return ErrInvalidCredentials
	}

	// 删除用户
	return s.userRepo.Delete(uint(user.ID))
}

func (s *AuthService) Logout(tokenString string) error {
	// 将 token 加入黑名单，存储在缓存中，过期时间与 token 相同
	claims, err := jwt.ValidateToken(tokenString, []byte(s.jwtSecret))
	if err != nil {
		return err
	}

	expiration := time.Until(claims.ExpiresAt.Time)
	err = s.cache.Set("blacklist:"+tokenString, "blacklisted", expiration)
	if err != nil {
		return fmt.Errorf("将 token 加入黑名单失败: %w", err)
	}
	return nil
}

func (s *AuthService) SendVerificationCode(username, phone string) (string, error) {
	// 查找用户
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		if err == repository.ErrNotFound {
			return "", ErrUserNotFound
		}
		return "", err
	}

	// 验证手机号是否匹配
	if user.Phone != phone {
		return "", ErrPhoneNotMatch
	}

	// 生成验证码（6位数字）
	verificationCode := util.GenerateVerificationCode(6)

	// 模拟发送验证码到控制台
	log.Printf("[模拟短信] 向手机号 %s 发送验证码: %s", phone, verificationCode)
	log.Printf("请在程序控制台查看验证码，无需真实短信发送")

	// 存储验证码到缓存或数据库（带过期时间）
	err = s.cache.Set("pwd_reset:"+phone, verificationCode, 10*time.Minute)
	if err != nil {
		return "", fmt.Errorf("保存验证码失败: %w", err)
	}

	return "验证码已发送（模拟模式）", nil
}

// GetClaimsFromContext 从请求的 context 中安全地提取 claims
func GetClaimsFromContext(ctx context.Context) (*jwt.Claims, bool) {
	claims, ok := ctx.Value(key).(*jwt.Claims)
	return claims, ok
}

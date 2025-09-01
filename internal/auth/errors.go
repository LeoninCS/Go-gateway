// 业务逻辑层错误定义
package auth

import "errors"

var (
	ErrUserExists              = errors.New("user already exists")
	ErrUserNotFound            = errors.New("user not found")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrPhoneRequired           = errors.New("phone number is required")
	ErrPhoneNotMatch           = errors.New("phone number does not match")
	ErrInvalidVerificationCode = errors.New("invalid verification code")
)

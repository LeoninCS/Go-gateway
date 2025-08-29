package repository

import "errors"

// 定义 Repository 层的通用错误，便于 Service 层进行特定错误的判断。
var (
	ErrNotFound  = errors.New("record not found")
	ErrDuplicate = errors.New("duplicate record")
	// ... 可以按需添加更多通用错误
)

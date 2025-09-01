package util

import (
	"math/rand"
	"time"
)

// 生成指定位数的随机验证码
func GenerateVerificationCode(length int) string {
	rand.Seed(time.Now().UnixNano())
	digits := make([]byte, length)
	for i := range digits {
		digits[i] = byte(rand.Intn(10) + 48) // 0-9的ASCII码
	}
	return string(digits)
}

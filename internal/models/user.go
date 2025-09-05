// file: internal/models/user.go
package models

// 为了简单起见，我们先用一个简单的 User 结构体
// 之后你可以添加 GORM 标签来映射数据库表
type User struct {
	ID       string
	Username string
	Password string // 在真实项目中，这里应该是密码的哈希值
}

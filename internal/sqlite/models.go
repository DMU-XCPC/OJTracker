package sqlite

import "encoding/json"

// User 的 ID 在改名和更换绑定后保持稳定。
type User struct {
	ID   string `gorm:"primaryKey" json:"id"`
	Name string `json:"name"`
}

// Account 与既有 accounts 表对应。版本字段只用于并发写入校验。
type Account struct {
	UserID   string          `gorm:"primaryKey" json:"-"`
	Platform string          `gorm:"primaryKey" json:"platform"`
	Handle   string          `json:"handle"`
	Revision string          `json:"-"`
	Profile  json.RawMessage `gorm:"serializer:json" json:"profile"`
	Updated  *string         `gorm:"column:updated_at" json:"updated_at"`
	Error    *string         `gorm:"column:last_error" json:"last_error"`
}

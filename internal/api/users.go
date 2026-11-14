package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"ojtracker/internal/sqlite"
)

// User 的 ID 在改名和更换平台绑定后保持稳定。
type User = sqlite.User

func (s *Handler) saveUser(w http.ResponseWriter, r *http.Request) {
	// 从请求体读取创建或改名需要的名称。
	var in struct{ Name string }
	if !body(w, r, &in) {
		return
	}
	// 去除首尾空白，名称须非空且不超过 256 字节。
	u := User{ID: r.PathValue("id"), Name: strings.TrimSpace(in.Name)}
	if u.Name == "" || len(u.Name) > 256 {
		fail(w, 400, "invalid name")
		return
	}
	status := 200
	var e error
	// 创建时生成独立 ID，以 201 返回新用户。
	if r.Method == "POST" {
		u.ID = id()
		e = s.db.WithContext(r.Context()).Create(&u).Error
		status = 201
	} else {
		// 改名时保留 ID，通过更新行数 识别用户不存在的情况。
		e = requireRow(s.db.WithContext(r.Context()).Model(&u).Update("name", u.Name))
	}
	// 数据库成功写入后才返回用户资料。
	if e != nil {
		dbError(w, e)
		return
	}
	respond(w, status, u)
}

func (s *Handler) list(w http.ResponseWriter, r *http.Request) {
	// 按 ID 排序，使列表响应顺序稳定。
	var out []User
	if err := s.db.WithContext(r.Context()).Order("id").Find(&out).Error; err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, out)
}

func (s *Handler) detail(w http.ResponseWriter, r *http.Request) {
	// 使用同一读事务，使用户资料和平台绑定对应同一时刻。
	var u User
	var accounts []sqlite.Account
	err := s.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", r.PathValue("id")).Take(&u).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", u.ID).Order("platform").Find(&accounts).Error
	})
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 200, map[string]any{"id": u.ID, "name": u.Name, "accounts": accounts})
}

func userExists(db *gorm.DB, uid string) error {
	return db.Select("id").Where("id = ?", uid).Take(&User{}).Error
}

// requireRow 区分数据库故障与修改目标不存在。
func requireRow(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	// 删除指定用户，依靠外键级联清理绑定和快照。
	uid := r.PathValue("id")
	err := requireRow(s.db.WithContext(r.Context()).Where("id = ?", uid).Delete(&User{}))
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 204, nil)
}

func id() string {
	// 用 128 位随机数生成用户 ID 或绑定版本。
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	// 十六进制编码后，标识长度固定为 32 个字符。
	return hex.EncodeToString(b[:])
}

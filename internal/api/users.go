package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// User 的 ID 在改名和更换平台绑定后保持稳定。
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

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
		_, e = s.db.Exec("INSERT INTO users VALUES(?,?)", u.ID, u.Name)
		status = 201
	} else {
		// 改名时保留 ID，通过 RETURNING 识别用户不存在的情况。
		e = s.db.QueryRow("UPDATE users SET name=? WHERE id=? RETURNING id", u.Name, u.ID).Scan(&u.ID)
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
	rows, e := s.db.Query("SELECT id,name FROM users ORDER BY id")
	if e != nil {
		dbError(w, e)
		return
	}
	defer rows.Close()
	// 逐行读取基础资料，空列表也编码为 JSON 数组。
	out := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Name); e != nil {
			dbError(w, e)
			return
		}
		out = append(out, u)
	}
	// 检查迭代错误，避免将部分结果当作完整列表。
	if e = rows.Err(); e != nil {
		dbError(w, e)
		return
	}
	respond(w, 200, out)
}

func (s *Handler) detail(w http.ResponseWriter, r *http.Request) {
	// 使用同一读事务，使用户资料和平台绑定对应同一时刻。
	tx, e := s.db.BeginTx(r.Context(), nil)
	if e != nil {
		dbError(w, e)
		return
	}
	defer tx.Rollback()
	// 先确认用户存在，再查询其平台绑定。
	u := User{ID: r.PathValue("id")}
	e = tx.QueryRow("SELECT name FROM users WHERE id=?", u.ID).Scan(&u.Name)
	if e != nil {
		dbError(w, e)
		return
	}
	// 按平台名排序，一并取出快照、更新时间和错误状态。
	const query = `
 SELECT platform, handle, profile, updated_at, last_error
 FROM accounts
 WHERE user_id = ?
 ORDER BY platform`
	rows, e := tx.Query(query, u.ID)
	if e != nil {
		dbError(w, e)
		return
	}
	defer rows.Close()
	// 组装各平台绑定，尚未绑定时返回空数组。
	accounts := []Account{}
	for rows.Next() {
		var a Account
		var profile *string
		if e = rows.Scan(&a.Platform, &a.Handle, &profile, &a.Updated, &a.Error); e != nil {
			dbError(w, e)
			return
		}
		// 将已有快照嵌入响应，保留数据库中的空值状态。
		if profile != nil {
			a.Profile = json.RawMessage(*profile)
		}
		accounts = append(accounts, a)
	}
	// 全部绑定读取成功后再返回聚合结果。
	if e = rows.Err(); e != nil {
		dbError(w, e)
		return
	}
	respond(w, 200, map[string]any{"id": u.ID, "name": u.Name, "accounts": accounts})
}

func (s *Handler) exists(uid string) error {
	// 只查询存在性，空结果由调用方映射为相应错误。
	var n int
	return s.db.QueryRow("SELECT 1 FROM users WHERE id=?", uid).Scan(&n)
}

func (s *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	// 删除指定用户，依靠外键级联清理绑定和快照。
	uid := r.PathValue("id")
	err := s.db.QueryRow("DELETE FROM users WHERE id=? RETURNING id", uid).Scan(&uid)
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

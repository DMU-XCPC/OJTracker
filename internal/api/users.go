package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"ojtracker/internal/sqlite"
)

// User 的 ID 在改名和更换平台绑定后保持稳定。
type User = sqlite.User

// 两个写入接口共用名称解析和校验。
func readUserName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var in struct{ Name string }
	if !body(w, r, &in) {
		return "", false
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 256 {
		fail(w, http.StatusBadRequest, "invalid name")
		return "", false
	}
	return name, true
}

func (s *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	name, ok := readUserName(w, r)
	if !ok {
		return
	}

	user := User{ID: id(), Name: name}
	db := s.db.WithContext(r.Context())
	if err := sqlite.CreateUser(db, user); err != nil {
		dbError(w, err)
		return
	}
	respond(w, http.StatusCreated, user)
}

func (s *Handler) renameUser(w http.ResponseWriter, r *http.Request) {
	name, ok := readUserName(w, r)
	if !ok {
		return
	}
	user := User{ID: r.PathValue("id"), Name: name}
	db := s.db.WithContext(r.Context())
	if err := sqlite.RenameUser(db, user); err != nil {
		dbError(w, err)
		return
	}
	respond(w, http.StatusOK, user)
}

func (s *Handler) list(w http.ResponseWriter, r *http.Request) {
	users, err := sqlite.ListUsers(s.db.WithContext(r.Context()))
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, http.StatusOK, users)
}

func (s *Handler) detail(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	user, accounts, err := sqlite.UserDetail(db, r.PathValue("id"))
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"id": user.ID, "name": user.Name, "accounts": accounts})
}

func (s *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	if err := sqlite.DeleteUser(db, r.PathValue("id")); err != nil {
		dbError(w, err)
		return
	}
	respond(w, http.StatusNoContent, nil)
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

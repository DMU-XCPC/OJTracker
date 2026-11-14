package api

import (
	"net/http"

	"gorm.io/gorm"

	"ojtracker/internal/refresh"
)

// Handler 通过数据库提供本地查询，通过共享更新器执行主动刷新。
type Handler struct {
	db      *gorm.DB
	updater *refresh.Updater
}

// NewHandler 装配 API 路由；更新器同时供后台定时任务使用。
func NewHandler(db *gorm.DB, updater *refresh.Updater) http.Handler {
	s := &Handler{db: db, updater: updater}
	// 注册用户管理与详情路由。
	m := http.NewServeMux()
	m.HandleFunc("POST /users", s.saveUser)
	m.HandleFunc("PUT /users/{id}", s.saveUser)
	m.HandleFunc("GET /users", s.list)
	m.HandleFunc("GET /users/{id}", s.detail)
	m.HandleFunc("DELETE /users/{id}", s.deleteUser)

	// 注册平台绑定的查询、写入和解绑路由。
	m.HandleFunc("GET /users/{id}/accounts/{platform}", s.account)
	m.HandleFunc("PUT /users/{id}/accounts/{platform}", s.bind)
	m.HandleFunc("DELETE /users/{id}/accounts/{platform}", s.unbind)
	// 主动刷新作用于指定用户的全部绑定。
	m.HandleFunc("POST /users/{id}/refresh", s.refreshUser)
	return m
}

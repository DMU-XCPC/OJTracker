package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"gorm.io/gorm"
)

func respond(w http.ResponseWriter, status int, data any) {
	// 在写入响应体前设置状态码和 JSON 类型。
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// 空响应跳过编码，确保 204 没有正文。
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func fail(w http.ResponseWriter, status int, msg string) {
	// 统一使用 error 字段携带错误信息。
	respond(w, status, map[string]string{"error": msg})
}

func dbError(w http.ResponseWriter, e error) {
	// 将未匹配记录映射为资源不存在。
	if errors.Is(e, gorm.ErrRecordNotFound) {
		fail(w, 404, "resource not found")
		return
	}
	// 数据库故障详情写入日志，客户端收到通用错误。
	log.Print(e)
	fail(w, 500, "database error")
}

func body(w http.ResponseWriter, r *http.Request, v any) bool {
	// 限制请求体为 8 KiB，并拒绝未定义字段。
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	// 第二次解码应遇到 EOF，以拒绝一个请求体中出现多个 JSON 值。
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "invalid JSON body")
		return false
	}
	return true
}

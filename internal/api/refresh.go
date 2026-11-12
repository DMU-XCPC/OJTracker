package api

import "net/http"

func (s *Handler) refreshUser(w http.ResponseWriter, r *http.Request) {
	// 先检查用户存在性，区分未知用户和没有绑定的用户。
	uid := r.PathValue("id")
	if e := s.exists(uid); e != nil {
		dbError(w, e)
		return
	}
	// 使用请求上下文同步刷新所有绑定。
	issues, e := s.updater.Refresh(r.Context(), uid)
	if e != nil {
		dbError(w, e)
		return
	}
	// 部分平台失败时返回 502，并附上每个失败项。
	status := 200
	if len(issues) > 0 {
		status = 502
	}
	respond(w, status, map[string]any{"errors": issues})
}

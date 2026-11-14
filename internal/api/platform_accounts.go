package api

import (
	"errors"
	"net/http"
	"regexp"

	"gorm.io/gorm"

	"ojtracker/internal/sqlite"
)

// Account 包含绑定信息和最近一次成功抓取的平台快照。
type Account = sqlite.Account

var handlePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// supported 校验服务已接入的平台标识。
func supported(p string) bool { return p == "codeforces" || p == "atcoder" }

func (s *Handler) account(w http.ResponseWriter, r *http.Request) {
	// 在查询数据库前检查平台名。
	p := r.PathValue("platform")
	if !supported(p) {
		fail(w, 400, "unknown platform")
		return
	}
	db := s.db.WithContext(r.Context())
	account, err := sqlite.FindAccount(db, r.PathValue("id"), p)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fail(w, 404, "user_not_found")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if account.Handle == "" {
		fail(w, 404, "platform_not_bound")
		return
	}
	respond(w, 200, account)
}

func (s *Handler) bind(w http.ResponseWriter, r *http.Request) {
	// 只允许写入已接入平台的绑定。
	p, uid := r.PathValue("platform"), r.PathValue("id")
	if !supported(p) {
		fail(w, 400, "unknown platform")
		return
	}
	// 解析账号名，校验长度、字符范围并排除路径点段。
	var in struct{ Handle string }
	if !body(w, r, &in) {
		return
	}
	if !handlePattern.MatchString(in.Handle) || in.Handle == "." || in.Handle == ".." {
		fail(w, 400, "invalid handle")
		return
	}
	// 相同账号重复绑定时保留快照。更换账号时同时清空快照并更新绑定版本。
	binding := Account{UserID: uid, Platform: p, Handle: in.Handle, Revision: id()}
	db := s.db.WithContext(r.Context())
	err := sqlite.BindAccount(db, binding)
	if err != nil {
		dbError(w, err)
		return
	}
	// 对新建绑定和重复绑定均返回空响应。
	respond(w, 204, nil)
}

func (s *Handler) unbind(w http.ResponseWriter, r *http.Request) {
	// 用用户 ID 和平台共同限定删除范围，未匹配时返回 404。
	uid := r.PathValue("id")
	db := s.db.WithContext(r.Context())
	err := sqlite.UnbindAccount(db, uid, r.PathValue("platform"))
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 204, nil)
}

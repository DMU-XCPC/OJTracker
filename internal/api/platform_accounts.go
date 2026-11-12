package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"ojtracker/platform/nowcoder"
)

// Account 包含绑定信息和最近一次成功抓取的平台快照。
type Account struct {
	Platform string          `json:"platform"`
	Handle   string          `json:"handle"`
	Profile  json.RawMessage `json:"profile"`
	Updated  *string         `json:"updated_at"`
	Error    *string         `json:"last_error"`
}

var handlePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// supported 校验服务已接入的平台标识。
func supported(p string) bool { return p == "codeforces" || p == "atcoder" || p == "nowcoder" }

func (s *Handler) account(w http.ResponseWriter, r *http.Request) {
	// 在查询数据库前检查平台名。
	p := r.PathValue("platform")
	if !supported(p) {
		fail(w, 400, "unknown platform")
		return
	}
	var handle, profile *string
	a := Account{Platform: p}
	// 左连接保留已注册但未绑定的用户行，便于区分两种 404。
	const query = `
 SELECT a.handle, a.profile, a.updated_at, a.last_error
 FROM users AS u
 LEFT JOIN accounts AS a ON a.user_id = u.id AND a.platform = ?
 WHERE u.id = ?`
	err := s.db.QueryRowContext(r.Context(), query, p, r.PathValue("id")).Scan(&handle, &profile, &a.Updated, &a.Error)
	// 查询无记录表示用户不存在，其他数据库错误单独处理。
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "user_not_found")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	// 用户行存在而账号名为空，表示该平台尚未绑定。
	if handle == nil {
		fail(w, 404, "platform_not_bound")
		return
	}
	// 将已有快照直接作为 JSON 返回，首次更新前保留 null。
	a.Handle = *handle
	if profile != nil {
		a.Profile = json.RawMessage(*profile)
	}
	respond(w, 200, a)
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
	// 牛客竞赛主页使用数字 UID 作为绑定标识。
	if p == "nowcoder" && !nowcoder.ValidUID(in.Handle) {
		fail(w, 400, "invalid nowcoder UID")
		return
	}
	if !handlePattern.MatchString(in.Handle) || in.Handle == "." || in.Handle == ".." {
		fail(w, 400, "invalid handle")
		return
	}
	// 相同账号重复绑定时保留快照。更换账号时同时清空快照并更新绑定版本。
	const upsert = `
 INSERT INTO accounts (user_id, platform, handle, revision)
 SELECT id, ?, ?, ? FROM users WHERE id = ?
 ON CONFLICT (user_id, platform) DO UPDATE SET
     handle = excluded.handle,
     revision = excluded.revision,
     profile = NULL,
     updated_at = NULL,
     last_error = NULL
 WHERE accounts.handle <> excluded.handle`
	// 为新绑定准备独立版本，并仅对已有用户执行写入。
	result, e := s.db.Exec(upsert, p, in.Handle, id(), uid)
	if e != nil {
		dbError(w, e)
		return
	}
	// 零行变更可能表示重复绑定或用户不存在，需要确认用户记录。
	if n, _ := result.RowsAffected(); n == 0 {
		if e = s.exists(uid); e != nil {
			dbError(w, e)
			return
		}
	}
	// 对新建绑定和重复绑定均返回空响应。
	respond(w, 204, nil)
}

func (s *Handler) unbind(w http.ResponseWriter, r *http.Request) {
	// 用用户 ID 和平台共同限定删除范围，未匹配时返回 404。
	uid := r.PathValue("id")
	err := s.db.QueryRow("DELETE FROM accounts WHERE user_id=? AND platform=? RETURNING user_id", uid, r.PathValue("platform")).Scan(&uid)
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 204, nil)
}

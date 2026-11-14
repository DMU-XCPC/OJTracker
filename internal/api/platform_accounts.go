package api

import (
	"errors"
	"net/http"
	"regexp"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"ojtracker/internal/sqlite"
	"ojtracker/platform/nowcoder"
)

// Account 包含绑定信息和最近一次成功抓取的平台快照。
type Account = sqlite.Account

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
	// 从用户表左连接绑定，一次查询区分未知用户和未绑定平台。
	var a Account
	err := s.db.WithContext(r.Context()).Model(&User{}).Select("accounts.*").
		Joins("LEFT JOIN accounts ON accounts.user_id = users.id AND accounts.platform = ?", p).
		Where("users.id = ?", r.PathValue("id")).Take(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fail(w, 404, "user_not_found")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if a.Handle == "" {
		fail(w, 404, "platform_not_bound")
		return
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
	err := s.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := userExists(tx, uid); err != nil {
			return err
		}
		binding := Account{UserID: uid, Platform: p, Handle: in.Handle, Revision: id()}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "platform"}},
			DoUpdates: clause.AssignmentColumns([]string{"handle", "revision", "profile", "updated_at", "last_error"}),
			Where:     clause.Where{Exprs: []clause.Expression{clause.Neq{Column: clause.Column{Table: "accounts", Name: "handle"}, Value: clause.Column{Table: "excluded", Name: "handle"}}}},
		}).Create(&binding).Error
	})
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
	err := requireRow(s.db.WithContext(r.Context()).Where("user_id = ? AND platform = ?", uid, r.PathValue("platform")).Delete(&Account{}))
	if err != nil {
		dbError(w, err)
		return
	}
	respond(w, 204, nil)
}

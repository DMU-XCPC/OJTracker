package sqlite

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FindAccount 用左连接区分用户不存在（返回错误）和平台未绑定（账号名为空）。
func FindAccount(db *gorm.DB, userID, platform string) (Account, error) {
	var account Account
	err := db.Model(&User{}).
		Select("accounts.*").
		Joins("LEFT JOIN accounts ON accounts.user_id = users.id AND accounts.platform = ?", platform).
		Where("users.id = ?", userID).
		Take(&account).Error
	return account, err
}

func BindAccount(db *gorm.DB, binding Account) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := RequireUser(tx, binding.UserID); err != nil {
			return err
		}
		// 仅在账号名变化时替换版本和快照，重复绑定同一账号保留已有资料。
		replaceChangedHandle := clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "platform"}},
			DoUpdates: clause.AssignmentColumns([]string{"handle", "revision", "profile", "updated_at", "last_error"}),
			Where:     clause.Where{Exprs: []clause.Expression{gorm.Expr("accounts.handle <> excluded.handle")}},
		}
		return tx.Clauses(replaceChangedHandle).Create(&binding).Error
	})
}

func UnbindAccount(db *gorm.DB, userID, platform string) error {
	result := db.Where("user_id = ? AND platform = ?", userID, platform).Delete(&Account{})
	return requireRow(result)
}

// ListRefreshBindings 按固定顺序读取本轮需要更新的绑定及版本。
func ListRefreshBindings(db *gorm.DB, userID string) ([]Account, error) {
	query := db.Select("user_id", "platform", "handle", "revision").
		Where("platform IN ?", []string{"codeforces", "atcoder"})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	var bindings []Account
	err := query.Order("user_id, platform").Find(&bindings).Error
	return bindings, err
}

// UpdateBindingSnapshot 仅写回仍匹配抓取时版本的绑定。
func UpdateBindingSnapshot(db *gorm.DB, binding Account, changes map[string]any) (bool, error) {
	result := db.Model(&Account{}).
		Where("user_id = ? AND platform = ? AND revision = ?", binding.UserID, binding.Platform, binding.Revision).
		Updates(changes)
	return result.RowsAffected > 0, result.Error
}

package sqlite

import "gorm.io/gorm"

func CreateUser(db *gorm.DB, user User) error {
	return db.Create(&user).Error
}

func RenameUser(db *gorm.DB, user User) error {
	result := db.Model(&User{}).Where("id = ?", user.ID).Update("name", user.Name)
	return requireRow(result)
}

func DeleteUser(db *gorm.DB, userID string) error {
	result := db.Where("id = ?", userID).Delete(&User{})
	return requireRow(result)
}

func ListUsers(db *gorm.DB) ([]User, error) {
	var users []User
	err := db.Order("id").Find(&users).Error
	return users, err
}

func RequireUser(db *gorm.DB, userID string) error {
	return db.Select("id").Where("id = ?", userID).Take(&User{}).Error
}

// UserDetail 在同一事务中读取用户与绑定，保证响应中的记录一致。
func UserDetail(db *gorm.DB, userID string) (User, []Account, error) {
	var user User
	var accounts []Account
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", userID).Take(&user).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Order("platform").Find(&accounts).Error
	})
	return user, accounts, err
}

// requireRow 将未命中记录的更新或删除映射为资源不存在。
func requireRow(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

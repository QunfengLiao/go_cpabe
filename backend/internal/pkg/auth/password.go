package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword 使用 bcrypt 生成密码摘要，返回值可安全保存到数据库。
func HashPassword(password string) (string, error) {
	// bcrypt 自带盐和成本参数，适合保存用户密码摘要；数据库中绝不能落明文密码。
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword 使用 bcrypt 校验明文密码和数据库中的密码摘要是否匹配。
func CheckPassword(password, hash string) bool {
	// 统一走 bcrypt 比较，避免把哈希细节散落到业务层或误用普通字符串比较。
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

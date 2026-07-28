package shared

import "golang.org/x/crypto/bcrypt"

const MinPasswordLength = 8

func PasswordIsValid(passwordHash *string, password string) bool {
	if passwordHash == nil || *passwordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte(password)) == nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

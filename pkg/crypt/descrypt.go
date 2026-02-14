// Package crypt wraps DES crypt(3) for TinyMUSH password verification.
// TinyMUSH stores passwords as crypt(password, "XX").
package crypt

import (
	descrypt "github.com/digitive/crypt"
)

// Crypt performs traditional Unix DES crypt(3).
func Crypt(password, salt string) string {
	result, err := descrypt.Crypt(password, salt)
	if err != nil {
		return ""
	}
	return result
}

// CheckPassword verifies a password against a DES-encrypted hash.
func CheckPassword(password, storedHash string) bool {
	if len(storedHash) < 2 {
		return false
	}
	salt := storedHash[:2]
	computed := Crypt(password, salt)
	return computed != "" && computed == storedHash
}

package crypt

import (
	"testing"
)

func TestCryptConsistency(t *testing.T) {
	// Verify that crypt is consistent: hash then verify
	passwords := []string{"test", "password", "hello", "mypass123", ""}
	salt := "XX"

	for _, pw := range passwords {
		hash := Crypt(pw, salt)
		t.Logf("crypt(%q, %q) = %q", pw, salt, hash)
		if len(hash) != 13 && pw != "" {
			t.Errorf("Expected 13-char hash for %q, got %d chars: %q", pw, len(hash), hash)
		}
		if hash[:2] != salt && pw != "" {
			t.Errorf("Hash should start with salt %q, got %q", salt, hash[:2])
		}
	}
}

func TestCheckPassword(t *testing.T) {
	hash := Crypt("testpass", "XX")
	t.Logf("crypt(testpass, XX) = %q", hash)

	if !CheckPassword("testpass", hash) {
		t.Error("CheckPassword should return true for correct password")
	}
	if CheckPassword("wrongpass", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
	if CheckPassword("", hash) {
		t.Error("CheckPassword should return false for empty password")
	}
}

func TestCheckPasswordDifferentSalts(t *testing.T) {
	salts := []string{"XX", "ab", "Ax", "..", "//"}
	pw := "mushpassword"

	for _, salt := range salts {
		hash := Crypt(pw, salt)
		t.Logf("salt=%q hash=%q", salt, hash)
		if !CheckPassword(pw, hash) {
			t.Errorf("Failed to verify password with salt %q", salt)
		}
	}
}

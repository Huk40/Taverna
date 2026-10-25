package auth

import "testing"

func TestPasswordHashUsesArgon2idAndVerifies(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, "correct horse battery staple") {
		t.Fatal("valid password was rejected")
	}
	if verifyPassword(hash, "wrong password") {
		t.Fatal("wrong password was accepted")
	}
}

func TestPasswordValidationAndEmailNormalization(t *testing.T) {
	if err := validatePassword("too-short"); err == nil {
		t.Fatal("short password was accepted")
	}
	email, err := normalizeEmail("User@Example.COM")
	if err != nil || email != "user@example.com" {
		t.Fatalf("normalizeEmail() = %q, %v", email, err)
	}
	if _, err := normalizeEmail("not an email"); err == nil {
		t.Fatal("invalid email was accepted")
	}
}

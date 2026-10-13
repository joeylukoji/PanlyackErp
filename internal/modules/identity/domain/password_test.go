package domain

import "testing"

func TestPasswordHashAndVerification(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if encoded == "correct horse battery staple" || !VerifyPassword(encoded, "correct horse battery staple") {
		t.Fatal("valid password was not stored or verified safely")
	}
	if VerifyPassword(encoded, "wrong password") || VerifyPassword("malformed", "correct horse battery staple") {
		t.Fatal("invalid credential accepted")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("weak password accepted")
	}
}

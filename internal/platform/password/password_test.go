package password

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("synthetic-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(hash, "synthetic-test-password") || Verify(hash, "incorrect-password") {
		t.Fatal("incorrect verification")
	}
	other, err := Hash("synthetic-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if hash == other {
		t.Fatal("salts must be random")
	}
	for _, bad := range []string{"", hash + "$extra", strings.Replace(hash, "m=65536", "m=999999999", 1), strings.Replace(hash, "v=19", "v=18", 1), strings.Replace(hash, "argon2id", "argon2i", 1), "$argon2id$v=19$m=65536,t=3,p=2$bad$bad"} {
		if Verify(bad, "synthetic-test-password") {
			t.Fatal("accepted malformed hash")
		}
	}
	if Verify(hash, strings.Repeat("x", MaxBytes+1)) {
		t.Fatal("accepted oversized input")
	}
}
func TestPasswordBounds(t *testing.T) {
	for _, value := range []string{"short", strings.Repeat("x", MaxBytes+1)} {
		if _, err := Hash(value); err != ErrInvalid {
			t.Fatalf("expected invalid: %v", err)
		}
	}
}

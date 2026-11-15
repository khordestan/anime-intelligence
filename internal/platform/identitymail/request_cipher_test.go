package identitymail

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestRequestEnvelopeHidesLengthAndBindsTypeAndRow(t *testing.T) {
	cipher, _ := NewEnvelopeCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)))
	var first []byte
	for _, value := range []string{"", "short@example.test", strings.Repeat("x", 254), strings.Repeat("\x00", 254)} {
		data, err := cipher.SealRequest("request-one", PasswordReset, value)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 540 {
			t.Fatal("length disclosed")
		}
		purpose, got, err := cipher.OpenRequest("request-one", data)
		if err != nil || purpose != PasswordReset || got != value {
			t.Fatal("request roundtrip", err)
		}
		if _, _, err = cipher.OpenRequest("request-two", data); err != ErrEnvelope {
			t.Fatal("substitution accepted")
		}
		if _, err = cipher.Open("request-one", data); err != ErrEnvelope {
			t.Fatal("request used as delivery")
		}
		first = data
	}
	first[len(first)-1] ^= 1
	if _, _, err := cipher.OpenRequest("request-one", first); err != ErrEnvelope {
		t.Fatal("tampering accepted")
	}
	if _, err := cipher.SealRequest("request-one", EmailVerification, strings.Repeat("A", 43)); err != ErrEnvelope {
		t.Fatal("raw bearer accepted as request value")
	}
	if _, err := cipher.SealRequest("request-one", "invalid", "value"); err != ErrEnvelope {
		t.Fatal("invalid purpose accepted")
	}
	if _, err := cipher.SealRequest("request-one", PasswordReset, strings.Repeat("a", 255)); err != ErrEnvelope {
		t.Fatal("oversized value accepted")
	}
}

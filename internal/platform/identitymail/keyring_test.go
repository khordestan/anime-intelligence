package identitymail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func syntheticKey(b byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32))
}
func TestKeyringReadsPreviousKeysButWritesOnlyPrimary(t *testing.T) {
	keys := []string{syntheticKey(1), syntheticKey(2), syntheticKey(3), syntheticKey(4)}
	ring, err := NewEnvelopeKeyring(keys[0], keys[1:])
	if err != nil {
		t.Fatal(err)
	}
	message := Message{Purpose: PasswordReset, Recipient: "rotation@example.test", Token: strings.Repeat("A", 43), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	for _, encoded := range keys {
		single, _ := NewEnvelopeCipher(encoded)
		oldMail, err := single.Seal("mail-one", message)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ring.Open("mail-one", oldMail)
		if err != nil || got != message {
			t.Fatal("previous mail key rejected", err)
		}
		oldRequest, err := single.SealRequest("request-one", PasswordReset, message.Recipient)
		if err != nil {
			t.Fatal(err)
		}
		purpose, value, err := ring.OpenRequest("request-one", oldRequest)
		if err != nil || purpose != PasswordReset || value != message.Recipient {
			t.Fatal("previous request key rejected", err)
		}
		if _, _, err = ring.OpenRequest("request-two", oldRequest); err != ErrEnvelope {
			t.Fatal("rotation bypassed row binding")
		}
	}
	newMail, err := ring.Seal("mail-one", message)
	if err != nil {
		t.Fatal(err)
	}
	newRequest, err := ring.SealRequest("request-one", PasswordReset, message.Recipient)
	if err != nil {
		t.Fatal(err)
	}
	for i, encoded := range keys {
		single, _ := NewEnvelopeCipher(encoded)
		_, mailErr := single.Open("mail-one", newMail)
		_, _, requestErr := single.OpenRequest("request-one", newRequest)
		if i == 0 && (mailErr != nil || requestErr != nil) {
			t.Fatal("primary cannot read new work")
		}
		if i > 0 && (mailErr != ErrEnvelope || requestErr != ErrEnvelope) {
			t.Fatal("new work encrypted with retired key")
		}
	}
	newMail[len(newMail)-1] ^= 1
	if _, err = ring.Open("mail-one", newMail); err != ErrEnvelope {
		t.Fatal("keyring accepted tampering")
	}
	rendered := fmt.Sprintf("%v %+v %#v", ring, ring, ring)
	for _, key := range keys {
		if strings.Contains(rendered, key) {
			t.Fatal("key leaked through formatting")
		}
	}
}
func TestKeyringRejectsUnsafeOrAmbiguousConfiguration(t *testing.T) {
	primary := syntheticKey(1)
	for _, previous := range [][]string{
		{primary}, {syntheticKey(2), syntheticKey(2)}, {""}, {"bad-key"}, {syntheticKey(2) + "\n"},
		{syntheticKey(2), syntheticKey(3), syntheticKey(4), syntheticKey(5)},
	} {
		if _, err := NewEnvelopeKeyring(primary, previous); err != ErrEnvelope {
			t.Fatal("unsafe keyring accepted")
		}
	}
	for _, primary := range []string{"", "bad-key", primary + "\n"} {
		if _, err := NewEnvelopeKeyring(primary, nil); err != ErrEnvelope {
			t.Fatal("unsafe primary accepted")
		}
	}
}

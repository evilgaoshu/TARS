package postgres

import (
	"bytes"
	"testing"
)

func TestEncryptedSecretEnvelopeDoesNotContainPlaintextAndRoundTrips(t *testing.T) {
	key, err := normalizeSecretCustodyMasterKey("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("normalizeSecretCustodyMasterKey() error = %v", err)
	}
	plaintext := []byte("local-only-ssh-private-key-material")
	sealed, err := sealSecret(key, plaintext)
	if err != nil {
		t.Fatalf("sealSecret() error = %v", err)
	}
	if bytes.Contains(sealed.Ciphertext, plaintext) {
		t.Fatalf("ciphertext contains plaintext material")
	}
	opened, err := openSecret(key, sealed)
	if err != nil {
		t.Fatalf("openSecret() error = %v", err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("openSecret() = %q, want %q", string(opened), string(plaintext))
	}
}

func TestNormalizeSecretCustodyMasterKeyRejectsShortKeys(t *testing.T) {
	if _, err := normalizeSecretCustodyMasterKey("short"); err == nil {
		t.Fatalf("expected short custody key to be rejected")
	}
}

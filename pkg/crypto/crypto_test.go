package crypto

import (
	"strings"
	"testing"
)

func key32(b byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = b
	}
	return k
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty string", ""},
		{"short password", "secret123"},
		{"google app password", "abcd efgh ijkl mnop"},
		{"unicode", "contraseña-segura-€"},
	}
	key := key32(0xAB)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := Encrypt(tc.plaintext, key)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			got, err := Decrypt(enc, key)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if got != tc.plaintext {
				t.Errorf("got %q, want %q", got, tc.plaintext)
			}
		})
	}
}

func TestEncryptProducesUniqueNonces(t *testing.T) {
	key := key32(0x01)
	a, _ := Encrypt("same", key)
	b, _ := Encrypt("same", key)
	if a == b {
		t.Error("expected different ciphertexts for same plaintext (nonce reuse)")
	}
}

func TestWrongKeyFails(t *testing.T) {
	key1 := key32(0x01)
	key2 := key32(0x02)
	enc, _ := Encrypt("hello", key1)
	_, err := Decrypt(enc, key2)
	if err == nil {
		t.Error("expected decryption to fail with wrong key")
	}
}

func TestBadKeySizeFails(t *testing.T) {
	_, err := Encrypt("x", []byte("tooshort"))
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Errorf("expected key-size error, got %v", err)
	}
}

func TestTamperedCiphertextFails(t *testing.T) {
	key := key32(0xAB)
	enc, _ := Encrypt("hello", key)
	// flip the last character in the base64 string
	tampered := enc[:len(enc)-1] + string(rune(enc[len(enc)-1]+1))
	_, err := Decrypt(tampered, key)
	if err == nil {
		t.Error("expected decryption to fail on tampered ciphertext")
	}
}

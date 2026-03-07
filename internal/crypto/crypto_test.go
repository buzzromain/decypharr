package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

// ── DeriveKeys ────────────────────────────────────────────────────────────────

func TestDeriveKeys_Deterministic(t *testing.T) {
	t.Parallel()
	password := []byte("test-password")
	salt := make([]byte, 16)
	for i := range salt {
		salt[i] = byte(i)
	}

	k1 := DeriveKeys(password, salt, 4)
	k2 := DeriveKeys(password, salt, 4)

	if !bytes.Equal(k1.Key, k2.Key) {
		t.Error("DeriveKeys should be deterministic: Key differs")
	}
	if !bytes.Equal(k1.CheckKey, k2.CheckKey) {
		t.Error("DeriveKeys should be deterministic: CheckKey differs")
	}
	if !bytes.Equal(k1.PwCheck, k2.PwCheck) {
		t.Error("DeriveKeys should be deterministic: PwCheck differs")
	}
}

func TestDeriveKeys_KeyLength(t *testing.T) {
	t.Parallel()
	keys := DeriveKeys([]byte("password"), []byte("salt"), 4)
	if len(keys.Key) != AESKeySize {
		t.Errorf("Key length = %d, want %d", len(keys.Key), AESKeySize)
	}
	if len(keys.PwCheck) != PwCheckSize+4 {
		t.Errorf("PwCheck length = %d, want %d", len(keys.PwCheck), PwCheckSize+4)
	}
}

func TestDeriveKeys_DifferentPasswords(t *testing.T) {
	t.Parallel()
	salt := []byte("fixed-salt")
	k1 := DeriveKeys([]byte("password1"), salt, 4)
	k2 := DeriveKeys([]byte("password2"), salt, 4)
	if bytes.Equal(k1.Key, k2.Key) {
		t.Error("different passwords should produce different keys")
	}
}

func TestDeriveKeys_DifferentSalts(t *testing.T) {
	t.Parallel()
	password := []byte("fixed-password")
	k1 := DeriveKeys(password, []byte("salt-aaaa"), 4)
	k2 := DeriveKeys(password, []byte("salt-bbbb"), 4)
	if bytes.Equal(k1.Key, k2.Key) {
		t.Error("different salts should produce different keys")
	}
}

func TestDeriveKeys_LongSalt_Truncated(t *testing.T) {
	t.Parallel()
	password := []byte("password")
	longSalt := make([]byte, MaxPbkdf2Salt+10)
	normalSalt := longSalt[:MaxPbkdf2Salt]

	k1 := DeriveKeys(password, longSalt, 4)
	k2 := DeriveKeys(password, normalSalt, 4)

	if !bytes.Equal(k1.Key, k2.Key) {
		t.Error("long salt should be truncated to MaxPbkdf2Salt, producing same key")
	}
}

func TestDeriveKeys_DifferentKdfCount(t *testing.T) {
	t.Parallel()
	password := []byte("password")
	salt := []byte("salt")
	k1 := DeriveKeys(password, salt, 4)
	k2 := DeriveKeys(password, salt, 8)
	if bytes.Equal(k1.Key, k2.Key) {
		t.Error("different kdfCount should produce different keys")
	}
}

// ── VerifyPassword ────────────────────────────────────────────────────────────

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	t.Parallel()
	password := []byte("correct-password")
	salt := []byte("test-salt-16byte")
	keys := DeriveKeys(password, salt, 4)
	if !VerifyPassword(keys, keys.PwCheck) {
		t.Error("VerifyPassword should return true for correct password")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	t.Parallel()
	salt := []byte("test-salt-16byte")
	goodKeys := DeriveKeys([]byte("correct"), salt, 4)
	badKeys := DeriveKeys([]byte("wrong"), salt, 4)

	if VerifyPassword(badKeys, goodKeys.PwCheck) {
		t.Error("VerifyPassword should return false for wrong password")
	}
}

func TestVerifyPassword_WrongLength(t *testing.T) {
	t.Parallel()
	keys := DeriveKeys([]byte("password"), []byte("salt"), 4)
	shortCheck := keys.PwCheck[:4]
	if VerifyPassword(keys, shortCheck) {
		t.Error("VerifyPassword should return false for wrong-length check value")
	}
}

func TestVerifyPassword_EmptyCheck(t *testing.T) {
	t.Parallel()
	keys := DeriveKeys([]byte("password"), []byte("salt"), 4)
	if VerifyPassword(keys, []byte{}) {
		t.Error("VerifyPassword should return false for empty check value")
	}
}

// ── NewDecrypter ─────────────────────────────────────────────────────────────

func TestNewDecrypter_ValidInputs(t *testing.T) {
	t.Parallel()
	key := make([]byte, AESKeySize)
	iv := make([]byte, BlockSize)
	mode, err := NewDecrypter(key, iv)
	if err != nil {
		t.Fatalf("NewDecrypter failed: %v", err)
	}
	if mode == nil {
		t.Error("NewDecrypter returned nil mode")
	}
}

func TestNewDecrypter_InvalidKeySize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		keySize int
	}{
		{"too short", 8},
		{"AES-128 not AES-256", 16},
		{"empty", 0},
	}
	iv := make([]byte, BlockSize)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewDecrypter(make([]byte, tt.keySize), iv)
			if err == nil {
				t.Errorf("expected error for key size %d", tt.keySize)
			}
		})
	}
}

func TestNewDecrypter_InvalidIVSize(t *testing.T) {
	t.Parallel()
	key := make([]byte, AESKeySize)
	tests := []struct {
		name   string
		ivSize int
	}{
		{"too short", 8},
		{"empty", 0},
		{"one byte too long", 17},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewDecrypter(key, make([]byte, tt.ivSize))
			if err == nil {
				t.Errorf("expected error for IV size %d", tt.ivSize)
			}
		})
	}
}

// ── DecryptBlock ─────────────────────────────────────────────────────────────

func TestDecryptBlock_NonAlignedData(t *testing.T) {
	t.Parallel()
	key := make([]byte, AESKeySize)
	iv := make([]byte, BlockSize)

	for _, size := range []int{1, 7, 17, 31} {
		err := DecryptBlock(make([]byte, size), key, iv)
		if err == nil {
			t.Errorf("expected error for non-aligned data size %d", size)
		}
	}
}

func TestDecryptBlock_EmptyData(t *testing.T) {
	t.Parallel()
	key := make([]byte, AESKeySize)
	iv := make([]byte, BlockSize)
	// Empty data (0 bytes): 0 % 16 == 0, should succeed (no-op)
	err := DecryptBlock([]byte{}, key, iv)
	if err != nil {
		t.Errorf("DecryptBlock on empty data should succeed, got: %v", err)
	}
}

func TestDecryptBlock_InvalidKey(t *testing.T) {
	t.Parallel()
	shortKey := make([]byte, 8)
	iv := make([]byte, BlockSize)
	data := make([]byte, 16)
	if err := DecryptBlock(data, shortKey, iv); err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestDecryptBlock_RoundTrip(t *testing.T) {
	t.Parallel()
	key := make([]byte, AESKeySize)
	for i := range key {
		key[i] = byte(i)
	}
	iv := make([]byte, BlockSize)
	for i := range iv {
		iv[i] = byte(i * 2 % 256)
	}

	original := make([]byte, 32)
	for i := range original {
		original[i] = byte(i + 1)
	}

	// Encrypt
	encrypted := encryptAESCBC(t, key, iv, original)

	// Decrypt
	err := DecryptBlock(encrypted, key, iv)
	if err != nil {
		t.Fatalf("DecryptBlock failed: %v", err)
	}
	if !bytes.Equal(encrypted, original) {
		t.Error("round-trip decrypt did not recover original data")
	}
}

// encryptAESCBC encrypts using AES-256-CBC for test round-trips
func encryptAESCBC(t *testing.T, key, iv, plaintext []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}
	out := make([]byte, len(plaintext))
	copy(out, plaintext)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, out)
	return out
}

// ── NewDecryptReader ──────────────────────────────────────────────────────────

func TestNewDecryptReader_InvalidKey(t *testing.T) {
	t.Parallel()
	_, err := NewDecryptReader(bytes.NewReader(nil), make([]byte, 8), make([]byte, BlockSize))
	if err == nil {
		t.Error("expected error for invalid key size")
	}
}

func TestNewDecryptReader_InvalidIV(t *testing.T) {
	t.Parallel()
	_, err := NewDecryptReader(bytes.NewReader(nil), make([]byte, AESKeySize), make([]byte, 8))
	if err == nil {
		t.Error("expected error for invalid IV size")
	}
}

func TestDecryptReader_RoundTrip(t *testing.T) {
	t.Parallel()
	key := make([]byte, AESKeySize)
	for i := range key {
		key[i] = byte(i * 3 % 256)
	}
	iv := make([]byte, BlockSize)
	for i := range iv {
		iv[i] = byte(i * 5 % 256)
	}

	original := make([]byte, 64)
	for i := range original {
		original[i] = byte(i)
	}

	// Encrypt
	encrypted := encryptAESCBC(t, key, iv, original)

	// DecryptReader
	r, err := NewDecryptReader(bytes.NewReader(encrypted), key, iv)
	if err != nil {
		t.Fatalf("NewDecryptReader failed: %v", err)
	}

	buf := make([]byte, len(original))
	n, err := r.Read(buf)
	if err != nil && err.Error() != "EOF" {
		// Read might return the data + EOF in a single call, which is fine
	}
	_ = n

	// Read all remaining
	var result []byte
	result = append(result, buf[:n]...)
	more := make([]byte, 256)
	for {
		n2, e := r.Read(more)
		if n2 > 0 {
			result = append(result, more[:n2]...)
		}
		if e != nil {
			break
		}
	}

	if !bytes.Equal(result, original) {
		t.Errorf("DecryptReader round-trip failed: got %v, want %v", result, original)
	}
}

// ── ParseEncryptionHeader ─────────────────────────────────────────────────────

func TestParseEncryptionHeader_TooShort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too short", []byte{0, 0, 0, 0, 0}},
		{"17 bytes", make([]byte, 17)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseEncryptionHeader(tt.data)
			if err == nil {
				t.Error("expected error for too-short data")
			}
		})
	}
}

func TestParseEncryptionHeader_Valid_WithoutPwCheck(t *testing.T) {
	t.Parallel()
	// Build minimal valid header: version=0, flags=0 (no pwcheck), kdf=8, salt=16 bytes
	data := make([]byte, 19) // 1+1+1+16 = 19
	data[0] = 0              // version = 0
	data[1] = 0              // flags = 0 (no password check)
	data[2] = 8              // kdfCount = 8
	// salt = data[3:19] = all zeros

	h, err := ParseEncryptionHeader(data)
	if err != nil {
		t.Fatalf("ParseEncryptionHeader failed: %v", err)
	}
	if h.Version != 0 {
		t.Errorf("Version = %d, want 0", h.Version)
	}
	if h.KdfCount != 8 {
		t.Errorf("KdfCount = %d, want 8", h.KdfCount)
	}
	if len(h.Salt) != 16 {
		t.Errorf("Salt length = %d, want 16", len(h.Salt))
	}
	if h.HasPwCheck {
		t.Error("expected HasPwCheck=false when flag bit 0 not set")
	}
}

func TestParseEncryptionHeader_Valid_WithPwCheck(t *testing.T) {
	t.Parallel()
	// version=0, flags=1 (has pwcheck), kdf=12, salt=16 bytes, pwcheck=12 bytes
	data := make([]byte, 31) // 1+1+1+16+12
	data[0] = 0              // version = 0
	data[1] = 1              // flags = 1 (has password check)
	data[2] = 12             // kdfCount = 12
	// salt = data[3:19], pwcheck = data[19:31]

	h, err := ParseEncryptionHeader(data)
	if err != nil {
		t.Fatalf("ParseEncryptionHeader failed: %v", err)
	}
	if !h.HasPwCheck {
		t.Error("expected HasPwCheck=true when flag bit 0 set")
	}
	if len(h.PwCheck) != 12 {
		t.Errorf("PwCheck length = %d, want 12", len(h.PwCheck))
	}
}

func TestParseEncryptionHeader_PwCheckTooShort(t *testing.T) {
	t.Parallel()
	// flags=1 but not enough bytes for pwcheck
	data := make([]byte, 20) // 1+1+1+16+1 = 20 (only 1 byte for pwcheck, need 12)
	data[0] = 0
	data[1] = 1 // flags = 1 (has pwcheck)
	data[2] = 8

	_, err := ParseEncryptionHeader(data)
	if err == nil {
		t.Error("expected error when pwcheck data too short")
	}
}

func TestParseEncryptionHeader_MultiByteVersion(t *testing.T) {
	t.Parallel()
	// version with high bit set → invalid
	data := make([]byte, 20)
	data[0] = 0x80 // high bit set = multi-byte, should fail

	_, err := ParseEncryptionHeader(data)
	if err == nil {
		t.Error("expected error for multi-byte version")
	}
}

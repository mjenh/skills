package crypto

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 bytes
	iv := []byte("abcdef0123456789")  // 16 bytes
	plaintext := []byte("Hello, Tapo!")

	ciphertext, err := Encrypt(key, iv, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch: got %q, want %q", got, plaintext)
	}
}

// NIST SP 800-38A F.2.1 AES-128-CBC Encrypt test vector.
func TestEncryptNISTVector(t *testing.T) {
	key, _ := hex.DecodeString("2b7e151628aed2a6abf7158809cf4f3c")
	iv, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")

	// NIST plaintext block 1
	plaintext, _ := hex.DecodeString("6bc1bee22e409f96e93d7e117393172a")
	// Expected ciphertext for block 1 (without PKCS7 padding on input,
	// but our Encrypt adds padding so we get two blocks).
	ciphertext, err := Encrypt(key, iv, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// First block of output must match NIST expected ciphertext.
	expectedBlock, _ := hex.DecodeString("7649abac8119b246cee98e9b12e9197d")
	if !bytes.Equal(ciphertext[:aes.BlockSize], expectedBlock) {
		t.Errorf("first block mismatch:\n  got  %x\n  want %x", ciphertext[:aes.BlockSize], expectedBlock)
	}

	// Verify round-trip.
	got, err := Decrypt(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch after NIST vector encrypt")
	}
}

func TestPKCS7PadBlockAligned(t *testing.T) {
	// Plaintext exactly one block: padding adds a full block of 0x10.
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	plaintext := []byte("exactly16chars!!")

	if len(plaintext) != aes.BlockSize {
		t.Fatalf("test setup: plaintext len %d, want %d", len(plaintext), aes.BlockSize)
	}

	ciphertext, err := Encrypt(key, iv, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Should be two blocks: original + padding block.
	if len(ciphertext) != 2*aes.BlockSize {
		t.Errorf("ciphertext len %d, want %d", len(ciphertext), 2*aes.BlockSize)
	}

	got, err := Decrypt(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch for block-aligned plaintext")
	}
}

func TestPKCS7PadEmpty(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")

	ciphertext, err := Encrypt(key, iv, []byte{})
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	// Empty plaintext padded to one full block.
	if len(ciphertext) != aes.BlockSize {
		t.Errorf("ciphertext len %d, want %d", len(ciphertext), aes.BlockSize)
	}

	got, err := Decrypt(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty plaintext, got %q", got)
	}
}

func TestPKCS7PadSingleByte(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	plaintext := []byte{0x42}

	ciphertext, err := Encrypt(key, iv, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch for single byte")
	}
}

func TestDecryptInvalidKeyLength(t *testing.T) {
	badKey := []byte("short")
	iv := []byte("abcdef0123456789")
	ciphertext := make([]byte, aes.BlockSize)

	_, err := Decrypt(badKey, iv, ciphertext)
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestEncryptInvalidKeyLength(t *testing.T) {
	badKey := []byte("short")
	iv := []byte("abcdef0123456789")

	_, err := Encrypt(badKey, iv, []byte("test"))
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestDecryptNotBlockAligned(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	ciphertext := []byte("not-aligned") // 11 bytes, not multiple of 16

	_, err := Decrypt(key, iv, ciphertext)
	if err == nil {
		t.Fatal("expected error for non-block-aligned ciphertext")
	}
}

func TestDecryptEmptyCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")

	_, err := Decrypt(key, iv, []byte{})
	if err == nil {
		t.Fatal("expected error for empty ciphertext")
	}
}

func TestDecryptCorruptedPadding(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")
	plaintext := []byte("Hello")

	ciphertext, err := Encrypt(key, iv, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Corrupt last byte of ciphertext to break padding.
	ciphertext[len(ciphertext)-1] ^= 0xff

	_, err = Decrypt(key, iv, ciphertext)
	if err == nil {
		t.Fatal("expected error for corrupted padding")
	}
}

func TestPkcs7PadUnpad(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"one byte", []byte{0x01}},
		{"15 bytes", bytes.Repeat([]byte{0xAB}, 15)},
		{"16 bytes", bytes.Repeat([]byte{0xCD}, 16)},
		{"17 bytes", bytes.Repeat([]byte{0xEF}, 17)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := pkcs7Pad(tt.data, aes.BlockSize)
			if len(padded)%aes.BlockSize != 0 {
				t.Fatalf("padded length %d not multiple of block size", len(padded))
			}
			unpadded, err := pkcs7Unpad(padded, aes.BlockSize)
			if err != nil {
				t.Fatalf("pkcs7Unpad: %v", err)
			}
			if !bytes.Equal(unpadded, tt.data) {
				t.Errorf("pad/unpad round-trip mismatch")
			}
		})
	}
}

func TestPkcs7UnpadInvalid(t *testing.T) {
	// Padding byte is 0.
	_, err := pkcs7Unpad([]byte{0x41, 0x41, 0x41, 0x00}, aes.BlockSize)
	if err == nil {
		t.Error("expected error for zero padding byte")
	}

	// Padding byte larger than block size.
	_, err = pkcs7Unpad([]byte{0x41, 0x41, 0x41, 0x11}, aes.BlockSize)
	if err == nil {
		t.Error("expected error for padding byte > block size")
	}

	// Empty input.
	_, err = pkcs7Unpad([]byte{}, aes.BlockSize)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestEncryptInvalidIVLength(t *testing.T) {
	key := []byte("0123456789abcdef")
	shortIV := []byte("short")

	_, err := Encrypt(key, shortIV, []byte("test"))
	if err == nil {
		t.Fatal("expected error for invalid IV length")
	}
}

func TestDecryptInvalidIVLength(t *testing.T) {
	key := []byte("0123456789abcdef")
	shortIV := []byte("short")
	ciphertext := make([]byte, 16) // valid block size

	_, err := Decrypt(key, shortIV, ciphertext)
	if err == nil {
		t.Fatal("expected error for invalid IV length")
	}
}

func TestPkcs7UnpadPaddingGreaterThanLen(t *testing.T) {
	// Last byte claims padding of 5, but data is only 3 bytes long.
	// This must be rejected even though 5 <= blockSize.
	data := []byte{0x41, 0x41, 0x05}

	_, err := pkcs7Unpad(data, aes.BlockSize)
	if err == nil {
		t.Fatal("expected error when padding length exceeds data length")
	}
}

func TestPkcs7UnpadInconsistentPaddingBytes(t *testing.T) {
	// Last byte says padding of 3, so the last three bytes should all be 0x03.
	// Corrupt the middle padding byte so it doesn't match.
	data := []byte{0x41, 0x41, 0x03, 0x99, 0x03}

	_, err := pkcs7Unpad(data, aes.BlockSize)
	if err == nil {
		t.Fatal("expected error for inconsistent padding bytes")
	}
}

func TestEncryptDecryptAES256Key(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes -> AES-256
	key = key[:32]
	iv := []byte("abcdef0123456789") // 16 bytes
	plaintext := []byte("Hello, AES-256!")

	ciphertext, err := Encrypt(key, iv, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(key, iv, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch with AES-256 key: got %q, want %q", got, plaintext)
	}
}

func TestDecryptEmptyCiphertextReturnsError(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")

	got, err := Decrypt(key, iv, []byte{})
	if err == nil {
		t.Fatal("expected error for empty ciphertext")
	}
	if got != nil {
		t.Errorf("expected nil plaintext on error, got %v", got)
	}
}

func TestDecryptCiphertextNotMultipleOfBlockSize(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := []byte("abcdef0123456789")

	// 20 bytes: more than one block but not a multiple of 16.
	ciphertext := bytes.Repeat([]byte{0x01}, 20)

	_, err := Decrypt(key, iv, ciphertext)
	if err == nil {
		t.Fatal("expected error for ciphertext not a multiple of block size")
	}
}

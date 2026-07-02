package crypto

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestKLAPAuthHashKnownInputs(t *testing.T) {
	email := "test@example.com"
	password := "testpass"

	// Manually compute expected: SHA256(SHA1(email) + SHA1(password))
	emailSHA1 := sha1.Sum([]byte(email))
	passSHA1 := sha1.Sum([]byte(password))

	combined := append(emailSHA1[:], passSHA1[:]...)
	expected := sha256.Sum256(combined)

	got := KLAPAuthHash(email, password)

	if !bytes.Equal(got, expected[:]) {
		t.Errorf("KLAPAuthHash mismatch:\n  got  %x\n  want %x", got, expected[:])
	}
}

func TestKLAPAuthHashLength(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
	}{
		{"normal", "user@example.com", "password123"},
		{"empty email", "", "password"},
		{"empty password", "user@example.com", ""},
		{"both empty", "", ""},
		{"unicode email", "user@examplé.com", "pass"},
		{"unicode password", "user@example.com", "пароль"},
		{"unicode both", "用户@例子.com", "密码"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KLAPAuthHash(tt.email, tt.password)
			if len(got) != 32 {
				t.Errorf("output length %d, want 32", len(got))
			}
		})
	}
}

func TestKLAPAuthHashEmptyInputs(t *testing.T) {
	// Must not panic and must return 32 bytes.
	got := KLAPAuthHash("", "")
	if len(got) != 32 {
		t.Fatalf("output length %d, want 32", len(got))
	}
}

func TestKLAPAuthHashUnicode(t *testing.T) {
	got := KLAPAuthHash("tëst@example.com", "pässwörd")
	if len(got) != 32 {
		t.Fatalf("output length %d, want 32", len(got))
	}

	// Different unicode input should produce different hash.
	got2 := KLAPAuthHash("test@example.com", "password")
	if bytes.Equal(got, got2) {
		t.Error("different inputs produced same hash")
	}
}

func TestKLAPDeriveKeyIVSeqSigFixedSeeds(t *testing.T) {
	localSeed, _ := hex.DecodeString("0102030405060708091011121314151617181920212223242526272829303132")
	remoteSeed, _ := hex.DecodeString("a1a2a3a4a5a6a7a8a9b0b1b2b3b4b5b6b7b8b9c0c1c2c3c4c5c6c7c8c9d0d1d2")
	authHash := KLAPAuthHash("test@example.com", "testpass")

	key, iv, seq, sig := KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash)

	if len(key) != 16 {
		t.Errorf("key length %d, want 16", len(key))
	}
	if len(iv) != 12 {
		t.Errorf("iv length %d, want 12", len(iv))
	}
	if len(sig) != 28 {
		t.Errorf("sig length %d, want 28", len(sig))
	}

	// Manually verify expected values.
	payload := make([]byte, 0, len(localSeed)+len(remoteSeed)+len(authHash))
	payload = append(payload, localSeed...)
	payload = append(payload, remoteSeed...)
	payload = append(payload, authHash...)

	expectedKeyHash := sha256.Sum256(append([]byte("lsk"), payload...))
	expectedIVHash := sha256.Sum256(append([]byte("iv"), payload...))
	expectedSigHash := sha256.Sum256(append([]byte("ldk"), payload...))

	if !bytes.Equal(key, expectedKeyHash[:16]) {
		t.Errorf("key mismatch:\n  got  %x\n  want %x", key, expectedKeyHash[:16])
	}
	if !bytes.Equal(iv, expectedIVHash[:12]) {
		t.Errorf("iv mismatch:\n  got  %x\n  want %x", iv, expectedIVHash[:12])
	}

	expectedSeq := binary.BigEndian.Uint32(expectedIVHash[28:])
	if seq != expectedSeq {
		t.Errorf("seq %d, want %d", seq, expectedSeq)
	}
	if !bytes.Equal(sig, expectedSigHash[:28]) {
		t.Errorf("sig mismatch:\n  got  %x\n  want %x", sig, expectedSigHash[:28])
	}
}

func TestKLAPDeriveKeyIVSeqSigDeterminism(t *testing.T) {
	localSeed := []byte("local-seed-value-here-16b")
	remoteSeed := []byte("remote-seed-value-here16")
	authHash := KLAPAuthHash("a@b.com", "pass")

	key1, iv1, seq1, sig1 := KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash)
	key2, iv2, seq2, sig2 := KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash)

	if !bytes.Equal(key1, key2) {
		t.Error("key not deterministic")
	}
	if !bytes.Equal(iv1, iv2) {
		t.Error("iv not deterministic")
	}
	if seq1 != seq2 {
		t.Error("seq not deterministic")
	}
	if !bytes.Equal(sig1, sig2) {
		t.Error("sig not deterministic")
	}
}

func TestKLAPDeriveKeyIVSeqSigSensitivity(t *testing.T) {
	localSeed := []byte("local-seed-value-here-16b")
	remoteSeed := []byte("remote-seed-value-here16")
	authHash := KLAPAuthHash("a@b.com", "pass")

	baseKey, baseIV, baseSeq, baseSig := KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash)

	// Change local seed.
	altLocal := []byte("LOCAL-SEED-VALUE-HERE-16B")
	key2, iv2, seq2, sig2 := KLAPDeriveKeyIVSeqSig(altLocal, remoteSeed, authHash)
	if bytes.Equal(baseKey, key2) {
		t.Error("changing localSeed did not change key")
	}
	if bytes.Equal(baseIV, iv2) {
		t.Error("changing localSeed did not change iv")
	}

	// Change remote seed.
	altRemote := []byte("REMOTE-SEED-VALUE-HERE16")
	key3, iv3, seq3, sig3 := KLAPDeriveKeyIVSeqSig(localSeed, altRemote, authHash)
	if bytes.Equal(baseKey, key3) {
		t.Error("changing remoteSeed did not change key")
	}
	if bytes.Equal(baseIV, iv3) {
		t.Error("changing remoteSeed did not change iv")
	}

	// Change auth hash.
	altAuth := KLAPAuthHash("different@b.com", "pass")
	key4, iv4, seq4, sig4 := KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, altAuth)
	if bytes.Equal(baseKey, key4) {
		t.Error("changing authHash did not change key")
	}
	if bytes.Equal(baseIV, iv4) {
		t.Error("changing authHash did not change iv")
	}

	// At least one seq should differ (statistically near certain with different inputs).
	if seq2 == baseSeq && seq3 == baseSeq && seq4 == baseSeq {
		t.Error("seq unchanged for all varied inputs (statistically improbable)")
	}

	// At least one sig should differ.
	if bytes.Equal(baseSig, sig2) && bytes.Equal(baseSig, sig3) && bytes.Equal(baseSig, sig4) {
		t.Error("sig unchanged for all varied inputs (statistically improbable)")
	}

	// Suppress unused variable warnings.
	_ = seq2
	_ = seq3
	_ = seq4
}

func TestLegacyLoginHashKnownInput(t *testing.T) {
	email := "test@example.com"

	// Manually compute expected: base64(SHA1(email))
	h := sha1.Sum([]byte(email))
	expected := base64.StdEncoding.EncodeToString(h[:])

	got := LegacyLoginHash(email)
	if got != expected {
		t.Errorf("LegacyLoginHash(%q) = %q, want %q", email, got, expected)
	}
}

func TestLegacyLoginHashEmptyEmail(t *testing.T) {
	// Should not panic with empty input.
	got := LegacyLoginHash("")

	// Result should be valid base64 that decodes to 20 bytes (SHA1 output).
	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("result is not valid base64: %v", err)
	}
	if len(decoded) != 20 {
		t.Errorf("decoded length %d, want 20", len(decoded))
	}
}

func TestLegacyLoginHashValidBase64(t *testing.T) {
	got := LegacyLoginHash("user@example.com")

	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("result is not valid base64: %v", err)
	}
	if len(decoded) != 20 {
		t.Errorf("decoded length %d, want 20 (SHA1 output size)", len(decoded))
	}
}

func TestLegacyLoginHashDeterminism(t *testing.T) {
	email := "deterministic@example.com"

	result1 := LegacyLoginHash(email)
	result2 := LegacyLoginHash(email)

	if result1 != result2 {
		t.Errorf("LegacyLoginHash is not deterministic: %q != %q", result1, result2)
	}
}

func TestKLAPDeriveKeyIVSeqSigEmptyInputs(t *testing.T) {
	// Must not panic with empty slices for all three inputs, and must still
	// return correctly sized outputs.
	key, iv, seq, sig := KLAPDeriveKeyIVSeqSig([]byte{}, []byte{}, []byte{})

	if len(key) != 16 {
		t.Errorf("key length %d, want 16", len(key))
	}
	if len(iv) != 12 {
		t.Errorf("iv length %d, want 12", len(iv))
	}
	if len(sig) != 28 {
		t.Errorf("sig length %d, want 28", len(sig))
	}

	// Suppress unused variable warning; seq has no length invariant to check.
	_ = seq
}

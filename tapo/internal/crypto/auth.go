package crypto

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
)

// LegacyLoginHash returns base64(SHA1(email)) for the legacy login_device username field.
func LegacyLoginHash(email string) string {
	h := sha1.Sum([]byte(email))
	return base64.StdEncoding.EncodeToString(h[:])
}

// KLAPAuthHash computes SHA256(SHA1(email) + SHA1(password)).
// The returned slice is always 32 bytes.
func KLAPAuthHash(email, password string) []byte {
	emailHash := sha1.Sum([]byte(email))
	passHash := sha1.Sum([]byte(password))

	combined := make([]byte, 0, sha1.Size*2)
	combined = append(combined, emailHash[:]...)
	combined = append(combined, passHash[:]...)

	h := sha256.Sum256(combined)
	return h[:]
}

// KLAPDeriveKeyIVSeqSig derives a 16-byte key, 12-byte IV, sequence number,
// and 32-byte signature base from local seed, remote seed, and auth hash.
func KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash []byte) (key []byte, iv []byte, seq uint32, sig []byte) {
	payload := make([]byte, 0, len(localSeed)+len(remoteSeed)+len(authHash))
	payload = append(payload, localSeed...)
	payload = append(payload, remoteSeed...)
	payload = append(payload, authHash...)

	keyHash := sha256.Sum256(append([]byte("lsk"), payload...))
	key = keyHash[:16]

	ivHash := sha256.Sum256(append([]byte("iv"), payload...))
	iv = ivHash[:12]
	seq = binary.BigEndian.Uint32(ivHash[28:]) // last 4 bytes of same hash

	sigHash := sha256.Sum256(append([]byte("ldk"), payload...))
	sig = sigHash[:28] // first 28 bytes

	return key, iv, seq, sig
}

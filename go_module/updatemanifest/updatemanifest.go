// Package updatemanifest defines the canonical, length-prefixed payload
// that binds an MBII Foundry update manifest to its Ed25519 signature.
//
// The exact same framing must be used by the release signer
// (go_module/cmd/signer) and the in-app updater (update_installer.go).
// It lives in its own package so both binaries — and the tests — share
// one implementation and cannot drift.
package updatemanifest

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
)

// Domain prefixes every payload. It separates this protocol from any
// other signed blob the publisher key might ever sign, and versions the
// framing: bump it when the field set changes.
const Domain = "MBII-FOUNDRY-MANIFEST-v2"

// MaxArtifactBytes is the largest update payload the signer will authorize and
// the updater will download or extract. Keeping the limit in the shared package
// prevents the producer and consumer from drifting.
const MaxArtifactBytes int64 = 1 << 30

// fieldSep delimits the length prefix from the field bytes, and closes
// each length-prefixed field. Because every field also carries its byte
// length, embedded separators are unambiguous data, not structure.
const fieldSep = ':'

// recordSep bounds the fixed-position numeric fields (length) and
// precedes the digest.
const recordSep = '\x1f'

// Payload returns the canonical byte string that is signed by the
// release signer and verified by the updater:
//
//	Domain 0x1F len(version):version: len(platform):platform: len(arch):arch: 0x1F length 0x1F digest
//
// Every variable-length field is preceded by its byte length, so no
// choice of field contents can make two different field tuples produce
// the same payload — a colon inside a version string is just data.
func Payload(version, platform, arch string, length int64, digest string) ([]byte, error) {
	if err := ValidateDigest(digest); err != nil {
		return nil, err
	}
	if length < 0 || length > MaxArtifactBytes {
		return nil, fmt.Errorf("manifest length out of range: %d (want 0..%d)", length, MaxArtifactBytes)
	}
	var b bytes.Buffer
	b.WriteString(Domain)
	appendField(&b, version)
	appendField(&b, platform)
	appendField(&b, arch)
	b.WriteByte(recordSep)
	b.WriteString(strconv.FormatInt(length, 10))
	b.WriteByte(recordSep)
	b.WriteString(digest)
	return b.Bytes(), nil
}

// appendField writes recordSep, the byte length of field, fieldSep, the
// field bytes, fieldSep.
func appendField(b *bytes.Buffer, field string) {
	b.WriteByte(recordSep)
	b.WriteString(strconv.Itoa(len(field)))
	b.WriteByte(fieldSep)
	b.WriteString(field)
	b.WriteByte(fieldSep)
}

// ValidateDigest enforces the digest format: lowercase hex encoding of
// a 32-byte SHA-256 sum.
func ValidateDigest(digest string) error {
	if len(digest) != sha256DigestHexLength {
		return fmt.Errorf("manifest digest is not a SHA-256 sum: got %d hex characters, want %d", len(digest), sha256DigestHexLength)
	}
	raw, err := hex.DecodeString(digest)
	if err != nil {
		return fmt.Errorf("manifest digest is not valid hex: %w", err)
	}
	if hex.EncodeToString(raw) != digest {
		return fmt.Errorf("manifest digest is not canonical lowercase hex")
	}
	return nil
}

const sha256DigestHexLength = 64

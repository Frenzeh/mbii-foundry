package updatemanifest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const testDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestPayloadGoldenVector(t *testing.T) {
	got, err := Payload("v1.0.0", "darwin", "universal", 123456, testDigest)
	if err != nil {
		t.Fatalf("Payload: %v", err)
	}
	want := Domain +
		"\x1f6:v1.0.0:" +
		"\x1f6:darwin:" +
		"\x1f9:universal:" +
		"\x1f123456\x1f" + testDigest
	if !bytes.Equal(got, []byte(want)) {
		t.Errorf("payload mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestPayloadFieldSpoofingIsImpossible(t *testing.T) {
	// The old colon-joined format let field boundaries shift:
	// join("v1:0.2.0", "darwin", ...) == join("v1", "0.2.0:darwin", ...).
	// The length-prefixed format must keep every permutation distinct.
	a, err := Payload("v1:0.2.0", "darwin", "universal", 10, testDigest)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Payload("v1", "0.2.0:darwin", "universal", 10, testDigest)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Payload("v1", "0.2.0", "darwin:universal", 10, testDigest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) || bytes.Equal(a, c) || bytes.Equal(b, c) {
		t.Errorf("distinct field tuples produced identical payloads:\n%q\n%q\n%q", a, b, c)
	}

	// Embedded record separators must not collide with the framing either.
	d, err := Payload("v1\x1f0.2.0", "darwin", "universal", 10, testDigest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, d) {
		t.Errorf("embedded record separator did not change the payload")
	}
}

func TestPayloadSignVerifyRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := Payload("v2.3.4", "linux", "amd64", 987654, testDigest)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(priv, payload)
	if !ed25519.Verify(pub, payload, sig) {
		t.Errorf("signature does not verify over canonical payload")
	}

	// Tampering with any field invalidates: re-frame with one field
	// changed and the signature must fail.
	tampered, err := Payload("v2.3.5", "linux", "amd64", 987654, testDigest)
	if err != nil {
		t.Fatal(err)
	}
	if ed25519.Verify(pub, tampered, sig) {
		t.Errorf("signature verified over a different version string")
	}
}

func TestPayloadRejectsBadDigest(t *testing.T) {
	for _, bad := range []string{
		"",
		"zzzz",
		"abc",
		testDigest + "00",
		strings.ToUpper(testDigest),
	} {
		if _, err := Payload("v1", "darwin", "universal", 1, bad); err == nil {
			t.Errorf("expected rejection for non-canonical SHA-256 digest %q", bad)
		}
	}
}

func TestPayloadRejectsOutOfRangeLength(t *testing.T) {
	for _, length := range []int64{-1, MaxArtifactBytes + 1} {
		if _, err := Payload("v1", "darwin", "universal", length, testDigest); err == nil {
			t.Errorf("expected rejection for out-of-range length %d", length)
		}
	}
}

func TestValidateDigestAcceptsSha256Sum(t *testing.T) {
	sum := sha256.Sum256([]byte("x"))
	if err := ValidateDigest(hex.EncodeToString(sum[:])); err != nil {
		t.Errorf("valid digest rejected: %v", err)
	}
}

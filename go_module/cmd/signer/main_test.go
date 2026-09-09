package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Frenzeh/mbii-foundry/updatemanifest"
)

func TestVerifyKeypairRequiresExactMatchingCanonicalKeys(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "private.hex")
	publicPath := filepath.Join(dir, "public.hex")
	if err := os.WriteFile(privatePath, []byte(hex.EncodeToString(priv)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, []byte(hex.EncodeToString(pub)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-verify-keypair", "-key", privatePath, "-public-key-file", publicPath}, &bytes.Buffer{}); err != nil {
		t.Fatalf("matching keypair rejected: %v", err)
	}

	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, []byte(hex.EncodeToString(otherPub)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-verify-keypair", "-key", privatePath, "-public-key-file", publicPath}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched keypair accepted: %v", err)
	}

	tamperedPrivate := append(ed25519.PrivateKey(nil), priv...)
	tamperedPrivate[len(tamperedPrivate)-1] ^= 1
	if err := os.WriteFile(privatePath, []byte(hex.EncodeToString(tamperedPrivate)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, []byte(hex.EncodeToString(tamperedPrivate[ed25519.SeedSize:])), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-verify-keypair", "-key", privatePath, "-public-key-file", publicPath}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "internally inconsistent") {
		t.Fatalf("private key with forged public half accepted: %v", err)
	}
	if err := os.WriteFile(privatePath, []byte(hex.EncodeToString(priv)), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(publicPath, []byte(strings.ToUpper(hex.EncodeToString(pub))), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-verify-keypair", "-key", privatePath, "-public-key-file", publicPath}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("uppercase public key accepted: %v", err)
	}
	if err := os.WriteFile(publicPath, []byte(hex.EncodeToString(pub)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-verify-keypair", "-key", privatePath, "-public-key-file", publicPath}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "length") {
		t.Fatalf("public key with trailing newline accepted: %v", err)
	}
}

func TestSignProducesVerifiableCanonicalManifest(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "private.hex")
	artifactPath := filepath.Join(dir, "release.tar.gz")
	artifact := []byte("signed release payload")
	if err := os.WriteFile(privatePath, []byte(hex.EncodeToString(priv)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, artifact, 0644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"-sign", artifactPath,
		"-key", privatePath,
		"-version", "1.2.3",
		"-platform", "linux",
		"-arch", "amd64",
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("sign: %v", err)
	}
	manifestBytes, err := os.ReadFile(artifactPath + ".manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Length != int64(len(artifact)) {
		t.Fatalf("length = %d, want %d", manifest.Length, len(artifact))
	}
	if err := updatemanifest.ValidateDigest(manifest.Digest); err != nil {
		t.Fatalf("non-canonical digest: %v", err)
	}
	signature, err := hex.DecodeString(manifest.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		t.Fatalf("invalid signature encoding: len=%d err=%v", len(signature), err)
	}
	payload, err := updatemanifest.Payload(manifest.Version, manifest.Platform, manifest.Architecture, manifest.Length, manifest.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, payload, signature) {
		t.Fatal("manifest signature did not verify")
	}
}

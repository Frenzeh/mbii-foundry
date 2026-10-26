package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	manifestPath := artifactPath + ".manifest.json"
	if err := os.WriteFile(manifestPath, []byte("previous manifest"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(manifestPath, 0600); err != nil {
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
	info, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("ordinary manifest replacement changed its existing mode: %o", info.Mode().Perm())
	}
}

func TestGenerateCreatesPrivateMatchingKey(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout bytes.Buffer
	if err := run([]string{"-generate"}, &stdout); err != nil {
		t.Fatal(err)
	}
	privateKey, err := readCanonicalKey("private_key.hex", ed25519.PrivateKeySize, "private key")
	if err != nil {
		t.Fatal(err)
	}
	canonical := ed25519.NewKeyFromSeed(privateKey[:ed25519.SeedSize])
	if !bytes.Equal(privateKey, canonical) {
		t.Fatal("generated private key is internally inconsistent")
	}
	publicLine, _, ok := strings.Cut(stdout.String(), "\n")
	if !ok || !strings.HasPrefix(publicLine, "Public Key: ") {
		t.Fatal("generated public key was not reported")
	}
	publicKey, err := hex.DecodeString(strings.TrimPrefix(publicLine, "Public Key: "))
	if err != nil || !bytes.Equal(publicKey, privateKey[ed25519.SeedSize:]) {
		t.Fatal("reported public key does not match the persisted private key")
	}
	info, err := os.Stat("private_key.hex")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("generated private key is not owner-only: %o", info.Mode().Perm())
	}
}

func TestGenerateNeverOverwritesExistingPrivateKey(t *testing.T) {
	for _, kind := range []string{"permissive-file", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			t.Chdir(t.TempDir())
			original := []byte("existing synthetic key fixture")
			switch kind {
			case "permissive-file":
				if err := os.WriteFile("private_key.hex", original, 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod("private_key.hex", 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.WriteFile("existing.hex", original, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("existing.hex", "private_key.hex"); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case "directory":
				if err := os.Mkdir("private_key.hex", 0700); err != nil {
					t.Fatal(err)
				}
			}
			var stdout bytes.Buffer
			if err := run([]string{"-generate"}, &stdout); err == nil {
				t.Fatal("generation accepted an existing private-key output")
			}
			if stdout.Len() != 0 {
				t.Fatal("failed private-key creation reported an unpublished public key or success")
			}
			if kind == "directory" {
				info, err := os.Stat("private_key.hex")
				if err != nil || !info.IsDir() {
					t.Fatal("generation changed the existing output directory")
				}
				return
			}
			retained, err := os.ReadFile("private_key.hex")
			if err != nil || !bytes.Equal(retained, original) {
				t.Fatal("generation replaced existing private-key data")
			}
			info, err := os.Lstat("private_key.hex")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" && info.Mode()&os.ModeSymlink == 0 {
				t.Fatal("generation replaced the existing private-key link")
			}
			if kind == "permissive-file" && runtime.GOOS != "windows" && info.Mode().Perm() != 0644 {
				t.Fatal("failed generation changed the existing file's mode")
			}
		})
	}
}

type failingSignerOutput struct {
	err error
}

func (w failingSignerOutput) Write([]byte) (int, error) {
	return 0, w.err
}

func TestGenerateOutputFailureReportsRetainedPrivateKey(t *testing.T) {
	t.Chdir(t.TempDir())
	err := run([]string{"-generate"}, failingSignerOutput{err: io.ErrClosedPipe})
	if !errors.Is(err, io.ErrClosedPipe) || !strings.Contains(err.Error(), "saved to private_key.hex") {
		t.Fatalf("output failure did not honestly report the retained private key: %v", err)
	}
	privateKey, err := readCanonicalKey("private_key.hex", ed25519.PrivateKeySize, "private key")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(privateKey, ed25519.NewKeyFromSeed(privateKey[:ed25519.SeedSize])) {
		t.Fatal("output failure left an incomplete private key")
	}
	before, err := os.ReadFile("private_key.hex")
	if err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-generate"}, &bytes.Buffer{}); err == nil {
		t.Fatal("retry after output failure silently replaced the retained key")
	}
	after, err := os.ReadFile("private_key.hex")
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("retry changed the retained private key")
	}
}

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Frenzeh/mbii-foundry/safeio"
	"github.com/Frenzeh/mbii-foundry/updatemanifest"
)

type Manifest struct {
	Version      string `json:"version"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Length       int64  `json:"length"`
	Digest       string `json:"digest"`
	Signature    string `json:"signature"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("signer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	generate := flags.Bool("generate", false, "Generate new keypair")
	sign := flags.String("sign", "", "File to sign")
	verifyKeypair := flags.Bool("verify-keypair", false, "Verify that private and public key files match")
	keyFile := flags.String("key", "", "Private key file (canonical lowercase hex)")
	publicKeyFile := flags.String("public-key-file", "", "Public key file (canonical lowercase hex)")
	version := flags.String("version", "", "Version string")
	platform := flags.String("platform", "", "Platform string")
	arch := flags.String("arch", "", "Architecture string")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	modes := 0
	if *generate {
		modes++
	}
	if *sign != "" {
		modes++
	}
	if *verifyKeypair {
		modes++
	}
	if modes != 1 {
		return errors.New("choose exactly one of -generate, -sign, or -verify-keypair")
	}

	if *generate {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "Public Key: %x\n", pub); err != nil {
			return fmt.Errorf("write public key: %w", err)
		}
		if err := safeio.WriteFile("private_key.hex", []byte(hex.EncodeToString(priv)), 0600); err != nil {
			return fmt.Errorf("write private key: %w", err)
		}
		if _, err := fmt.Fprintln(stdout, "Private key written to private_key.hex"); err != nil {
			return fmt.Errorf("write status: %w", err)
		}
		return nil
	}

	if *keyFile == "" {
		return errors.New("missing required flag: -key")
	}
	privateKey, err := readCanonicalKey(*keyFile, ed25519.PrivateKeySize, "private key")
	if err != nil {
		return err
	}
	canonicalPrivateKey := ed25519.NewKeyFromSeed(privateKey[:ed25519.SeedSize])
	if subtle.ConstantTimeCompare(privateKey, canonicalPrivateKey) != 1 {
		return errors.New("private key is internally inconsistent: public half does not match its seed")
	}
	privateKey = canonicalPrivateKey

	if *verifyKeypair {
		if *publicKeyFile == "" {
			return errors.New("missing required flag: -public-key-file")
		}
		publicKey, err := readCanonicalKey(*publicKeyFile, ed25519.PublicKeySize, "public key")
		if err != nil {
			return err
		}
		derived := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey)
		if subtle.ConstantTimeCompare(derived, publicKey) != 1 {
			return errors.New("publisher public key does not match publisher private key")
		}
		if _, err := fmt.Fprintln(stdout, "Publisher keypair verified"); err != nil {
			return fmt.Errorf("write status: %w", err)
		}
		return nil
	}

	if *version == "" || *platform == "" || *arch == "" {
		return errors.New("missing required flags: -version, -platform, -arch")
	}
	length, digest, err := hashFile(*sign)
	if err != nil {
		return err
	}
	payload, err := updatemanifest.Payload(*version, *platform, *arch, length, digest)
	if err != nil {
		return fmt.Errorf("build manifest payload: %w", err)
	}
	manifest := Manifest{
		Version:      *version,
		Platform:     *platform,
		Architecture: *arch,
		Length:       length,
		Digest:       digest,
		Signature:    hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(privateKey), payload)),
	}
	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := safeio.WriteFile(*sign+".manifest.json", out, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, "Created manifest at "+*sign+".manifest.json"); err != nil {
		return fmt.Errorf("write status: %w", err)
	}
	return nil
}

func readCanonicalKey(path string, byteLength int, description string) ([]byte, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", description, err)
	}
	if len(encoded) != byteLength*2 {
		return nil, fmt.Errorf("invalid %s length: got %d hex characters, want %d", description, len(encoded), byteLength*2)
	}
	decoded := make([]byte, byteLength)
	if _, err := hex.Decode(decoded, encoded); err != nil {
		return nil, fmt.Errorf("decode %s hex: %w", description, err)
	}
	if hex.EncodeToString(decoded) != string(encoded) {
		return nil, fmt.Errorf("%s must be exact canonical lowercase hex with no surrounding whitespace", description)
	}
	return decoded, nil
}

func hashFile(path string) (length int64, digest string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", fmt.Errorf("open file to sign: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		return 0, "", errors.Join(
			fmt.Errorf("stat file: %w", err),
			wrapSignerError("close file to sign", f.Close()),
		)
	}
	if !info.Mode().IsRegular() {
		return 0, "", errors.Join(
			fmt.Errorf("file to sign is not regular: %s", path),
			wrapSignerError("close file to sign", f.Close()),
		)
	}
	if info.Size() < 0 || info.Size() > updatemanifest.MaxArtifactBytes {
		return 0, "", errors.Join(
			fmt.Errorf("file to sign has unsupported size %d", info.Size()),
			wrapSignerError("close file to sign", f.Close()),
		)
	}

	hasher := sha256.New()
	hashed, copyErr := io.Copy(hasher, io.LimitReader(f, updatemanifest.MaxArtifactBytes+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		return 0, "", errors.Join(
			wrapSignerError("hash file", copyErr),
			wrapSignerError("close file to sign", closeErr),
		)
	}
	if hashed != info.Size() {
		return 0, "", fmt.Errorf("file size changed while signing: stat reported %d bytes, hashed %d", info.Size(), hashed)
	}
	return hashed, hex.EncodeToString(hasher.Sum(nil)), nil
}

func wrapSignerError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

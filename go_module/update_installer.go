package main

// Cross-platform update-installer glue.
//
// InstallUpdate is the single entry point called by the Home-screen
// banner. The platform-specific heavy lifting lives in
// update_installer_{darwin,linux,windows}.go — this file just holds
// the shared download helper and the small error type the UI uses
// to decide whether to show a toast vs. open the release page.

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
)

// ErrUpdateNotSupported is returned by platforms where the auto-
// installer can't run (e.g. Windows, where SmartScreen blocks
// swapping unsigned binaries). The UI falls back to opening the
// release page in the user's browser.
var ErrUpdateNotSupported = errors.New("auto-update not supported on this platform")

// UpdateProgress is passed to InstallUpdate's progress callback so
// the banner can render a "Downloading… 34%" state without the
// caller having to marshal bytes themselves.
type UpdateProgress struct {
	Stage   string  // "downloading", "extracting", "installing", "relaunching"
	Percent float64 // 0-100, negative if unknown
	Message string  // human-readable status
}

// InstallUpdate downloads and installs the release asset for this
// platform. On success it exits the process (after relaunching the
// new binary) so the caller should treat a nil return as "process
// is about to die, don't touch the UI."
//
// progress is optional; pass nil to skip progress reporting.
func InstallUpdate(asset *ReleaseAsset, progress func(UpdateProgress)) error {
	if asset == nil {
		return errors.New("no release asset for this platform")
	}
	return installUpdatePlatform(asset, progress)
}

// downloadToTemp fetches the asset into a temp directory and returns
// the local file path + a cleanup func the caller should defer.
func downloadToTemp(asset *ReleaseAsset, progress func(UpdateProgress)) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "foundry-update-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	localPath := filepath.Join(tmpDir, asset.Name)
	out, err := os.Create(localPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create %s: %w", localPath, err)
	}
	defer out.Close()

	req, err := http.NewRequest(http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	req.Header.Set("User-Agent", "mbii-foundry-updater/1.0")
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cleanup()
		return "", nil, fmt.Errorf("download %s: server returned %s",
			asset.Name, resp.Status)
	}

	total := asset.Size
	if total <= 0 {
		total = resp.ContentLength
	}

	r := &progressReader{
		r:     resp.Body,
		total: total,
		cb: func(read int64) {
			if progress == nil || total <= 0 {
				return
			}
			progress(UpdateProgress{
				Stage:   "downloading",
				Percent: 100 * float64(read) / float64(total),
				Message: fmt.Sprintf("Downloading %s…", asset.Name),
			})
		},
	}
	if _, err := io.Copy(out, r); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("save %s: %w", asset.Name, err)
	}
	return localPath, cleanup, nil
}

// progressReader wraps an io.Reader with a byte-count callback.
// Throttled to one callback per ~256KB so the UI isn't hammered.
type progressReader struct {
	r     io.Reader
	total int64
	read  int64
	last  int64
	cb    func(int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.cb != nil && (p.read-p.last > 256<<10 || err == io.EOF) {
		p.last = p.read
		p.cb(p.read)
	}
	return n, err
}

// relaunchAndExit starts a fresh instance of the given binary and
// exits the current one. Used by the platform installers after the
// on-disk swap completes.
//
// We give the new process ~1 second of head start before fyne.Do-ing
// the exit so the window manager handoff is smooth — calling
// os.Exit immediately has, on both Mac and Linux, occasionally left
// the new process fighting for focus against a disappearing ghost.
func relaunchAndExit(binaryPath string, args ...string) error {
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("relaunch %s: %w", binaryPath, err)
	}
	// Detach so we don't block if the parent lingers.
	go func() { _ = cmd.Process.Release() }()

	go func() {
		time.Sleep(700 * time.Millisecond)
		fyne.Do(func() {
			LogInfo("Relaunching into %s", binaryPath)
			os.Exit(0)
		})
	}()
	return nil
}

// appBundleContainingExe walks up from the current executable path
// looking for the enclosing ".app" bundle, if any. Returns "" when
// the binary isn't inside a bundle (dev-mode / raw binary on Linux).
// Used by the Mac installer to decide between bundle-swap and raw-
// binary swap.
func appBundleContainingExe() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	for i := 0; i < 8 && dir != "/" && dir != "."; i++ {
		if filepath.Ext(dir) == ".app" {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

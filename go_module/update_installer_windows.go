//go:build windows

package main

// Windows stub — no auto-install. SmartScreen blocks unsigned
// replacement binaries with "Windows protected your PC" and there's
// no equivalent to macOS's xattr-stripping escape hatch. The UI
// falls back to opening the release page in the user's browser, so
// they can re-download and run through the install dialog again.

func installUpdatePlatform(asset *ReleaseAsset, progress func(UpdateProgress)) error {
	return ErrUpdateNotSupported
}

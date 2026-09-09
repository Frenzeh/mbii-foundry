//go:build windows

package main

// Windows stub — no auto-install. SmartScreen blocks unsigned
// replacement binaries with "Windows protected your PC", and unlike
// macOS there is no structural-check path that would make a swap
// meaningful: without Authenticode signing credentials there is no
// publisher identity to verify, so the safe default is to fail
// closed. The UI falls back to opening the release page in the
// user's browser, so they can re-download and run through the
// install dialog again.

func installUpdatePlatform(asset *ReleaseAsset, manifest UpdateManifest, progress func(UpdateProgress)) error {
	return ErrUpdateNotSupported
}

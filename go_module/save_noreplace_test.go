//go:build darwin || linux || windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRenameNoReplacePreservesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	destination := filepath.Join(dir, "destination")
	if err := os.WriteFile(source, []byte("source bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("destination bytes"), 0600); err != nil {
		t.Fatal(err)
	}

	err := renameNoReplace(source, destination)
	if err == nil || !os.IsExist(err) {
		t.Fatalf("existing destination must produce an existence error, got %v", err)
	}
	assertFileBytes(t, source, "source bytes")
	assertFileBytes(t, destination, "destination bytes")
}

func TestRenameNoReplacePublishesToAbsentDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	destination := filepath.Join(dir, "destination")
	if err := os.WriteFile(source, []byte("candidate"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := renameNoReplace(source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists after rename: %v", err)
	}
	assertFileBytes(t, destination, "candidate")
}

func TestReviewedSaveFailureReportsPublicationOutcome(t *testing.T) {
	published := reviewedSaveCommitFailure("reviewed.veh", &savePublicationError{
		Published: true,
		Err:       errors.New("backup proof failed"),
	})
	if !saveWasPublished(published) || !strings.Contains(published.Error(), "Candidate was published") ||
		!strings.Contains(published.Error(), "editor remains dirty") {
		t.Fatalf("published failure was misreported: %v", published)
	}

	notPublished := reviewedSaveCommitFailure("reviewed.veh", &savePublicationError{
		Published: false,
		Err:       errors.New("destination changed"),
	})
	if saveWasPublished(notPublished) || !strings.Contains(notPublished.Error(), "Failed to save") {
		t.Fatalf("pre-publication failure was misreported: %v", notPublished)
	}
}

func TestRecoveryDiagnosticsUseUnescapedPathsAndPreserveCause(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, `document\reviewed.sab`)
	holdPath := filepath.Join(root, `.document\reviewed.sab.reviewed-hold-test`)
	backupPath := filepath.Join(root, `backups\reviewed.sab`)
	cause := errors.New("publication failed")

	err := restoreSaveHoldAfterFailure(path, holdPath, backupPath, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("recovery diagnostic lost the publication failure: %v", err)
	}
	for _, recoveryPath := range []string{path, holdPath, backupPath} {
		if !strings.Contains(err.Error(), recoveryPath) {
			t.Fatalf("recovery diagnostic escaped path %q: %v", recoveryPath, err)
		}
	}
}

func TestReviewedPublicationPreservesWriterAfterDestinationMove(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "reviewed.sab")
	if err := os.WriteFile(path, []byte("reviewed original"), 0640); err != nil {
		t.Fatal(err)
	}
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}

	var holdPath string
	result, err := publishEditorCandidateReviewedWithHooks(
		NewFileManager(filepath.Join(root, "config")),
		path,
		"approved candidate",
		reviewed,
		savePublicationHooks{
			beforeCandidatePublish: func(hold, backup string) {
				holdPath = hold
				if hold == "" || backup == "" {
					t.Fatal("existing destination publication did not expose recovery paths")
				}
				if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
					t.Fatalf("destination must be absent after its move to hold: %v", statErr)
				}
				assertFileBytes(t, hold, reviewed.Bytes)
				if writeErr := os.WriteFile(path, []byte("external winner"), 0600); writeErr != nil {
					t.Fatalf("inject last-window writer: %v", writeErr)
				}
			},
		},
	)
	backupPath := result.BackupPath
	if result.Published || saveWasPublished(err) {
		t.Fatal("failed no-replace publication incorrectly reported candidate installation")
	}
	if err == nil {
		t.Fatal("last-window writer must make reviewed publication fail")
	}
	assertFileBytes(t, path, "external winner")
	assertFileBytes(t, holdPath, reviewed.Bytes)
	assertExactSnapshot(t, backupPath, reviewed)
	for _, recoveryPath := range []string{path, holdPath, backupPath} {
		if !strings.Contains(err.Error(), recoveryPath) {
			t.Fatalf("error is missing actionable recovery path %q: %v", recoveryPath, err)
		}
	}
	assertNoCandidateTemps(t, root, path)
}

func TestReviewedNewDestinationPreservesLastWindowWriter(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "new.sab")
	result, err := publishEditorCandidateReviewedWithHooks(
		NewFileManager(filepath.Join(root, "config")),
		path,
		"approved candidate",
		saveDestinationSnapshot{},
		savePublicationHooks{
			beforeCandidatePublish: func(hold, backup string) {
				if hold != "" || backup != "" {
					t.Fatalf("new destination unexpectedly has recovery paths: hold=%q backup=%q", hold, backup)
				}
				if writeErr := os.WriteFile(path, []byte("external winner"), 0600); writeErr != nil {
					t.Fatalf("inject last-window writer: %v", writeErr)
				}
			},
		},
	)
	if result.Published || saveWasPublished(err) {
		t.Fatal("failed new-file publication incorrectly reported candidate installation")
	}
	if err == nil {
		t.Fatal("last-window creator must make reviewed publication fail")
	}
	assertFileBytes(t, path, "external winner")
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "preserved") {
		t.Fatalf("error is not actionable: %v", err)
	}
	assertNoCandidateTemps(t, root, path)
}

func TestReviewedPublicationKeepsHoldUntilFinalBackupProof(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "reviewed.veh")
	if err := os.WriteFile(path, []byte("reviewed original"), 0640); err != nil {
		t.Fatal(err)
	}
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}

	var holdPath string
	result, err := publishEditorCandidateReviewedWithHooks(
		NewFileManager(filepath.Join(root, "config")),
		path,
		"approved candidate",
		reviewed,
		savePublicationHooks{
			afterCandidatePublish: func(hold, backup string) {
				holdPath = hold
				if writeErr := os.WriteFile(backup, []byte("corrupt backup"), 0600); writeErr != nil {
					t.Fatalf("corrupt backup after publish: %v", writeErr)
				}
			},
		},
	)
	backupPath := result.BackupPath
	if !result.Published || !saveWasPublished(err) {
		t.Fatalf("post-publication failure lost publication outcome: result=%+v err=%v", result, err)
	}
	if err == nil {
		t.Fatal("failed final backup proof must be reported")
	}
	assertFileBytes(t, path, "approved candidate")
	assertFileBytes(t, holdPath, reviewed.Bytes)
	if !strings.Contains(err.Error(), holdPath) || !strings.Contains(err.Error(), backupPath) {
		t.Fatalf("error is missing recovery paths: %v", err)
	}
	assertNoCandidateTemps(t, root, path)
}

func TestReviewedPublicationRejectsModeMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission modes")
	}
	root := t.TempDir()
	path := filepath.Join(root, "reviewed.siege")
	if err := os.WriteFile(path, []byte("reviewed original"), 0640); err != nil {
		t.Fatal(err)
	}
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}

	backupPath, err := publishEditorCandidateReviewedWithHook(
		NewFileManager(filepath.Join(root, "config")),
		path,
		"approved candidate",
		reviewed,
		func() {
			if chmodErr := os.Chmod(path, 0600); chmodErr != nil {
				t.Fatal(chmodErr)
			}
		},
	)
	if err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("mode mutation must abort publication, got %v", err)
	}
	assertFileBytes(t, path, reviewed.Bytes)
	current, statErr := snapshotSaveDestination(path)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if current.Mode != 0600 {
		t.Fatalf("external mode mutation was not preserved: got %04o", current.Mode)
	}
	assertExactSnapshot(t, backupPath, reviewed)
	assertNoCandidateTemps(t, root, path)
}

func TestReviewedPublicationSuccessRemovesHoldAfterBackupProof(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "reviewed.mbch")
	if err := os.WriteFile(path, []byte("reviewed original"), 0640); err != nil {
		t.Fatal(err)
	}
	reviewed, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatal(err)
	}

	backupPath, err := publishEditorCandidateReviewedWithBackup(
		NewFileManager(filepath.Join(root, "config")),
		path,
		"approved candidate",
		reviewed,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, path, "approved candidate")
	assertExactSnapshot(t, backupPath, reviewed)
	holds, globErr := filepath.Glob(filepath.Join(root, ".reviewed.mbch.reviewed-hold-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(holds) != 0 {
		t.Fatalf("successful publication left reviewed holds: %v", holds)
	}
	assertNoCandidateTemps(t, root, path)
}

func assertFileBytes(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%q bytes = %q, want %q", path, got, want)
	}
}

func assertExactSnapshot(t *testing.T, path string, want saveDestinationSnapshot) {
	t.Helper()
	got, err := snapshotSaveDestination(path)
	if err != nil {
		t.Fatalf("snapshot %q: %v", path, err)
	}
	if got != want {
		t.Fatalf("snapshot %q = %+v, want %+v", path, got, want)
	}
}

func assertNoCandidateTemps(t *testing.T, root, path string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, filepath.Base(path)+".*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("publication left candidate staging files: %v", matches)
	}
}

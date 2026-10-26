package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"github.com/Frenzeh/mbii-foundry/safeio"
)

// Configuration constants
const (
	MaxRecentFiles    = 20
	MaxBackupsPerFile = 5
	BackupDir         = "backups"
	ConfigDir         = "config"
	RecentFilesFile   = "recent_files.json"
)

// Editor interface for MDI
type Editor interface {
	GetContent() fyne.CanvasObject
	LoadFile(path string) error
	SaveFile(path string) error
	SaveToWriter(w io.Writer) error
	GetCurrentPath() string
	SetCurrentPath(path string)
	SetOnHover(func(string, string))
	SetAssetBrowser(*AssetBrowser)
	SetHolocronClient(*HolocronClient)

	IsDirty() bool
	MarkClean()
	SetOnDirtyChanged(func(bool))

	Validate() []string
}

type SourceProvider interface {
	GenerateSource() string
	SetOnSourceChanged(func())
}

// RecentFile stores info about a recently accessed file
type RecentFile struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	AccessedAt time.Time `json:"accessed_at"`
}

// FileManager handles file operations with backup and recent files support
type FileManager struct {
	baseDir     string
	recentFiles []RecentFile
}

// NewFileManager creates a new file manager
// baseDir should be the user configuration directory
func NewFileManager(baseDir string) *FileManager {
	fm := &FileManager{
		baseDir:     baseDir,
		recentFiles: []RecentFile{},
	}
	if baseDir != "" {
		fm.loadRecentFiles()
	}
	return fm
}

func (fm *FileManager) getConfigPath() string {
	return fm.baseDir
}

func (fm *FileManager) getBackupPath() string {
	if fm.baseDir == "" {
		return ""
	}
	return filepath.Join(fm.baseDir, BackupDir)
}

func ensureDir(path string) error {
	if path == "" {
		return fmt.Errorf("operation unavailable: no config directory")
	}
	return os.MkdirAll(path, 0755)
}

func (fm *FileManager) loadRecentFiles() {
	if fm.baseDir == "" {
		return
	}
	configPath := filepath.Join(fm.getConfigPath(), RecentFilesFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &fm.recentFiles)
}

func (fm *FileManager) saveRecentFiles() error {
	if fm.baseDir == "" {
		// Memory only
		return nil
	}
	if err := ensureDir(fm.getConfigPath()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(fm.recentFiles, "", "  ")
	if err != nil {
		return err
	}
	// Use safeio to safely write the recent files list
	configPath := filepath.Join(fm.getConfigPath(), RecentFilesFile)
	return safeio.WriteFile(configPath, data, 0644)
}

func (fm *FileManager) AddRecentFile(path string) {
	if path == "" {
		return
	}
	name := filepath.Base(path)
	filtered := []RecentFile{}
	for _, rf := range fm.recentFiles {
		if rf.Path != path {
			filtered = append(filtered, rf)
		}
	}
	fm.recentFiles = filtered
	fm.recentFiles = append([]RecentFile{{
		Path:       path,
		Name:       name,
		AccessedAt: time.Now(),
	}}, fm.recentFiles...)

	if len(fm.recentFiles) > MaxRecentFiles {
		fm.recentFiles = fm.recentFiles[:MaxRecentFiles]
	}
	fm.saveRecentFiles()
}

func (fm *FileManager) GetRecentFiles() []RecentFile {
	return fm.recentFiles
}

func (fm *FileManager) getPathHash(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	hash := sha256.Sum256([]byte(abs))
	return fmt.Sprintf("%x", hash[:8])
}

func (fm *FileManager) CreateBackup(path string) (string, error) {
	if fm.baseDir == "" {
		return "", fmt.Errorf("backup unavailable: no config directory configured")
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if err := ensureDir(fm.getBackupPath()); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	baseName := filepath.Base(path)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	pathHash := fm.getPathHash(path)

	timestamp := time.Now().Format("20060102_150405")

	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	randHex := hex.EncodeToString(randBytes)

	baseBackupName := fmt.Sprintf("%s_%s_%s_%s", nameWithoutExt, pathHash, timestamp, randHex)

	// Create an exclusive staging file
	tmpF, err := os.CreateTemp(fm.getBackupPath(), "staging_*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create backup staging file: %w", err)
	}
	tmpName := tmpF.Name()

	// Copy data to the staging file
	srcFile, err := os.Open(path)
	if err != nil {
		tmpF.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to open source for backup: %w", err)
	}
	defer srcFile.Close()

	if _, err := io.Copy(tmpF, srcFile); err != nil {
		tmpF.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to copy data to backup: %w", err)
	}

	if err := tmpF.Chmod(info.Mode().Perm()); err != nil {
		tmpF.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to chmod backup: %w", err)
	}

	if err := tmpF.Sync(); err != nil {
		tmpF.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to sync backup: %w", err)
	}

	if err := tmpF.Close(); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to close backup: %w", err)
	}

	// Publish to final path using strict no-replace allocation
	finalPath := filepath.Join(fm.getBackupPath(), baseBackupName+ext)
	counter := 1
	for {
		err := os.Link(tmpName, finalPath)
		if err == nil {
			os.Remove(tmpName)
			break
		}
		// Explicitly check for exists using standard error checks
		if os.IsExist(err) || errors.Is(err, fs.ErrExist) {
			finalPath = filepath.Join(fm.getBackupPath(), fmt.Sprintf("%s_%d%s", baseBackupName, counter, ext))
			counter++
			continue
		}

		// For unsupported filesystems or other link errors, fail cleanly
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to finalize backup link: %w", err)
	}

	fm.cleanupOldBackups(path)
	return finalPath, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	if src == "" || dst == "" {
		return fmt.Errorf("copy requires nonempty source and destination paths")
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	return safeio.AtomicWrite(dst, mode, func(w io.Writer) error {
		_, err := io.Copy(w, srcFile)
		return err
	})
}

// isBackupForPath checks if a backup file corresponds to the given original path.
func (fm *FileManager) isBackupForPath(backupName, originalPath string) bool {
	baseName := filepath.Base(originalPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	pathHash := fm.getPathHash(originalPath)

	// New pattern: nameWithoutExt_hash_...
	return strings.HasPrefix(backupName, nameWithoutExt+"_"+pathHash+"_") && strings.HasSuffix(backupName, ext)
}

func (fm *FileManager) cleanupOldBackups(path string) {
	matches := fm.ListBackups(path)

	if len(matches) <= MaxBackupsPerFile {
		return
	}

	// ListBackups already sorts them newest first
	for i := MaxBackupsPerFile; i < len(matches); i++ {
		os.Remove(matches[i])
	}
}

func (fm *FileManager) ListBackups(path string) []string {
	if fm.baseDir == "" {
		return nil
	}
	backupDir := fm.getBackupPath()
	var matches []string

	if path == "" {
		// Return all backups if no path provided
		ext := ".mbch"
		matches, _ = filepath.Glob(filepath.Join(backupDir, "*"+ext))
	} else {
		files, err := os.ReadDir(backupDir)
		if err == nil {
			for _, file := range files {
				if file.IsDir() {
					continue
				}
				if fm.isBackupForPath(file.Name(), path) {
					matches = append(matches, filepath.Join(backupDir, file.Name()))
				}
			}
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		infoI, errI := os.Stat(matches[i])
		infoJ, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return matches
}

func (fm *FileManager) RestoreBackup(backupPath, destPath string) error {
	if backupPath == "" || destPath == "" {
		return fmt.Errorf("restore requires nonempty backup and destination paths")
	}
	info, err := os.Stat(destPath)
	mode := os.FileMode(0644)
	if err == nil {
		mode = info.Mode().Perm()
	}
	return copyFile(backupPath, destPath, mode)
}

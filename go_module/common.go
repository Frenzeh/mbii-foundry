package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
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
	fm.loadRecentFiles()
	return fm
}

func (fm *FileManager) getConfigPath() string {
	return fm.baseDir
}

func (fm *FileManager) getBackupPath() string {
	return filepath.Join(fm.baseDir, BackupDir)
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func (fm *FileManager) loadRecentFiles() {
	configPath := filepath.Join(fm.getConfigPath(), RecentFilesFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &fm.recentFiles)
}

func (fm *FileManager) saveRecentFiles() error {
	if err := ensureDir(fm.getConfigPath()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(fm.recentFiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(fm.getConfigPath(), RecentFilesFile), data, 0644)
}

func (fm *FileManager) AddRecentFile(path string) {
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

func (fm *FileManager) CreateBackup(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	}
	if err := ensureDir(fm.getBackupPath()); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	baseName := filepath.Base(path)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)
	backupPath := filepath.Join(fm.getBackupPath(), backupName)

	if err := copyFile(path, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}
	fm.cleanupOldBackups(nameWithoutExt, ext)
	return backupPath, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (fm *FileManager) cleanupOldBackups(baseName, ext string) {
	backupDir := fm.getBackupPath()
	pattern := baseName + "_*" + ext
	matches, err := filepath.Glob(filepath.Join(backupDir, pattern))
	if err != nil || len(matches) <= MaxBackupsPerFile {
		return
	}
	sort.Slice(matches, func(i, j int) bool {
		infoI, _ := os.Stat(matches[i])
		infoJ, _ := os.Stat(matches[j])
		if infoI == nil || infoJ == nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
	for i := MaxBackupsPerFile; i < len(matches); i++ {
		os.Remove(matches[i])
	}
}

func (fm *FileManager) ListBackups(baseName string) []string {
	backupDir := fm.getBackupPath()
	ext := ".mbch"

	if baseName == "" {
		matches, _ := filepath.Glob(filepath.Join(backupDir, "*"+ext))
		return matches
	}

	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	pattern := nameWithoutExt + "_*" + ext
	matches, _ := filepath.Glob(filepath.Join(backupDir, pattern))

	sort.Slice(matches, func(i, j int) bool {
		infoI, _ := os.Stat(matches[i])
		infoJ, _ := os.Stat(matches[j])
		if infoI == nil || infoJ == nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return matches
}

func (fm *FileManager) RestoreBackup(backupPath, destPath string) error {
	return copyFile(backupPath, destPath)
}

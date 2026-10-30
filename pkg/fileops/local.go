package fileops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type FileItem struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`    // string format like drwxr-xr-x or -rw-r--r--
	ModeOct string    `json:"modeOct"` // e.g. "0755" or "0644"
	ModTime time.Time `json:"modTime"`
}

type LocalFileManager struct{}

func NewLocalFileManager() *LocalFileManager {
	return &LocalFileManager{}
}

func (l *LocalFileManager) ListDir(dirPath string) (string, []*FileItem, error) {
	if dirPath == "" || dirPath == "." {
		var err error
		dirPath, err = os.Getwd()
		if err != nil {
			return "", nil, err
		}
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return absPath, nil, err
	}

	var items []*FileItem
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		mode := info.Mode()
		oct := fmt.Sprintf("%04o", mode.Perm())

		items = append(items, &FileItem{
			Name:    info.Name(),
			Path:    filepath.Join(absPath, info.Name()),
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			Mode:    mode.String(),
			ModeOct: oct,
			ModTime: info.ModTime(),
		})
	}

	return absPath, items, nil
}

func (l *LocalFileManager) CreateFolder(dirPath string) error {
	return os.MkdirAll(dirPath, 0755)
}

func (l *LocalFileManager) CreateFile(filePath string) error {
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

func (l *LocalFileManager) Delete(itemPath string) error {
	return os.RemoveAll(itemPath)
}

func (l *LocalFileManager) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (l *LocalFileManager) Chmod(itemPath string, modeOct string) error {
	var mode uint32
	_, err := fmt.Sscanf(modeOct, "%o", &mode)
	if err != nil {
		return err
	}
	return os.Chmod(itemPath, os.FileMode(mode))
}

func (l *LocalFileManager) ReadFileText(filePath string, maxBytes int64) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("file size (%d bytes) exceeds maximum allowable limit (%d bytes)", info.Size(), maxBytes)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (l *LocalFileManager) WriteFileText(filePath string, content string) error {
	return os.WriteFile(filePath, []byte(content), 0644)
}

func (l *LocalFileManager) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

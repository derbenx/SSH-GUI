package main

import (
	"context"
	"fmt"
	"sync"

	"app/pkg/fileops"
	"app/pkg/sshclient"
	"app/pkg/storage"
)

type App struct {
	ctx        context.Context
	storage    *storage.StorageManager
	localFM    *fileops.LocalFileManager
	sshSession *sshclient.SSHSession
	termBuf    []byte
	mu         sync.Mutex
}

func NewApp() *App {
	sm, _ := storage.NewStorageManager("")
	return &App{
		storage:    sm,
		localFM:    fileops.NewLocalFileManager(),
		sshSession: sshclient.NewSSHSession(),
		termBuf:    make([]byte, 8192),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Storage / Session Manager Bindings
func (a *App) HasMasterPassword() bool {
	if a.storage == nil {
		return false
	}
	return a.storage.HasMasterPassword()
}

func (a *App) SetMasterPassword(password string) error {
	return a.storage.SetMasterPassword(password)
}

func (a *App) VerifyMasterPassword(password string) bool {
	return a.storage.VerifyAndUnlock(password)
}

func (a *App) GetProfiles() ([]*storage.SessionProfile, error) {
	return a.storage.GetProfiles()
}

func (a *App) SaveProfile(profile *storage.SessionProfile) error {
	return a.storage.SaveProfile(profile)
}

func (a *App) DeleteProfile(id string) error {
	return a.storage.DeleteProfile(id)
}

// Local File Operations
type ListDirResult struct {
	CurrentPath string              `json:"currentPath"`
	Items       []*fileops.FileItem `json:"items"`
}

func (a *App) ListLocalDir(dirPath string) (*ListDirResult, error) {
	path, items, err := a.localFM.ListDir(dirPath)
	if err != nil {
		return nil, err
	}
	return &ListDirResult{CurrentPath: path, Items: items}, nil
}

func (a *App) CreateLocalFolder(dirPath string) error {
	return a.localFM.CreateFolder(dirPath)
}

func (a *App) CreateLocalFile(filePath string) error {
	return a.localFM.CreateFile(filePath)
}

func (a *App) DeleteLocal(itemPath string) error {
	return a.localFM.Delete(itemPath)
}

func (a *App) RenameLocal(oldPath, newPath string) error {
	return a.localFM.Rename(oldPath, newPath)
}

func (a *App) ChmodLocal(itemPath string, modeOct string) error {
	return a.localFM.Chmod(itemPath, modeOct)
}

func (a *App) ReadLocalFileText(filePath string) (string, error) {
	return a.localFM.ReadFileText(filePath, 2*1024*1024) // 2MB max
}

func (a *App) WriteLocalFileText(filePath string, content string) error {
	return a.localFM.WriteFileText(filePath, content)
}

func (a *App) CopyLocalFile(src, dst string) error {
	return a.localFM.CopyFile(src, dst)
}

// Remote / SSH Operations
func (a *App) ConnectSSH(host string, port int, user, authType, pass, keyPath, keyPass string) error {
	return a.sshSession.Connect(host, port, user, authType, pass, keyPath, keyPass)
}

func (a *App) DisconnectSSH() {
	a.sshSession.Close()
}

func (a *App) IsSSHConnected() bool {
	return a.sshSession.IsConnected()
}

func (a *App) SetSSHRootMode(isRoot bool) {
	a.sshSession.SetRootMode(isRoot)
}

func (a *App) ListRemoteDir(remotePath string) (*ListDirResult, error) {
	path, items, err := a.sshSession.ListRemoteDir(remotePath)
	if err != nil {
		return nil, err
	}
	return &ListDirResult{CurrentPath: path, Items: items}, nil
}

func (a *App) ExecuteRemoteCmd(cmd string) (string, error) {
	return a.sshSession.ExecuteRemoteCommand(cmd)
}

func (a *App) CreateRemoteFolder(dirPath string) error {
	cmd := fmt.Sprintf("mkdir -p %s", sshclient.QuotePath(dirPath))
	_, err := a.sshSession.ExecuteRemoteCommand(cmd)
	return err
}

func (a *App) CreateRemoteFile(filePath string) error {
	cmd := fmt.Sprintf("touch %s", sshclient.QuotePath(filePath))
	_, err := a.sshSession.ExecuteRemoteCommand(cmd)
	return err
}

func (a *App) DeleteRemote(itemPath string) error {
	cmd := fmt.Sprintf("rm -rf %s", sshclient.QuotePath(itemPath))
	_, err := a.sshSession.ExecuteRemoteCommand(cmd)
	return err
}

func (a *App) RenameRemote(oldPath, newPath string) error {
	cmd := fmt.Sprintf("mv %s %s", sshclient.QuotePath(oldPath), sshclient.QuotePath(newPath))
	_, err := a.sshSession.ExecuteRemoteCommand(cmd)
	return err
}

func (a *App) CopyRemoteFile(oldPath, newPath string) error {
	cmd := fmt.Sprintf("cp -r %s %s", sshclient.QuotePath(oldPath), sshclient.QuotePath(newPath))
	_, err := a.sshSession.ExecuteRemoteCommand(cmd)
	return err
}

func (a *App) ChmodRemote(itemPath string, modeOct string) error {
	cmd := fmt.Sprintf("chmod %s %s", modeOct, sshclient.QuotePath(itemPath))
	_, err := a.sshSession.ExecuteRemoteCommand(cmd)
	return err
}

func (a *App) ReadRemoteFileText(filePath string) (string, error) {
	return a.sshSession.ReadRemoteFile(filePath, 2*1024*1024)
}

func (a *App) WriteRemoteFileText(filePath string, content string) error {
	return a.sshSession.WriteRemoteFile(filePath, content)
}

func (a *App) UploadFile(localPath, remotePath string) error {
	return a.sshSession.UploadFile(localPath, remotePath)
}

func (a *App) DownloadFile(remotePath, localPath string) error {
	return a.sshSession.DownloadFile(remotePath, localPath)
}

func (a *App) SendTerminalInput(input string) error {
	return a.sshSession.WriteInput(input)
}

func (a *App) PollTerminalOutput() (string, error) {
	buf := make([]byte, 4096)
	n, err := a.sshSession.ReadOutput(buf)
	if err != nil || n == 0 {
		return "", err
	}
	return string(buf[:n]), nil
}

func (a *App) UpdateRemoteDirTrack(newDir string) {
	a.sshSession.UpdateCurrentDir(newDir)
}

func (a *App) GetRemoteDirTrack() string {
	return a.sshSession.GetCurrentDir()
}

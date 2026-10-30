package sshclient

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"app/pkg/fileops"

	"golang.org/x/crypto/ssh"
)

type SSHSession struct {
	mu           sync.Mutex
	client       *ssh.Client
	session      *ssh.Session
	stdin        io.WriteCloser
	stdout       io.Reader
	stderr       io.Reader
	connected    bool
	currentDir   string
	user         string
	isRoot       bool
	outputBuffer *bytes.Buffer
}

func NewSSHSession() *SSHSession {
	return &SSHSession{
		outputBuffer: new(bytes.Buffer),
		currentDir:   "~",
	}
}

func (s *SSHSession) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

func (s *SSHSession) SetRootMode(isRoot bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isRoot = isRoot
}

func (s *SSHSession) IsRootMode() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRoot
}

func (s *SSHSession) Connect(host string, port int, username, authType, password, keyPath, keyPassphrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.connected {
		s.closeLocked()
	}

	var authMethods []ssh.AuthMethod
	if authType == "key" && keyPath != "" {
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("failed to read private key file: %w", err)
		}
		var signer ssh.Signer
		if keyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(keyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(password))
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("ssh dial failed: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return fmt.Errorf("ssh new session failed: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("xterm", 80, 24, modes); err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("request pty failed: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("stdin pipe failed: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("stdout pipe failed: %w", err)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("shell failed: %w", err)
	}

	s.client = client
	s.session = session
	s.stdin = stdin
	s.stdout = stdout
	s.connected = true
	s.user = username

	// Resolve initial current directory
	pwd, err := s.runSingleCommandLocked("pwd")
	if err == nil && pwd != "" {
		s.currentDir = strings.TrimSpace(pwd)
	}

	return nil
}

func (s *SSHSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
}

func (s *SSHSession) closeLocked() {
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
	s.connected = false
	s.isRoot = false
}

func (s *SSHSession) WriteInput(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected || s.stdin == nil {
		return errors.New("not connected")
	}

	_, err := s.stdin.Write([]byte(data))
	return err
}

func (s *SSHSession) ReadOutput(buf []byte) (int, error) {
	if !s.connected || s.stdout == nil {
		return 0, io.EOF
	}
	return s.stdout.Read(buf)
}

func QuotePath(p string) string {
	return "'" + strings.ReplaceAll(p, "'", "'\\''") + "'"
}

func (s *SSHSession) runSingleCommandLocked(cmdStr string) (string, error) {
	if !s.connected || s.client == nil {
		return "", errors.New("not connected")
	}

	sess, err := s.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	var buf bytes.Buffer
	sess.Stdout = &buf

	fullCmd := cmdStr
	if s.currentDir != "" && s.currentDir != "~" {
		fullCmd = fmt.Sprintf("cd %s && %s", QuotePath(s.currentDir), cmdStr)
	}

	if s.isRoot {
		// When elevated via su, wrap execution with sudo/su runner
		fullCmd = fmt.Sprintf("sudo -n sh -c %s 2>/dev/null || sh -c %s", QuotePath(fullCmd), QuotePath(fullCmd))
	}

	err = sess.Run(fullCmd)
	return buf.String(), err
}

func (s *SSHSession) ListRemoteDir(remotePath string) (string, []*fileops.FileItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if remotePath == "" || remotePath == "." {
		remotePath = s.currentDir
	}

	targetDir := remotePath
	pwdCmd := fmt.Sprintf("cd %s 2>/dev/null && pwd", QuotePath(targetDir))
	absDir, err := s.runSingleCommandLocked(pwdCmd)
	if err == nil && strings.TrimSpace(absDir) != "" {
		targetDir = strings.TrimSpace(absDir)
		s.currentDir = targetDir
	}

	lsCmd := fmt.Sprintf("cd %s && ls -la --time-style=full-iso", QuotePath(targetDir))
	out, err := s.runSingleCommandLocked(lsCmd)
	if err != nil {
		lsCmd = fmt.Sprintf("cd %s && ls -la", QuotePath(targetDir))
		out, err = s.runSingleCommandLocked(lsCmd)
		if err != nil {
			return targetDir, nil, err
		}
	}

	items := parseLsOutput(out, targetDir)
	return targetDir, items, nil
}

func parseLsOutput(output string, baseDir string) []*fileops.FileItem {
	lines := strings.Split(output, "\n")
	var items []*fileops.FileItem

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		mode := fields[0]
		if len(mode) < 10 {
			continue
		}

		isDir := mode[0] == 'd'
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		name := strings.Join(fields[8:], " ")

		if name == "." || name == ".." {
			continue
		}

		if idx := strings.Index(name, " -> "); idx != -1 {
			name = name[:idx]
		}

		items = append(items, &fileops.FileItem{
			Name:    name,
			Path:    path.Join(baseDir, name),
			IsDir:   isDir,
			Size:    size,
			Mode:    mode,
			ModeOct: modeToOctal(mode),
			ModTime: time.Now(),
		})
	}
	return items
}

func modeToOctal(mode string) string {
	if len(mode) < 10 {
		return "0644"
	}
	perm := mode[1:10]
	var val uint32
	for i, c := range perm {
		if c != '-' {
			shift := 8 - i
			val |= (1 << shift)
		}
	}
	return fmt.Sprintf("%04o", val)
}

func (s *SSHSession) ExecuteRemoteCommand(cmdStr string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runSingleCommandLocked(cmdStr)
}

func (s *SSHSession) ReadRemoteFile(remotePath string, maxBytes int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := fmt.Sprintf("cat %s | base64 -w 0", QuotePath(remotePath))
	b64Out, err := s.runSingleCommandLocked(cmd)
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Out))
	if err != nil {
		return "", fmt.Errorf("failed to decode remote file content: %w", err)
	}

	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("remote file size (%d bytes) exceeds maximum allowed (%d bytes)", len(data), maxBytes)
	}

	return string(data), nil
}

func (s *SSHSession) WriteRemoteFile(remotePath string, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b64Content := base64.StdEncoding.EncodeToString([]byte(content))
	cmd := fmt.Sprintf("echo '%s' | base64 -d > %s", b64Content, QuotePath(remotePath))
	_, err := s.runSingleCommandLocked(cmd)
	return err
}

func (s *SSHSession) UploadFile(localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	b64 := base64.StdEncoding.EncodeToString(data)
	cmd := fmt.Sprintf("echo '%s' | base64 -d > %s", b64, QuotePath(remotePath))
	_, err = s.runSingleCommandLocked(cmd)
	return err
}

func (s *SSHSession) DownloadFile(remotePath, localPath string) error {
	s.mu.Lock()
	cmd := fmt.Sprintf("cat %s | base64 -w 0", QuotePath(remotePath))
	b64Out, err := s.runSingleCommandLocked(cmd)
	s.mu.Unlock()

	if err != nil {
		return err
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Out))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(localPath, data, 0644)
}

func (s *SSHSession) UpdateCurrentDir(newDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentDir = newDir
}

func (s *SSHSession) GetCurrentDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentDir
}

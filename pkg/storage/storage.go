package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/pbkdf2"
)

// SessionProfile represents a saved SSH connection configuration.
type SessionProfile struct {
	ID            string `json:"id"`
	Name          string `json:"name"` // Preset default e.g. user@address
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AuthType      string `json:"authType"` // "password" or "key"
	Password      string `json:"password,omitempty"`
	KeyPath       string `json:"keyPath,omitempty"`
	KeyPassphrase string `json:"keyPassphrase,omitempty"`
}

type EncryptedStore struct {
	Salt       string            `json:"salt"`
	MasterHash string            `json:"masterHash"` // SHA256 of master key
	Profiles   []*SessionProfile `json:"profiles"`
}

type StorageManager struct {
	mu           sync.Mutex
	filePath     string
	masterKey    []byte
	unlocked     bool
	store        *EncryptedStore
}

func NewStorageManager(configDir string) (*StorageManager, error) {
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			configDir = "."
		} else {
			configDir = filepath.Join(homeDir, ".putty-winscp-app")
		}
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, err
	}

	filePath := filepath.Join(configDir, "sessions.json")
	sm := &StorageManager{
		filePath: filePath,
		store: &EncryptedStore{
			Profiles: []*SessionProfile{},
		},
	}

	if err := sm.loadFromFile(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return sm, nil
}

func (sm *StorageManager) HasMasterPassword() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.store.MasterHash != ""
}

func (sm *StorageManager) IsUnlocked() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.unlocked || sm.store.MasterHash == ""
}

// SetMasterPassword initializes or changes the master password
func (sm *StorageManager) SetMasterPassword(password string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if password == "" {
		return errors.New("master password cannot be empty")
	}

	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}

	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
	h := sha256.Sum256(key)

	sm.store.Salt = base64.StdEncoding.EncodeToString(salt)
	sm.store.MasterHash = base64.StdEncoding.EncodeToString(h[:])
	sm.masterKey = key
	sm.unlocked = true

	return sm.saveToFileLocked()
}

// VerifyAndUnlock checks the provided master password and unlocks session storage
func (sm *StorageManager) VerifyAndUnlock(password string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.store.MasterHash == "" {
		sm.unlocked = true
		return true
	}

	salt, err := base64.StdEncoding.DecodeString(sm.store.Salt)
	if err != nil {
		return false
	}

	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
	h := sha256.Sum256(key)
	expectedHash, err := base64.StdEncoding.DecodeString(sm.store.MasterHash)
	if err != nil {
		return false
	}

	if string(h[:]) != string(expectedHash) {
		return false
	}

	sm.masterKey = key
	sm.unlocked = true
	return true
}

func (sm *StorageManager) encrypt(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	if len(sm.masterKey) == 0 {
		return "", errors.New("storage locked, master key not available")
	}

	block, err := aes.NewCipher(sm.masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (sm *StorageManager) decrypt(cipherText string) (string, error) {
	if cipherText == "" {
		return "", nil
	}
	if len(sm.masterKey) == 0 {
		return "", errors.New("storage locked, master key not available")
	}

	data, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(sm.masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (sm *StorageManager) SaveProfile(profile *SessionProfile) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if profile.Name == "" {
		profile.Name = fmt.Sprintf("%s@%s", profile.Username, profile.Host)
	}
	if profile.Port <= 0 {
		profile.Port = 22
	}
	if profile.ID == "" {
		profile.ID = fmt.Sprintf("%s_%d_%s", profile.Username, profile.Port, profile.Host)
	}

	// Encrypt passwords if master key is set
	var savedProfile = *profile
	if len(sm.masterKey) > 0 {
		encPass, err := sm.encrypt(profile.Password)
		if err != nil {
			return err
		}
		savedProfile.Password = encPass

		encKeyPass, err := sm.encrypt(profile.KeyPassphrase)
		if err != nil {
			return err
		}
		savedProfile.KeyPassphrase = encKeyPass
	}

	// Update existing or append
	found := false
	for i, p := range sm.store.Profiles {
		if p.ID == profile.ID {
			sm.store.Profiles[i] = &savedProfile
			found = true
			break
		}
	}
	if !found {
		sm.store.Profiles = append(sm.store.Profiles, &savedProfile)
	}

	return sm.saveToFileLocked()
}

func (sm *StorageManager) GetProfiles() ([]*SessionProfile, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	res := make([]*SessionProfile, 0, len(sm.store.Profiles))
	for _, p := range sm.store.Profiles {
		cp := *p
		if len(sm.masterKey) > 0 {
			decPass, err := sm.decrypt(p.Password)
			if err == nil {
				cp.Password = decPass
			}
			decKeyPass, err := sm.decrypt(p.KeyPassphrase)
			if err == nil {
				cp.KeyPassphrase = decKeyPass
			}
		}
		res = append(res, &cp)
	}

	return res, nil
}

func (sm *StorageManager) DeleteProfile(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	newProfiles := make([]*SessionProfile, 0, len(sm.store.Profiles))
	for _, p := range sm.store.Profiles {
		if p.ID != id {
			newProfiles = append(newProfiles, p)
		}
	}
	sm.store.Profiles = newProfiles
	return sm.saveToFileLocked()
}

func (sm *StorageManager) loadFromFile() error {
	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, sm.store)
}

func (sm *StorageManager) saveToFileLocked() error {
	data, err := json.MarshalIndent(sm.store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sm.filePath, data, 0600)
}

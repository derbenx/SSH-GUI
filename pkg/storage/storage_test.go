package storage

import (
	"os"
	"testing"
)

func TestStorageManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sm, err := NewStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewStorageManager error: %v", err)
	}

	if sm.HasMasterPassword() {
		t.Errorf("Expected HasMasterPassword to be false initially")
	}

	masterPass := "SecretMaster123!"
	err = sm.SetMasterPassword(masterPass)
	if err != nil {
		t.Fatalf("SetMasterPassword error: %v", err)
	}

	if !sm.HasMasterPassword() {
		t.Errorf("Expected HasMasterPassword to be true after setting master password")
	}

	prof := &SessionProfile{
		Host:     "192.168.1.100",
		Port:     22,
		Username: "root",
		AuthType: "password",
		Password: "SuperSecretPassword123",
	}

	err = sm.SaveProfile(prof)
	if err != nil {
		t.Fatalf("SaveProfile error: %v", err)
	}

	// Verify saved encrypted file
	sm2, err := NewStorageManager(tempDir)
	if err != nil {
		t.Fatalf("NewStorageManager 2 error: %v", err)
	}

	if !sm2.HasMasterPassword() {
		t.Errorf("Expected sm2 HasMasterPassword to be true")
	}

	if sm2.VerifyAndUnlock("WrongPassword") {
		t.Errorf("Expected VerifyAndUnlock to fail with wrong password")
	}

	if !sm2.VerifyAndUnlock(masterPass) {
		t.Errorf("Expected VerifyAndUnlock to succeed with correct password")
	}

	profiles, err := sm2.GetProfiles()
	if err != nil {
		t.Fatalf("GetProfiles error: %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("Expected 1 profile, got %d", len(profiles))
	}

	if profiles[0].Password != "SuperSecretPassword123" {
		t.Errorf("Expected decrypted password 'SuperSecretPassword123', got '%s'", profiles[0].Password)
	}

	if profiles[0].Name != "root@192.168.1.100" {
		t.Errorf("Expected default name 'root@192.168.1.100', got '%s'", profiles[0].Name)
	}
}

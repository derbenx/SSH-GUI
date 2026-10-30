package sshclient

import (
	"testing"

	"app/pkg/fileops"
)

func TestParseLsOutput(t *testing.T) {
	sampleLs := `total 32
drwxr-xr-x  5 root root 4096 Oct 25 12:00 .
drwxr-xr-x 20 root root 4096 Oct 25 11:00 ..
-rw-r--r--  1 user group  123 Oct 25 12:01 test.txt
drwxrwxrwx  2 user group 4096 Oct 25 12:02 testdir
lrwxrwxrwx  1 user group    8 Oct 25 12:03 link.txt -> test.txt
`

	items := parseLsOutput(sampleLs, "/home/user")
	if len(items) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(items))
	}

	if items[0].Name != "test.txt" || items[0].IsDir || items[0].Size != 123 {
		t.Errorf("Item 0 incorrect: %+v", items[0])
	}

	if items[1].Name != "testdir" || !items[1].IsDir {
		t.Errorf("Item 1 incorrect: %+v", items[1])
	}

	if items[2].Name != "link.txt" {
		t.Errorf("Item 2 name incorrect: %s", items[2].Name)
	}
}

func TestModeToOctal(t *testing.T) {
	tests := []struct {
		mode     string
		expected string
	}{
		{"-rw-r--r--", "0644"},
		{"drwxr-xr-x", "0755"},
		{"-rwxrwxrwx", "0777"},
		{"-r--------", "0400"},
	}

	for _, tt := range tests {
		res := modeToOctal(tt.mode)
		if res != tt.expected {
			t.Errorf("modeToOctal(%s) = %s, expected %s", tt.mode, res, tt.expected)
		}
	}
}

func TestLocalFileManager(t *testing.T) {
	lm := fileops.NewLocalFileManager()
	dir, items, err := lm.ListDir(".")
	if err != nil {
		t.Fatalf("ListDir error: %v", err)
	}

	if dir == "" {
		t.Errorf("Expected non-empty directory path")
	}

	if len(items) == 0 {
		t.Errorf("Expected items in current directory")
	}
}

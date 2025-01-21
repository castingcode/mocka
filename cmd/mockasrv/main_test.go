package main

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_dataFolder(t *testing.T) {
	t.Run("folderFlag is nil", func(t *testing.T) {
		folder, err := dataFolder(nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		ex, _ := os.Executable()
		expected := filepath.Join(filepath.Dir(ex), "responses")
		if folder != expected {
			t.Fatalf("expected %s, got %s", expected, folder)
		}
	})

	t.Run("folderFlag is empty", func(t *testing.T) {
		folderFlag := ""
		folder, err := dataFolder(&folderFlag)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		ex, _ := os.Executable()
		expected := filepath.Join(filepath.Dir(ex), "responses")
		if folder != expected {
			t.Fatalf("expected %s, got %s", expected, folder)
		}
	})

	t.Run("folderFlag points to non-existent folder", func(t *testing.T) {
		folderFlag := "/non/existent/folder"
		_, err := dataFolder(&folderFlag)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("folderFlag points to existing folder", func(t *testing.T) {
		tempDir := t.TempDir()
		folderFlag := tempDir
		folder, err := dataFolder(&folderFlag)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if folder != tempDir {
			t.Fatalf("expected %s, got %s", tempDir, folder)
		}
	})
}

func Test_router(t *testing.T) {
	t.Run("valid folder", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := router(&tempDir)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("invalid folder", func(t *testing.T) {
		folderFlag := "/non/existent/folder"
		_, err := router(&folderFlag)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

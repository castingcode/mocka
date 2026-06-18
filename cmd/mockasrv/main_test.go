package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_parseLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    slog.Level
		wantErr bool
	}{
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "info", input: "info", want: slog.LevelInfo},
		{name: "warn", input: "warn", want: slog.LevelWarn},
		{name: "warning alias", input: "warning", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
		{name: "case insensitive", input: "INFO", want: slog.LevelInfo},
		{name: "unsupported", input: "trace", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLogLevel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected level %v, got %v", tt.want, got)
			}
		})
	}
}

func Test_newLogger(t *testing.T) {
	t.Run("text format", func(t *testing.T) {
		var buf bytes.Buffer
		logger, err := newLogger("text", "info", &buf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		logger.Info("hello", "key", "value")
		if !strings.Contains(buf.String(), "hello") {
			t.Fatalf("expected log output to contain message, got %q", buf.String())
		}
		if strings.Contains(buf.String(), `"msg"`) {
			t.Fatalf("expected text format, got json-like output: %q", buf.String())
		}
	})

	t.Run("json format", func(t *testing.T) {
		var buf bytes.Buffer
		logger, err := newLogger("json", "info", &buf)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		logger.Info("hello", "key", "value")
		if !strings.Contains(buf.String(), `"msg":"hello"`) {
			t.Fatalf("expected json log output, got %q", buf.String())
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := newLogger("xml", "info", &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid level", func(t *testing.T) {
		_, err := newLogger("text", "trace", &bytes.Buffer{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

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

func Test_buildMux(t *testing.T) {
	t.Run("valid folder", func(t *testing.T) {
		tempDir := t.TempDir()
		_, responsesPath, err := buildMux(&tempDir)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if responsesPath != tempDir {
			t.Fatalf("expected responses path %s, got %s", tempDir, responsesPath)
		}
	})

	t.Run("invalid folder", func(t *testing.T) {
		folderFlag := "/non/existent/folder"
		_, _, err := buildMux(&folderFlag)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("malformed responses.yml", func(t *testing.T) {
		tempDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tempDir, "responses.yml"), []byte("responses:\n  - [unclosed"), 0644); err != nil {
			t.Fatal(err)
		}
		_, _, err := buildMux(&tempDir)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/castingcode/mocka"
)

func main() {
	port := flag.Int("port", 4500, "Port to run the web server on")
	folder := flag.String("folder", "", "Folder to store mock data")
	logFormat := flag.String("log-format", "text", "Log format: text or json")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, or error")
	flag.Parse()

	logger, err := newLogger(*logFormat, *logLevel, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid logging configuration: %v\n", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)

	mux, responsesPath, err := buildMux(folder)
	if err != nil {
		logger.Error("failed to start server", "error", err)
		os.Exit(1)
	}

	address := fmt.Sprintf(":%d", *port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("failed to listen", "addr", address, "error", err)
		os.Exit(1)
	}

	logger.Info("mocka server listening", "addr", listener.Addr().String(), "responses", responsesPath)

	if err := http.Serve(listener, mux); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func newLogger(format, level string, w io.Writer) (*slog.Logger, error) {
	lvl, err := parseLogLevel(level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "text":
		handler = slog.NewTextHandler(w, opts)
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		return nil, fmt.Errorf("unsupported log format %q (want text or json)", format)
	}

	return slog.New(handler), nil
}

func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q (want debug, info, warn, or error)", level)
	}
}

func buildMux(folder *string) (*http.ServeMux, string, error) {
	f, err := dataFolder(folder)
	if err != nil {
		return nil, "", err
	}
	lookup, err := mocka.NewResponseLookup(mocka.NewFileResponseLoader(f))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create response lookup: %w", err)
	}
	handler := mocka.NewMocaRequestHandler(lookup)

	mux := http.NewServeMux()
	mocka.RegisterRoutes(mux, handler)

	return mux, f, nil
}

func dataFolder(folderFlag *string) (string, error) {
	if folderFlag != nil && *folderFlag != "" {
		if _, err := os.Stat(*folderFlag); os.IsNotExist(err) {
			return "", fmt.Errorf("folder %s does not exist", *folderFlag)
		}
		return *folderFlag, nil
	}
	ex, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	responsesPath := filepath.Join(filepath.Dir(ex), "responses")
	if _, err := os.Stat(responsesPath); os.IsNotExist(err) {
		if err := os.Mkdir(responsesPath, os.ModePerm); err != nil {
			return "", fmt.Errorf("failed to create responses folder: %w", err)
		}
	}
	return responsesPath, nil
}

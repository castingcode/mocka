package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/castingcode/mocka"
)

func main() {
	port := flag.Int("port", 9000, "Port to run the web server on")
	folder := flag.String("folder", "", "Folder to store mock data")
	flag.Parse()

	mux, err := buildMux(folder)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	address := fmt.Sprintf(":%d", *port)
	err = http.ListenAndServe(address, mux)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func buildMux(folder *string) (*http.ServeMux, error) {
	f, err := dataFolder(folder)
	if err != nil {
		return nil, err
	}
	lookup, err := mocka.NewResponseLookup(mocka.NewFileResponseLoader(f))
	if err != nil {
		return nil, fmt.Errorf("failed to create response lookup: %w", err)
	}
	handler := mocka.NewMocaRequestHandler(lookup)

	mux := http.NewServeMux()
	mocka.RegisterRoutes(mux, handler)

	return mux, nil
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

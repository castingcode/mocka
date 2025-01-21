package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/castingcode/mocka"
	"github.com/gin-gonic/gin"
)

func main() {
	port := flag.Int("port", 9000, "Port to run the web server on")
	folder := flag.String("folder", "", "Folder to store mock data")
	flag.Parse()

	r, err := router(folder)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	address := fmt.Sprintf(":%d", *port)
	err = r.Run(address)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func router(folder *string) (*gin.Engine, error) {
	f, err := dataFolder(folder)
	if err != nil {
		return nil, err
	}
	lookup, _ := mocka.NewResponseLookup(mocka.WithDataFolder(f))
	handler := mocka.NewMocaRequestHandler(lookup)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	mocka.RegisterRoutes(router, handler)

	return router, nil
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

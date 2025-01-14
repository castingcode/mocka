package main

import (
	"flag"
	"fmt"

	"github.com/castingcode/mocka"
	"github.com/gin-gonic/gin"
)

func main() {
	port := flag.Int("port", 9000, "Port to run the web server on")
	flag.Parse()

	lookup, _ := mocka.NewResponseLookup()
	handler := mocka.NewMocaRequestHandler(lookup)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	mocka.RegisterRoutes(router, handler)

	address := fmt.Sprintf(":%d", *port)
	router.Run(address)
}

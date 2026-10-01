package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TODO: US-201: Load Maya sandbox keys from .env

func getting(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"method": "GET"})
}

func main() {
	router := gin.Default()

	router.GET("/someGet", getting)
	router.Run()
}

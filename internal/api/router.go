package api

import (
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	router.POST("/score", scoreHandler)
	router.GET("/health", healthHandler)

	return router
}

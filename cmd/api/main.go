package main

import "github.com/gin-gonic/gin"

func main() {
	router := gin.Default()

	v1 := router.Group("/api/v1")
	v1.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	router.Run() // listens on 0.0.0.0:8080 by default
}

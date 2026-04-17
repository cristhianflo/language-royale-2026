package main

import (
	"cristhianflo/language-royale/internal/api"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := api.NewRouter()
	router.Run() // listens on 0.0.0.0:8080 by default
}

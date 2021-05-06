package main

import (
	"github.com/gin-gonic/gin"
	"github.com/ray-xu-deltatre/pact-go/examples/gin/provider"
)

func main() {
	router := gin.Default()
	router.POST("/login/:id", provider.UserLogin)
	router.POST("/users/:id", provider.IsAuthenticated(), provider.GetUser)
	router.Run("localhost:8080")
}

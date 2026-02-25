package main

import (
	"github.com/gin-gonic/gin"
	"vetWS/handlers"
)

func main() {
	r := gin.Default()

	wsHandler := handlers.NewWebSocketHandler()

	r.GET("/ws/frontend", wsHandler.HandleFrontend)
	r.GET("/ws/esp32", wsHandler.HandleESP32)

	r.Run(":8081")
}

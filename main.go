package main

import (
	"github.com/gin-gonic/gin"
	"vetWS/handlers"
)

func main() {
	r := gin.Default()

	wsHandler := handlers.NewWebSocketHandler()

	// Frontend WebSocket (recibe datos del backend)
	r.GET("/ws/frontend", wsHandler.HandleFrontend)

	// Bluetooth Bridge WebSocket (envía datos del Bluetooth Bridge)
	r.GET("/ws/bluetooth", wsHandler.HandleBluetooth)

	// Comandos WebSocket (Bluetooth Bridge recibe comandos)
	r.GET("/ws/commands", wsHandler.HandleCommands)

	// ESP32 directo (compatibilidad heredada)
	r.GET("/ws/esp32", wsHandler.HandleESP32)

	// Endpoint de estadísticas (opcional)
	r.GET("/ws/stats", func(c *gin.Context) {
		stats := wsHandler.GetStatistics()
		c.JSON(200, stats)
	})

	r.Run(":8081")
}

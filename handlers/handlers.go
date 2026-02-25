package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	frontends map[*websocket.Conn]bool
	esp32s    map[*websocket.Conn]bool
	mutex     sync.Mutex
	upgrader  websocket.Upgrader
}

type CommandMessage struct {
	Action      string `json:"action"`
	Type        string `json:"type"`
	TotalTime   string `json:"totalTime"`
	FrequencyHZ int    `json:"frequencyHZ"`
	AmplitudeMV int    `json:"amplitudeMV"`
}


type ESP32Data struct {
	// Datos de la IMU (MPU6050)
	Gyroscope     json.RawMessage `json:"gyroscope"`
	Accelerometer json.RawMessage `json:"accelerometer"`
	
	// Datos de los 4 sensores de peso
	WeightDistributionLF json.RawMessage `json:"weightDistributionLF"`
	WeightDistributionRF json.RawMessage `json:"weightDistributionRF"`
	WeightDistributionLB json.RawMessage `json:"weightDistributionLB"`
	WeightDistributionRB json.RawMessage `json:"weightDistributionRB"`
	
	// Índices de simetría
	SymmetryIndexLF json.RawMessage `json:"symmetryIndexLF"`
	SymmetryIndexRF json.RawMessage `json:"symmetryIndexRF"`
	SymmetryIndexLB json.RawMessage `json:"symmetryIndexLB"`
	SymmetryIndexRB json.RawMessage `json:"symmetryIndexRB"`
	
	// Fuerza vertical
	VerticalForce   json.RawMessage `json:"verticalForce"`
	VerticalImpulse string          `json:"verticalImpulse"`
}

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		frontends: make(map[*websocket.Conn]bool),
		esp32s:    make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

/* ================= FRONTEND ================= */

func (h *WebSocketHandler) HandleFrontend(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Frontend upgrade error:", err)
		return
	}
	defer conn.Close()

	h.mutex.Lock()
	h.frontends[conn] = true
	h.mutex.Unlock()

	log.Println("Frontend connected")

	for {
		var cmd CommandMessage
		if err := conn.ReadJSON(&cmd); err != nil {
			break
		}
		log.Printf("Command received from frontend: %+v", cmd)
		h.broadcastCommandToESP32(cmd)
	}

	h.mutex.Lock()
	delete(h.frontends, conn)
	h.mutex.Unlock()
	log.Println("Frontend disconnected")
}

/* ================= ESP32 ================= */

func (h *WebSocketHandler) HandleESP32(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("ESP32 upgrade error:", err)
		return
	}
	defer conn.Close()

	h.mutex.Lock()
	h.esp32s[conn] = true
	h.mutex.Unlock()

	log.Println("ESP32 connected")

	for {
		var data ESP32Data
		if err := conn.ReadJSON(&data); err != nil {
			log.Printf("Error reading from ESP32: %v", err)
			break
		}
		log.Println("Data received from ESP32, broadcasting to frontends")
		h.broadcastDataToFrontend(data)
	}

	h.mutex.Lock()
	delete(h.esp32s, conn)
	h.mutex.Unlock()
	log.Println("ESP32 disconnected")
}

/* ================= BROADCAST ================= */

func (h *WebSocketHandler) broadcastCommandToESP32(cmd CommandMessage) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for conn := range h.esp32s {
		if err := conn.WriteJSON(cmd); err != nil {
			log.Printf("Error broadcasting command to ESP32: %v", err)
		}
	}
}

func (h *WebSocketHandler) broadcastDataToFrontend(data ESP32Data) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for conn := range h.frontends {
		if err := conn.WriteJSON(data); err != nil {
			log.Printf("Error broadcasting data to frontend: %v", err)
			conn.Close()
			delete(h.frontends, conn)
		}
	}
}
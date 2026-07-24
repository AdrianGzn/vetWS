package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	frontends map[*websocket.Conn]bool
	bluetooth map[*websocket.Conn]bool // Conexiones desde Bluetooth Bridge
	mutex     sync.Mutex
	upgrader  websocket.Upgrader
}

// CommandMessage - Comandos del Frontend al Bluetooth Bridge
type CommandMessage struct {
	Action      string  `json:"action"`
	Type        string  `json:"type"`
	TotalTime   string  `json:"totalTime"`
	FrequencyHZ int     `json:"frequencyHZ"`
	AmplitudeMV int     `json:"amplitudeMV"`
	SpeedX      float64 `json:"speedX,omitempty"`
	SpeedY      float64 `json:"speedY,omitempty"`
}

// BluetoothSensorData - Datos unificados desde Bluetooth Bridge
type BluetoothSensorData struct {
	// Datos de pesos (ESP32 #1)
	WeightDistributionLF float64            `json:"weightDistributionLF"`
	WeightDistributionRF float64            `json:"weightDistributionRF"`
	WeightDistributionLB float64            `json:"weightDistributionLB"`
	WeightDistributionRB float64            `json:"weightDistributionRB"`
	TotalWeight          float64            `json:"totalWeight"`
	COP                  json.RawMessage    `json:"cop"`

	// Datos de rotación (ESP32 #2)
	Gyroscope     json.RawMessage `json:"gyroscope"`
	Accelerometer json.RawMessage `json:"accelerometer"`
	Angles        json.RawMessage `json:"angles"`
	Temperature   float64         `json:"temperature"`

	// Metadatos
	Timestamp int64 `json:"timestamp"`
}

// ESP32Data - Estructura heredada para compatibilidad
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
		bluetooth: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

/* ================= FRONTEND ================= */

func (h *WebSocketHandler) HandleFrontend(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("[WS-ERROR] Frontend upgrade error:", err)
		return
	}
	defer conn.Close()

	h.mutex.Lock()
	h.frontends[conn] = true
	h.mutex.Unlock()

	log.Println("[WS-FRONTEND] Cliente conectado")

	for {
		var cmd CommandMessage
		if err := conn.ReadJSON(&cmd); err != nil {
			log.Printf("[WS-FRONTEND] Error de lectura: %v", err)
			break
		}

		log.Printf("[WS-COMMAND] Comando recibido: %s (tipo: %s)", cmd.Action, cmd.Type)

		// Reenviar comando al Bluetooth Bridge
		h.broadcastCommandToBluetooth(cmd)
	}

	h.mutex.Lock()
	delete(h.frontends, conn)
	h.mutex.Unlock()

	log.Println("[WS-FRONTEND] Cliente desconectado")
}

/* ================= BLUETOOTH BRIDGE ================= */

// HandleBluetooth es el nuevo endpoint para el Bluetooth Bridge
func (h *WebSocketHandler) HandleBluetooth(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("[WS-ERROR] Bluetooth upgrade error:", err)
		return
	}
	defer conn.Close()

	h.mutex.Lock()
	h.bluetooth[conn] = true
	h.mutex.Unlock()

	log.Println("[WS-BLUETOOTH] Bluetooth Bridge conectado")

	for {
		var data BluetoothSensorData
		if err := conn.ReadJSON(&data); err != nil {
			log.Printf("[WS-BLUETOOTH] Error de lectura: %v", err)
			break
		}

		log.Printf(
			"[WS-DATA] Datos Bluetooth recibidos - Pesos: LF=%.2f RF=%.2f LB=%.2f RB=%.2f | Temp=%.1f°C",
			data.WeightDistributionLF, data.WeightDistributionRF,
			data.WeightDistributionLB, data.WeightDistributionRB,
			data.Temperature,
		)

		// Reenviar datos al Frontend
		h.broadcastDataToFrontend(data)
	}

	h.mutex.Lock()
	delete(h.bluetooth, conn)
	h.mutex.Unlock()

	log.Println("[WS-BLUETOOTH] Bluetooth Bridge desconectado")
}

/* ================= ESP32 (LEGADO) ================= */

// HandleESP32 - Mantener para compatibilidad con ESP32 directos
func (h *WebSocketHandler) HandleESP32(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("[WS-ERROR] ESP32 upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Println("[WS-WARNING] Conexión directa de ESP32 detectada (usar Bluetooth Bridge)")

	// Reconvertir datos de ESP32 a formato Bluetooth
	for {
		var data ESP32Data
		if err := conn.ReadJSON(&data); err != nil {
			log.Printf("[WS-ESP32] Error de lectura: %v", err)
			break
		}

		// Convertir formato ESP32 a BluetoothSensorData
		bluetoothData := BluetoothSensorData{
			Gyroscope:     data.Gyroscope,
			Accelerometer: data.Accelerometer,
		}

		h.broadcastDataToFrontend(bluetoothData)
	}
}

/* ================= BROADCAST ================= */

func (h *WebSocketHandler) broadcastCommandToBluetooth(cmd CommandMessage) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Enviar comando a todas las conexiones Bluetooth
	for conn := range h.bluetooth {
		if err := conn.WriteJSON(cmd); err != nil {
			log.Printf("[WS-ERROR] Error enviando comando: %v", err)
			conn.Close()
			delete(h.bluetooth, conn)
		}
	}

	// Log
	if len(h.bluetooth) > 0 {
		log.Printf("[WS-BROADCAST] Comando enviado a %d cliente(s) Bluetooth", len(h.bluetooth))
	}
}

func (h *WebSocketHandler) broadcastDataToFrontend(data interface{}) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Crear mensaje JSON
	var payload interface{}

	switch v := data.(type) {
	case BluetoothSensorData:
		// Asegurarse de que los valores son numéricos
		payload = map[string]interface{}{
			"weightDistributionLF": v.WeightDistributionLF,
			"weightDistributionRF": v.WeightDistributionRF,
			"weightDistributionLB": v.WeightDistributionLB,
			"weightDistributionRB": v.WeightDistributionRB,
			"totalWeight":          v.TotalWeight,
			"cop":                  v.COP,
			"gyroscope":            v.Gyroscope,
			"accelerometer":        v.Accelerometer,
			"angles":               v.Angles,
			"temperature":          v.Temperature,
			"timestamp":            v.Timestamp,
		}
	case ESP32Data:
		payload = v
	}

	// Enviar a todos los frontends
	for conn := range h.frontends {
		if err := conn.WriteJSON(payload); err != nil {
			log.Printf("[WS-ERROR] Error enviando datos: %v", err)
			conn.Close()
			delete(h.frontends, conn)
		}
	}

	// Log
	if len(h.frontends) > 0 {
		log.Printf("[WS-BROADCAST] Datos enviados a %d cliente(s) frontend", len(h.frontends))
	}
}

// GetStatistics devuelve estadísticas del WebSocket
func (h *WebSocketHandler) GetStatistics() map[string]interface{} {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	return map[string]interface{}{
		"frontends": len(h.frontends),
		"bluetooth": len(h.bluetooth),
		"timestamp": int64(time.Now().Unix()),
	}
}

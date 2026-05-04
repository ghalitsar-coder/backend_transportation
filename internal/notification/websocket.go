package notification

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"transit-app/internal/domain"
	"transit-app/internal/logger"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for public MVP
	},
}

type WebSocketServer struct {
	clients     map[*websocket.Conn]bool
	clientsMu   sync.RWMutex
	vehicleRepo domain.VehicleRepository
}

func NewWebSocketServer(vehicleRepo domain.VehicleRepository) *WebSocketServer {
	return &WebSocketServer{
		clients:     make(map[*websocket.Conn]bool),
		vehicleRepo: vehicleRepo,
	}
}

func (ws *WebSocketServer) RegisterRoutes(router *gin.Engine) {
	router.GET("/ws/transit/track", ws.handleConnections)
}

func (ws *WebSocketServer) handleConnections(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Failed to upgrade connection: %v", err)
		return
	}

	ws.clientsMu.Lock()
	ws.clients[conn] = true
	ws.clientsMu.Unlock()

	defer func() {
		ws.clientsMu.Lock()
		delete(ws.clients, conn)
		ws.clientsMu.Unlock()
		conn.Close()
	}()

	// Keep alive loop
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (ws *WebSocketServer) broadcastMessage(msg interface{}) {
	var failed []*websocket.Conn

	ws.clientsMu.RLock()
	for client := range ws.clients {
		if err := client.WriteJSON(msg); err != nil {
			logger.Error("Error broadcasting to client: %v", err)
			failed = append(failed, client)
		}
	}
	ws.clientsMu.RUnlock()

	// Hapus koneksi yang gagal — butuh Lock penuh, bukan RLock
	if len(failed) > 0 {
		ws.clientsMu.Lock()
		for _, client := range failed {
			delete(ws.clients, client)
			client.Close()
		}
		ws.clientsMu.Unlock()
	}
}

func (ws *WebSocketServer) RunSimulator(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping WebSocket Simulator")
			return
		case <-ticker.C:
			// Fetch active vehicles from DB to track their status
			// In a real app, this would interpolate pos based on polyline data
			// For MVP simulation, we just broadcast mock/fetched positions
			vehicles, err := ws.vehicleRepo.FindAllActive(ctx)
			if err != nil {
				logger.Error("Simulator err fetching vehicles: %v", err)
				continue
			}

			now := time.Now().Unix()
			for _, v := range vehicles {
				routeID := ""
				if v.RouteID != nil {
					routeID = v.RouteID.String()
				}
				// Mock update payload
				update := domain.VehicleUpdate{
					Type:      "VEHICLE_UPDATE",
					VehicleID: v.VehicleCode,
					RouteID:   routeID,
					Latitude:  -6.200000, // mock location
					Longitude: 106.816666,
					Heading:   45,
					Speed:     30,
					Timestamp: now,
				}
				ws.broadcastMessage(update)
			}
		}
	}
}

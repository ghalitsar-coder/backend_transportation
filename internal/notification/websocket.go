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
	ticker := time.NewTicker(5 * time.Second) // Update every 5 seconds for smoother movement
	defer ticker.Stop()

	// Bandung center coordinates for fallback
	const bandungLat = -6.917474
	const bandungLng = 107.619123

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping WebSocket Simulator")
			return
		case <-ticker.C:
			// Fetch active vehicles from DB to track their status
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

				// Generate position based on time and vehicle code for variety
				// This creates movement along the route
				timeOffset := float64(now%60) / 60.0 // 0-1 over 60 seconds
				vehicleOffset := float64(hashCode(v.VehicleCode)%100) / 100.0 // 0-1 per vehicle
				
				// Combine time and vehicle offset for unique positions
				progress := (timeOffset + vehicleOffset)
				if progress > 1.0 {
					progress -= 1.0
				}

				// Use Bandung coordinates with route-based variation
				// Different routes get slightly different base positions
				var lat, lng float64
				switch routeID {
				case "11111111-1111-1111-1111-111111111111": // Koridor 1
					lat = -6.9204 + progress*0.02 // Cibiru to Cibeureum area
					lng = 107.7194 - progress*0.16
				case "22222222-2222-2222-2222-222222222222": // Koridor 2  
					lat = -6.9083 + progress*0.01 // Cicaheum to Cibeureum area
					lng = 107.6537 - progress*0.10
				case "33333333-3333-3333-3333-333333333333": // Koridor 3
					lat = -6.9083 - progress*0.04 // Cicaheum to Sarijadi area
					lng = 107.6537 - progress*0.06
				case "44444444-4444-4444-4444-444444444444": // Feeder 1
					lat = -6.9117 + progress*0.02 // Stasiun Hall to Gunung Batu
					lng = 107.6034 - progress*0.03
				case "55555555-5555-5555-5555-555555555555": // Angkot Dago
					lat = -6.9117 - progress*0.03 // Stasiun Hall to Dago
					lng = 107.6034 + progress*0.02
				default:
					// Fallback to Bandung center with some movement
					lat = bandungLat + (progress-0.5)*0.05
					lng = bandungLng + (progress-0.5)*0.05
				}

				// Calculate heading based on movement direction
				heading := float64((now + hashCode(v.VehicleCode)) % 360)

				// Realistic speed for different vehicle types (km/h)
				speed := 25.0 // Base speed
				if v.Type == "bus" {
					speed = 30.0 + float64(now%10) // 30-40 km/h for buses
				} else if v.Type == "angkot" {
					speed = 20.0 + float64(now%8) // 20-28 km/h for angkot
				} else if v.Type == "minibus" {
					speed = 25.0 + float64(now%6) // 25-31 km/h for minibus
				}

				update := domain.VehicleUpdate{
					Type:      "VEHICLE_UPDATE",
					VehicleID: v.VehicleCode,
					RouteID:   routeID,
					Latitude:  lat,
					Longitude: lng,
					Heading:   heading,
					Speed:     speed,
					Timestamp: now,
				}
				ws.broadcastMessage(update)
			}
		}
	}
}

// Simple hash function for vehicle code variation
func hashCode(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

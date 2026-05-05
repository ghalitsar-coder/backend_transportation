package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"transit-app/internal/domain"
	"transit-app/internal/logger"
)

// LatLngPoint mewakili satu titik koordinat pada polyline rute.
type LatLngPoint struct {
	Lat float64
	Lng float64
}

// RoutePolyline menyimpan seluruh titik path dan metadata rute dari CSV.
type RoutePolyline struct {
	RouteID   string        // id_trayek dari CSV, misal "01A"
	RouteName string        // nama trayek lengkap
	ColorHex  string        // warna rute
	Points    []LatLngPoint // titik-titik polyline (lat, lng)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for public MVP
	},
}

// WebSocketServer mengelola koneksi klien WebSocket dan simulasi posisi kendaraan.
type WebSocketServer struct {
	clients     map[*websocket.Conn]bool
	clientsMu   sync.RWMutex
	vehicleRepo domain.VehicleRepository

	// routePolylines: cache in-memory polyline per route_id dari CSV.
	// Key: VehicleCode atau RouteID dari DB, Value: polyline titik-titik rute.
	// Diisi sekali saat LoadCSVRoutes dipanggil, dibaca concurrent saat simulator berjalan.
	routePolylines []RoutePolyline
	polylineMu     sync.RWMutex
}

func NewWebSocketServer(vehicleRepo domain.VehicleRepository) *WebSocketServer {
	return &WebSocketServer{
		clients:     make(map[*websocket.Conn]bool),
		vehicleRepo: vehicleRepo,
	}
}

// ─────────────────────────────────────────────────────────────
//  LOAD GEOJSON ROUTES — dipanggil sekali saat startup
// ─────────────────────────────────────────────────────────────

// geojsonFeatureCollection adalah struct minimal untuk decode routes.geojson.
type geojsonFeatureCollection struct {
	Features []struct {
		Properties struct {
			RouteID   string `json:"route_id"`
			RouteName string `json:"route_name"`
			ColorHex  string `json:"color_hex"`
		} `json:"properties"`
		Geometry struct {
			Coordinates [][2]float64 `json:"coordinates"` // [lng, lat]
		} `json:"geometry"`
	} `json:"features"`
}

// LoadGeoJSONRoutes membaca file routes.geojson dan membangun cache polyline.
// GeoJSON coordinates format: [longitude, latitude] (dikonversi ke LatLngPoint).
// Fungsi ini dipanggil dari main.go saat server startup.
func (ws *WebSocketServer) LoadGeoJSONRoutes(geoJSONPath string) error {
	data, err := os.ReadFile(geoJSONPath)
	if err != nil {
		return fmt.Errorf("gagal membaca GeoJSON: %w", err)
	}

	var fc geojsonFeatureCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("gagal parse GeoJSON: %w", err)
	}

	var routes []RoutePolyline
	for _, feature := range fc.Features {
		if len(feature.Geometry.Coordinates) < 2 {
			continue
		}
		// Konversi [lng, lat] → LatLngPoint{Lat, Lng}
		points := make([]LatLngPoint, 0, len(feature.Geometry.Coordinates))
		for _, coord := range feature.Geometry.Coordinates {
			points = append(points, LatLngPoint{
				Lat: coord[1], // index 1 = latitude
				Lng: coord[0], // index 0 = longitude
			})
		}
		routes = append(routes, RoutePolyline{
			RouteID:   feature.Properties.RouteID,
			RouteName: feature.Properties.RouteName,
			ColorHex:  feature.Properties.ColorHex,
			Points:    points,
		})
	}

	ws.polylineMu.Lock()
	ws.routePolylines = routes
	ws.polylineMu.Unlock()

	logger.Info("Loaded %d route polylines from GeoJSON", len(routes))
	return nil
}


// ─────────────────────────────────────────────────────────────
//  INTERPOLASI POLYLINE — inti dari simulator baru
// ─────────────────────────────────────────────────────────────

// interpolateOnPolyline menghitung posisi dan heading kendaraan pada
// polyline berdasarkan nilai progress (0.0 = awal rute, 1.0 = akhir rute).
//
// Algoritma:
// 1. Hitung total panjang polyline (Euclidean antar titik)
// 2. Target distance = progress × total_length
// 3. Walk titik satu per satu hingga akumulasi >= target
// 4. Interpolasi linear antara dua titik terdekat
// 5. Heading = atan2(Δlat, Δlng) antara titik saat ini dan berikutnya
//
// Ini memastikan kendaraan SELALU berada tepat di atas garis rute.
func interpolateOnPolyline(points []LatLngPoint, progress float64) (lat, lng, heading float64) {
	if len(points) == 0 {
		return 0, 0, 0
	}
	if len(points) == 1 {
		return points[0].Lat, points[0].Lng, 0
	}

	// Clamp progress ke [0, 1]
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	// Hitung total panjang polyline
	totalLen := 0.0
	for i := 1; i < len(points); i++ {
		totalLen += segmentLength(points[i-1], points[i])
	}

	// Panjang yang ditarget
	targetDist := progress * totalLen

	// Walk segmen satu per satu sampai mencapai target distance
	accumulated := 0.0
	for i := 1; i < len(points); i++ {
		segLen := segmentLength(points[i-1], points[i])

		if accumulated+segLen >= targetDist || i == len(points)-1 {
			// Kendaraan ada di segmen ini
			remaining := targetDist - accumulated
			t := 0.0
			if segLen > 0 {
				t = remaining / segLen
			}
			if t > 1 {
				t = 1
			}

			// Interpolasi linear lat dan lng
			p0 := points[i-1]
			p1 := points[i]
			lat = p0.Lat + t*(p1.Lat-p0.Lat)
			lng = p0.Lng + t*(p1.Lng-p0.Lng)

			// Heading dalam derajat (0 = utara, 90 = timur, dst)
			dLat := p1.Lat - p0.Lat
			dLng := p1.Lng - p0.Lng
			heading = math.Atan2(dLng, dLat) * 180 / math.Pi
			if heading < 0 {
				heading += 360
			}
			return lat, lng, heading
		}
		accumulated += segLen
	}

	// Fallback: titik terakhir
	last := points[len(points)-1]
	return last.Lat, last.Lng, 0
}

// segmentLength menghitung panjang Euclidean antara dua titik (dalam derajat).
// Cukup akurat untuk jarak pendek dalam area kota.
func segmentLength(a, b LatLngPoint) float64 {
	dLat := b.Lat - a.Lat
	dLng := b.Lng - a.Lng
	return math.Sqrt(dLat*dLat + dLng*dLng)
}

// ─────────────────────────────────────────────────────────────
//  WEBSOCKET SERVER
// ─────────────────────────────────────────────────────────────

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

	// Keep alive — baca pesan dari client (ping/pong)
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

	// Hapus koneksi yang gagal dengan Lock penuh
	if len(failed) > 0 {
		ws.clientsMu.Lock()
		for _, client := range failed {
			delete(ws.clients, client)
			client.Close()
		}
		ws.clientsMu.Unlock()
	}
}

// ─────────────────────────────────────────────────────────────
//  SIMULATOR — logika utama pergerakan kendaraan
// ─────────────────────────────────────────────────────────────

// RunSimulator adalah goroutine utama yang berjalan setiap 5 detik.
// Untuk setiap kendaraan aktif di DB:
//   1. Ambil polyline rute yang sesuai dari cache CSV (round-robin index)
//   2. Hitung progress kendaraan berdasarkan waktu + offset unik per kendaraan
//   3. Interpolasi posisi tepat di atas polyline
//   4. Broadcast posisi ke semua klien WebSocket
func (ws *WebSocketServer) RunSimulator(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping WebSocket Simulator")
			return
		case <-ticker.C:
			// Ambil daftar kendaraan aktif dari DB
			vehicles, err := ws.vehicleRepo.FindAllActive(ctx)
			if err != nil {
				logger.Error("Simulator err fetching vehicles: %v", err)
				continue
			}

			// Ambil snapshot polylines (thread-safe read)
			ws.polylineMu.RLock()
			polylines := ws.routePolylines
			ws.polylineMu.RUnlock()

			// Jika tidak ada polyline, tidak ada yang bisa disimulasikan
			if len(polylines) == 0 {
				logger.Error("No route polylines loaded — kendaraan tidak bisa bergerak")
				continue
			}

			now := time.Now().Unix()

			for i, v := range vehicles {
				routeID := ""
				if v.RouteID != nil {
					routeID = v.RouteID.String()
				}

				// ─────────────────────────────────────────────
				// Pilih polyline untuk kendaraan ini.
				//
				// Strategi: distribusi merata kendaraan ke rute CSV.
				// - Setiap kendaraan mendapat rute berdasarkan index modulo
				//   total rute yang ada di CSV.
				// - Ini memastikan kendaraan tersebar di SEMUA rute, tidak
				//   menumpuk di satu rute saja.
				// ─────────────────────────────────────────────
				routeIdx := i % len(polylines)
				chosenRoute := polylines[routeIdx]

				// ─────────────────────────────────────────────
				// Hitung progress kendaraan (0.0 → 1.0).
				//
				// - timeOffset: berubah setiap 5 menit (siklus penuh 300 detik)
				//   → semua kendaraan bergerak bersama mengikuti waktu
				// - vehicleOffset: nilai unik per kendaraan berdasarkan hash kode
				//   → kendaraan tidak bertumpuk di titik yang sama
				// - Kombinasi keduanya: kendaraan tersebar merata di sepanjang rute
				//   dan bergerak maju bersama-sama
				// ─────────────────────────────────────────────
				cycleSeconds := int64(300) // siklus 5 menit
				timeOffset := float64(now%cycleSeconds) / float64(cycleSeconds)
				vehicleOffset := float64(hashCode(v.VehicleCode)%100) / 100.0

				progress := timeOffset + vehicleOffset
				if progress > 1.0 {
					progress -= 1.0
				}

				// Interpolasi posisi tepat di atas polyline CSV
				lat, lng, heading := interpolateOnPolyline(chosenRoute.Points, progress)

				// Kecepatan realistis berdasarkan tipe kendaraan
				speed := 25.0
				switch v.Type {
				case "bus":
					speed = 30.0 + float64(now%10)
				case "angkot":
					speed = 20.0 + float64(now%8)
				case "minibus":
					speed = 25.0 + float64(now%6)
				}

				update := domain.VehicleUpdate{
					Type:      "VEHICLE_UPDATE",
					VehicleID: v.VehicleCode,
					// Gunakan route_id dari DB jika ada, fallback ke ID rute CSV
					// Frontend menggunakan route_id ini untuk match dengan warna rute
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

// ─────────────────────────────────────────────────────────────
//  UTILITIES
// ─────────────────────────────────────────────────────────────

// hashCode menghasilkan integer non-negatif dari string.
// Digunakan untuk memberi offset unik per kendaraan agar tidak bertumpuk.
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

package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"transit-app/internal/domain"
	"transit-app/internal/logger"
)

type routeUsecase struct {
	routeRepo domain.RouteRepository
}

type CacheEntry struct {
	Data      []domain.Route
	ExpiredAt time.Time
}

var routesCache sync.Map // key: "all_routes", value: CacheEntry

func NewRouteUsecase(routeRepo domain.RouteRepository) domain.RouteUsecase {
	return &routeUsecase{
		routeRepo: routeRepo,
	}
}

func (u *routeUsecase) GetAllActiveRoutes(ctx context.Context) ([]domain.Route, error) {
	// Check cache
	if cached, ok := routesCache.Load("all_routes"); ok {
		entry := cached.(CacheEntry)
		if time.Now().Before(entry.ExpiredAt) {
			logger.Info("Returning routes from cache")
			return entry.Data, nil
		}
	}

	// Fetch from DB
	routes, err := u.routeRepo.FindAllActive(ctx)
	if err != nil {
		return nil, err
	}

	// Save to cache (TTL 5 minutes)
	routesCache.Store("all_routes", CacheEntry{
		Data:      routes,
		ExpiredAt: time.Now().Add(5 * time.Minute),
	})

	return routes, nil
}

func (u *routeUsecase) GetRouteDetails(ctx context.Context, id uuid.UUID) (*domain.Route, error) {
	route, err := u.routeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if route == nil {
		return nil, errors.New("route not found")
	}

	// Fetch stops and schedules
	stops, err := u.routeRepo.FindStopsByRouteID(ctx, id)
	if err != nil {
		return nil, err
	}
	route.Stops = stops

	schedules, err := u.routeRepo.FindSchedulesByRouteID(ctx, id)
	if err != nil {
		return nil, err
	}
	route.Schedules = schedules

	return route, nil
}

func (u *routeUsecase) GetRouteStops(ctx context.Context, id uuid.UUID) ([]domain.Stop, error) {
	return u.routeRepo.FindStopsByRouteID(ctx, id)
}

func (u *routeUsecase) GetJourney(ctx context.Context, fromLat, fromLng, toLat, toLng string) (interface{}, error) {
	apiKey := os.Getenv("TOMTOM_API_KEY")
	if apiKey == "" {
		return nil, errors.New("TOMTOM_API_KEY not configured")
	}

	url := fmt.Sprintf("https://api.tomtom.com/routing/1/calculateRoute/%s,%s:%s,%s/json?key=%s",
		fromLat, fromLng, toLat, toLng, apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TomTom API error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

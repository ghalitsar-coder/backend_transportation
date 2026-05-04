package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"transit-app/internal/domain"
	"transit-app/internal/response"
)

type routeHandler struct {
	routeUsecase domain.RouteUsecase
}

func NewRouteHandler(router *gin.RouterGroup, ru domain.RouteUsecase) {
	handler := &routeHandler{
		routeUsecase: ru,
	}

	router.GET("/routes", handler.GetAllActiveRoutes)
	router.GET("/routes/:id", handler.GetRouteDetails)
	router.GET("/routes/:id/stops", handler.GetRouteStops)
	router.GET("/routes/:id/journey", handler.GetJourney)
}

func (h *routeHandler) GetAllActiveRoutes(c *gin.Context) {
	routes, err := h.routeUsecase.GetAllActiveRoutes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(routes))
}

func (h *routeHandler) GetRouteDetails(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error("INVALID_ID", "Invalid route ID format"))
		return
	}

	route, err := h.routeUsecase.GetRouteDetails(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "route not found" {
			c.JSON(http.StatusNotFound, response.Error("NOT_FOUND", "Route not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(route))
}

func (h *routeHandler) GetRouteStops(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error("INVALID_ID", "Invalid route ID format"))
		return
	}

	stops, err := h.routeUsecase.GetRouteStops(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(stops))
}

func (h *routeHandler) GetJourney(c *gin.Context) {
	fromLat := c.Query("from_lat")
	fromLng := c.Query("from_lng")
	toLat := c.Query("to_lat")
	toLng := c.Query("to_lng")

	if fromLat == "" || fromLng == "" || toLat == "" || toLng == "" {
		c.JSON(http.StatusBadRequest, response.Error("MISSING_PARAMS", "from_lat, from_lng, to_lat, to_lng required"))
		return
	}

	journey, err := h.routeUsecase.GetJourney(c.Request.Context(), fromLat, fromLng, toLat, toLng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error("INTERNAL_ERROR", err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(journey))
}

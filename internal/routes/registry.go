package routes

import "net/http"

const (
	RouteIDPing   = "ping"
	RouteIDOrders = "orders"
	RouteIDReport = "report"
	RouteAll      = "ALL"
)

type ProtectedRouteDefinition struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Cost   int    `json:"cost"`
}

var protectedRouteDefinitions = []ProtectedRouteDefinition{
	{
		ID:     RouteIDPing,
		Method: http.MethodGet,
		Path:   "/api/protected/ping",
		Cost:   1,
	},
	{
		ID:     RouteIDOrders,
		Method: http.MethodPost,
		Path:   "/api/protected/orders",
		Cost:   2,
	},
	{
		ID:     RouteIDReport,
		Method: http.MethodGet,
		Path:   "/api/protected/report",
		Cost:   5,
	},
}

func ProtectedRoutes() []ProtectedRouteDefinition {
	definitions := make([]ProtectedRouteDefinition, len(protectedRouteDefinitions))
	copy(definitions, protectedRouteDefinitions)
	return definitions
}

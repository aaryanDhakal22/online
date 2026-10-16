package transport

import (
	"net/http"

	handler "quicc/online/internal/transport/handlers"

	"github.com/labstack/echo/v5"
)

type CMS struct {
	AuthMiddleware  echo.MiddlewareFunc
	AdminMiddleware echo.MiddlewareFunc
	// Version + Commit are injected from cmd/server/main.go's ldflags-set
	// vars and surfaced via GET /api/v1/version.
	Version string
	Commit  string
}

// RouteInfo is the JSON shape served at GET /__routes (shared across all
// modules: method, path, auth, description).
type RouteInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Auth        string `json:"auth"`
	Description string `json:"description"`
}

type routeDef struct {
	RouteInfo
	Handler echo.HandlerFunc
	MW      []echo.MiddlewareFunc
}

func def(method, path, auth, desc string, h echo.HandlerFunc, mw ...echo.MiddlewareFunc) routeDef {
	return routeDef{RouteInfo{method, path, auth, desc}, h, mw}
}

// RegisterRoutes registers the route table and adds GET /__routes. The table is
// the single source of truth — it both registers with Echo and is serialized at
// /__routes for the console's routes viewer.
func RegisterRoutes(e *echo.Echo, cms *CMS, h *handler.Handler) {
	defs := routeTable(cms, h)
	for _, r := range defs {
		e.Add(r.Method, r.Path, r.Handler, r.MW...)
	}

	infos := make([]RouteInfo, 0, len(defs)+1)
	for _, r := range defs {
		infos = append(infos, r.RouteInfo)
	}
	infos = append(infos, RouteInfo{"GET", "/__routes", "none", "This route list (method/path/auth/description)"})
	e.GET("/__routes", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"module": "online", "version": cms.Version, "routes": infos,
		})
	})
}

func routeTable(cms *CMS, h *handler.Handler) []routeDef {
	ok := func(c *echo.Context) error { return c.String(http.StatusOK, "OK") }
	versionBody := map[string]string{"service": "quicc-online", "version": cms.Version, "commit": cms.Commit}
	version := func(c *echo.Context) error { return c.JSON(http.StatusOK, versionBody) }

	return []routeDef{
		def("GET", "/health", "none", "Liveness — 200 if the process is up", ok),
		def("GET", "/api/v1/healthz", "none", "Liveness (versioned)", ok),
		def("GET", "/api/v1/version", "none", "Build identifiers (version + commit)", version),
		def("GET", "/api/v1/verify", "vendor", "Report whether the supplied bearer matches the active key", h.Verify),
		// Standard mutate flow: anyone /generate (stages a Primed key), admin /set promotes it.
		def("GET", "/api/v1/generate", "none", "Mint a new Primed key (plaintext returned once)", h.Generate),
		def("GET", "/api/v1/set", "admin", "Promote the Primed key to Active (deactivates prior)", h.Set, cms.AdminMiddleware),
		// Break-glass: admin already holds the plaintext and sets it Active directly.
		def("POST", "/api/v1/setKey", "admin", "Insert + activate a plaintext key directly", h.SetKey, cms.AdminMiddleware),
		def("GET", "/api/v1/getKey", "admin", "Return the active key plaintext", h.GetKey, cms.AdminMiddleware),
		// Order ingest — the only path the vendor hits.
		def("POST", "/api/v1/order", "vendor", "Ingest vendor JSON → publish to SQS (202)", h.CreateOrder, cms.AuthMiddleware),
	}
}

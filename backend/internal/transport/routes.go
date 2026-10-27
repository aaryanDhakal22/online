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
		def("GET", "/api/v1/verify", "vendor", "Report whether the supplied bearer matches an active key", h.Verify),
		// Two-step rotation. Add: anyone /generate (stages a Primed key), admin
		// /set promotes it next to the current key (break-glass: admin /setKey
		// adds a known plaintext). Retire: admin retires the old key once the
		// vendor has used the new one. The only active key can never be retired.
		def("GET", "/api/v1/generate", "none", "Mint a new Primed key (plaintext returned once)", h.Generate),
		def("GET", "/api/v1/set", "admin", "Add the Primed key as Active (prior keys stay active)", h.Set, cms.AdminMiddleware),
		def("POST", "/api/v1/setKey", "admin", "Add a plaintext key as Active directly (prior keys stay active)", h.SetKey, cms.AdminMiddleware),
		def("GET", "/api/v1/getKey", "admin", "Return the newest active key plaintext", h.GetKey, cms.AdminMiddleware),
		def("GET", "/api/v1/keys", "admin", "List keys: id/status/created_at/last_used_at (no plaintext)", h.ListKeys, cms.AdminMiddleware),
		def("POST", "/api/v1/keys/:id/retire", "admin", "Deactivate a key (409 last_active_key / new_key_unconfirmed)", h.RetireKey, cms.AdminMiddleware),
		// Order ingest — the only path the vendor hits.
		def("POST", "/api/v1/order", "vendor", "Ingest vendor JSON → publish to SQS (202)", h.CreateOrder, cms.AuthMiddleware),
	}
}

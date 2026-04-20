package app

import (
	"net/http"

	httpapi "tars/internal/api/http"
	"tars/internal/api/http/middleware"
	"tars/internal/modules/org"
)

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, a.HTTPDependencies())

	// ORG-N4: inject org context from request headers/query params into ctx
	return middleware.RequestID(org.OrgContextMiddleware(a.services.Org, mux))
}

func (a *App) OpsHandler() http.Handler {
	mux := http.NewServeMux()
	httpapi.RegisterOpsRoutes(mux, a.HTTPDependencies())
	httpapi.RegisterConnectorRegistryRoutes(mux, a.HTTPDependencies())

	return middleware.RequestID(org.OrgContextMiddleware(a.services.Org, mux))
}

package api

import (
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"
	"github.com/lu1a/lcaas/core-service/api/auth"
	"github.com/lu1a/lcaas/core-service/api/containerOps"
	"github.com/lu1a/lcaas/core-service/types"
)

func APIRouter(log log.Logger, db *sqlx.DB, kubeClients []types.ContainerZone, config types.Config) *http.ServeMux {
	r := http.NewServeMux()
	authLog := log.With("auth")
	containerOpsLog := log.With("container-ops")
	r.Handle("/auth/", http.StripPrefix("/auth", auth.AuthRouter(authLog, db, &config)))
	r.Handle("/", containerOps.ContaineropsRouter(containerOpsLog, db, &config, kubeClients))
	return r
}

package frontend

import (
	"github.com/lu1a/lcaas/core-service/kubeOps"
	"github.com/lu1a/lcaas/core-service/types"
)

type NavProps struct {
	Account   types.Account
	PageTitle string
}

type IIndexResponse struct {
	Account  types.Account
	NavProps NavProps
	Projects []types.Project
}

type IAccountSettingsResponse struct {
	Account  types.Account
	NavProps NavProps

	// in case the page is loaded as a redirect from /account/generate-auth-token
	NewAPIToken string
}

type IProjectResponse struct {
	Account  types.Account
	NavProps NavProps

	Project     types.Project
	ProjectName string

	Containers     []types.ContainerClaim
	UserDBClaim    types.UserDBClaim
	ObjectStorages []types.ObjectStorageClaim
}

type INewContainerResponse struct {
	Account     types.Account
	NavProps    NavProps
	Project     types.Project
	ProjectName string

	Zones []string
}

type IContainerLogsResponse struct {
	Account     types.Account
	NavProps    NavProps
	ProjectName string

	Container          types.ContainerClaim
	LatestLogsForZones []kubeOps.LogsForZone
}

type INewDBResponse struct {
	Account     types.Account
	NavProps    NavProps
	Project     types.Project
	ProjectName string

	Zones []string
}

type IUserDBDetailsResponse struct {
	Account  types.Account
	NavProps NavProps

	Project types.Project
	UserDB  types.UserDBClaim
}

type INewObjectStorageResponse struct {
	Account     types.Account
	NavProps    NavProps
	Project     types.Project
	ProjectName string

	Zones []string
}

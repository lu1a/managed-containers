package containerOps

import (
	"github.com/lu1a/lcaas/core-service/kubeOps"
	"github.com/lu1a/lcaas/core-service/types"
)

/*
Route: /api/project/{projectName}/get-all-containers
Type: query
*/
type IGetAllContainersResponse struct {
	Containers []types.ContainerClaim `json:"containers"`
}

/*
Route: /api/project/{projectName}/container/{containerName}
Type: query
*/
type IGetContainerResponse struct {
	Container    types.ContainerClaim  `json:"container"`
	LogsForZones []kubeOps.LogsForZone `json:"logs"`
}

/*
Route: /api/project/{projectName}/create-container
Type: query
*/
type ICreateContainerResponse struct {
	Container types.ContainerClaim `json:"container"`
}

/*
Route: /api/project/{projectName}/container/{containerName}/delete
Type: query
*/
type IDeleteContainerResponse struct {
	Container types.ContainerClaim `json:"container"`
}

/*
Route: /api/project/{projectName}/container/{containerName}/rerun-once
Type: query
*/
type IRerunContainerResponse struct {
	Container types.ContainerClaim `json:"container"`
}

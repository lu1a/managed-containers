package containerOps

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lu1a/lcaas/core-service/db"
	"github.com/lu1a/lcaas/core-service/kubeOps"
	"github.com/lu1a/lcaas/core-service/types"

	"github.com/charmbracelet/log"

	"github.com/jmoiron/sqlx"
)

func ContaineropsRouter(log *log.Logger, adminDB *sqlx.DB, config *types.Config, kubeClients []types.ContainerZone) *http.ServeMux {
	r := http.NewServeMux()
	// Everything's POST, to reduce argument over REST stupidity

	// Get all containers
	r.HandleFunc("POST /project/{projectName}/get-all-containers", func(w http.ResponseWriter, r *http.Request) {
		apiResponse := IGetAllContainersResponse{}
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		containerClaimList, err := db.GetContainersByProject(adminDB, thisProject)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		apiResponse.Containers = containerClaimList

		apiResponseJSON, err := json.Marshal(apiResponse)
		if err != nil {
			http.Error(w, "Error encoding to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(apiResponseJSON)
		if err != nil {
			http.Error(w, "Error writing out JSON", http.StatusNotFound)
		}
	})

	// Get container by name, including logs
	r.HandleFunc("POST /project/{projectName}/container/{containerName}", func(w http.ResponseWriter, r *http.Request) {
		apiResponse := IGetContainerResponse{}
		projectName := r.PathValue("projectName")
		containerName := r.PathValue("containerName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		containerClaim, err := db.GetContainerByProjectAndName(adminDB, thisProject, containerName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		apiResponse.Container = containerClaim

		logs, err := kubeOps.GetContainerLogs(*log, kubeClients, thisProject, containerClaim)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		apiResponse.LogsForZones = logs

		apiResponseJSON, err := json.Marshal(apiResponse)
		if err != nil {
			http.Error(w, "Error encoding to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(apiResponseJSON)
		if err != nil {
			http.Error(w, "Error writing out JSON", http.StatusNotFound)
		}
	})

	// Create a container
	r.HandleFunc("POST /project/{projectName}/create-container", func(w http.ResponseWriter, r *http.Request) {
		apiResponse := ICreateContainerResponse{}
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		newContainer := types.ContainerClaim{}
		newContainer, err = newContainer.ParseContainerFieldsFromHTTPFormZoneProject(r, types.GetZonesFromContainerZones(kubeClients), thisProject.ProjectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		newContainer, err = db.CreateContainerClaimForProject(adminDB, account, thisProject, newContainer)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// actually go and create the container
		go func() {
			err = kubeOps.CreateContainerFromClaim(*log, adminDB, kubeClients, thisProject, newContainer, true)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}()
		apiResponse.Container = newContainer

		apiResponseJSON, err := json.Marshal(apiResponse)
		if err != nil {
			http.Error(w, "Error encoding to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(apiResponseJSON)
		if err != nil {
			http.Error(w, "Error writing out JSON", http.StatusNotFound)
		}
	})

	// Delete a container
	r.HandleFunc("POST /project/{projectName}/container/{containerName}/delete", func(w http.ResponseWriter, r *http.Request) {
		apiResponse := IDeleteContainerResponse{}
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		containerName := r.PathValue("containerName")
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		thisContainer, err := db.GetContainerByProjectAndName(adminDB, thisProject, containerName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = db.SetContainerAsDeactivating(adminDB, thisContainer)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// actually go and delete the container
		go func() {
			err = kubeOps.DeleteContainer(*log, kubeClients, thisProject, thisContainer, false)
			if err != nil {
				log.Error(err.Error())
			}

			err = db.DeleteContainerByProjectAndName(adminDB, thisProject, containerName)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}()
		apiResponse.Container = thisContainer

		apiResponseJSON, err := json.Marshal(apiResponse)
		if err != nil {
			http.Error(w, "Error encoding to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(apiResponseJSON)
		if err != nil {
			http.Error(w, "Error writing out JSON", http.StatusNotFound)
		}
	})

	// Re-run a container
	r.HandleFunc("POST /project/{projectName}/container/{containerName}/rerun-once", func(w http.ResponseWriter, r *http.Request) {
		apiResponse := IDeleteContainerResponse{}
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		containerName := r.PathValue("containerName")
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		oldContainer, err := db.GetContainerByProjectAndName(adminDB, thisProject, containerName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// retain the same env vars, since we can't recreate them (we don't know the values of the secrets)
		newContainer := oldContainer
		for _, envVarName := range oldContainer.EnvVarNames {
			if strings.HasSuffix(envVarName, "image-pull-secret") { // do the image-pull-secret separately
				continue
			}
			newContainer.EnvVars = append(newContainer.EnvVars, types.EnvVar{Name: envVarName})
		}
		newContainer.ImagePullSecret = &types.ImagePullSecret{}

		err = db.SetContainerAsDeactivating(adminDB, oldContainer)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// delete the old container, then instantiate the new one
		go func() {
			err = kubeOps.DeleteContainer(*log, kubeClients, thisProject, oldContainer, true)
			if err != nil {
				log.Error(err.Error())
				return
			}
			err = db.DeleteContainerByProjectAndName(adminDB, thisProject, containerName)
			if err != nil {
				log.Error(err.Error())
				return
			}

			newContainer, err = db.CreateContainerClaimForProject(adminDB, account, thisProject, newContainer)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = kubeOps.CreateContainerFromClaim(*log, adminDB, kubeClients, thisProject, newContainer, true)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}()
		apiResponse.Container = newContainer

		apiResponseJSON, err := json.Marshal(apiResponse)
		if err != nil {
			http.Error(w, "Error encoding to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(apiResponseJSON)
		if err != nil {
			http.Error(w, "Error writing out JSON", http.StatusNotFound)
		}
	})
	// ...
	return r
}

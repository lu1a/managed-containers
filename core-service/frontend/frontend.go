package frontend

import (
	"fmt"
	"net/http"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"
	"github.com/lu1a/lcaas/core-service/db"
	"github.com/lu1a/lcaas/core-service/kubeOps"
	"github.com/lu1a/lcaas/core-service/postgresOps"
	"github.com/lu1a/lcaas/core-service/types"
)

func FrontendRouter(log log.Logger, adminDB *sqlx.DB, kubeClients []types.ContainerZone, config types.Config) *http.ServeMux {
	r := http.NewServeMux()
	r.Handle("GET /static/*", http.StripPrefix("/static/", http.FileServer(http.Dir(path.Join("frontend", "static")))))

	r.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "login.html"),
		}
		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.ExecuteTemplate(w, "base", config.ListenURL); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fp := path.Join("frontend", "templates", "pages", "static-landing.html")
		tmpl, err := template.ParseFiles(fp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, struct{}{}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("GET /home", func(w http.ResponseWriter, r *http.Request) {
		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "index.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}
		respData := IIndexResponse{}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: "Home"}

		projects, err := db.GetProjectsByAccount(adminDB, account)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respData.Projects = projects

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("GET /project/{projectName}", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "project.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}
		respData := IProjectResponse{ProjectName: projectName}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}

		respData.Project, err = db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("GET /project/{projectName}/settings", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "project-settings.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}
		respData := IProjectResponse{ProjectName: projectName}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}

		respData.Project, err = db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("GET /new-project", func(w http.ResponseWriter, r *http.Request) {
		respData := NavProps{}
		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "new-project.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.PageTitle = ""

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("POST /new-project", func(w http.ResponseWriter, r *http.Request) {
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newProject := types.Project{
			Name:        r.FormValue("name"),
			Description: r.FormValue("description"),
		}

		createdProject, err := db.CreateProjectForAccount(adminDB, account, newProject)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/project/%s", createdProject.Name), http.StatusSeeOther)
	})

	r.HandleFunc("GET /project/{projectName}/containers", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "project-containers.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}
		respData := IProjectResponse{ProjectName: projectName}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}

		respData.Project, err = db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		respData.Containers, err = db.GetContainersByProject(adminDB, respData.Project)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("GET /project/{projectName}/new-container", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		respData := INewContainerResponse{}

		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "new-container.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}
		respData.ProjectName = projectName
		respData.Zones = types.GetZonesFromContainerZones(kubeClients)

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("POST /project/{projectName}/new-container", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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
			err = kubeOps.CreateContainerFromClaim(log, adminDB, kubeClients, thisProject, newContainer, false)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}()

		http.Redirect(w, r, fmt.Sprintf("/project/%s", projectName), http.StatusSeeOther)
	})

	r.HandleFunc("POST /project/{projectName}/{containerName}/delete-container", func(w http.ResponseWriter, r *http.Request) {
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
			err = kubeOps.DeleteContainer(log, kubeClients, thisProject, thisContainer, false)
			if err != nil {
				log.Error(err.Error())
			}

			err = db.DeleteContainerByProjectAndName(adminDB, thisProject, containerName)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}()

		http.Redirect(w, r, fmt.Sprintf("/project/%s", projectName), http.StatusSeeOther)
	})

	r.HandleFunc("POST /project/{projectName}/{containerName}/rerun-container-once", func(w http.ResponseWriter, r *http.Request) {
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
			err = kubeOps.DeleteContainer(log, kubeClients, thisProject, oldContainer, true)
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
			err = kubeOps.CreateContainerFromClaim(log, adminDB, kubeClients, thisProject, newContainer, true)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}()

		http.Redirect(w, r, fmt.Sprintf("/project/%s", projectName), http.StatusSeeOther)
	})

	r.HandleFunc("GET /project/{projectName}/c/{containerName}/logs", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		containerName := r.PathValue("containerName")
		respData := IContainerLogsResponse{}

		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "container-logs.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}

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
		respData.Container = thisContainer

		logs, err := kubeOps.GetContainerLogs(log, kubeClients, thisProject, thisContainer)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respData.LatestLogsForZones = logs

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("GET /project/{projectName}/database", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "project-database.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}
		respData := IProjectResponse{ProjectName: projectName}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}

		respData.Project, err = db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		pretendToHaveMultipleUserDBClaims, err := db.GetUserDBClaimsByProject(adminDB, respData.Project)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(pretendToHaveMultipleUserDBClaims) > 0 {
			respData.UserDBClaim = pretendToHaveMultipleUserDBClaims[0]
		}

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("GET /project/{projectName}/new-db", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		respData := INewDBResponse{}

		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "new-db.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}
		respData.ProjectName = projectName
		respData.Zones = config.GetZonesFromUserDBConnections()

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("POST /project/{projectName}/new-db", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(r.Form["zone"]) != 1 {
			http.Error(w, "Choose 1 and only 1 zone", http.StatusInternalServerError)
			return
		}

		newUserDBClaim := types.UserDBClaim{
			ProjectID: thisProject.ProjectID,
			Zones:     []string{r.FormValue("zone")},
		}

		newUserDBClaim, err = db.CreateUserDBClaimForProject(adminDB, thisProject, newUserDBClaim)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// actually go and create database for this project
		go func() {
			err = postgresOps.CreateDatabaseForProject(log, adminDB, config.UserDBConnections, thisProject, newUserDBClaim)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}()

		http.Redirect(w, r, fmt.Sprintf("/project/%s", projectName), http.StatusSeeOther)
	})

	r.HandleFunc("GET /project/{projectName}/db/{userDBName}", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		userDBName := r.PathValue("userDBName")
		respData := IUserDBDetailsResponse{}

		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "user-db-details.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}

		log.Debug("Logging this for no reason", "userDBName", userDBName)
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respData.Project = thisProject

		thisUserDBClaim, err := db.GetUserDBClaimByProject(adminDB, thisProject)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respData.UserDB = thisUserDBClaim

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("POST /project/{projectName}/db/{userDBName}/new-user", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		userDBName := r.PathValue("userDBName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		thisUserDBClaim, err := db.GetUserDBClaimByProject(adminDB, thisProject)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = postgresOps.CreateNewUserForUserDB(log, adminDB, config.UserDBConnections, thisProject, thisUserDBClaim, r.FormValue("username"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/project/%s/db/%s", projectName, userDBName), http.StatusSeeOther)
	})

	r.HandleFunc("POST /project/{projectName}/delete-db", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		thisUserDBClaim, err := db.GetUserDBClaimByProject(adminDB, thisProject)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// actually go and delete database
		go func() {
			err = postgresOps.DeleteDatabaseForProject(log, adminDB, config.UserDBConnections, thisProject, thisUserDBClaim)
			if err != nil {
				log.Error(err.Error())
				return
			}
		}()

		http.Redirect(w, r, fmt.Sprintf("/project/%s", projectName), http.StatusSeeOther)
	})

	r.HandleFunc("GET /project/{projectName}/new-object-storage", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		respData := INewObjectStorageResponse{}

		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "new-object-storage.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: projectName}
		respData.ProjectName = projectName
		respData.Zones = types.GetZonesFromContainerZones(kubeClients)

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.HandleFunc("POST /project/{projectName}/new-object-storage", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		zones := []string{}
		for i := 0; i < len(r.Form["zone[]"]); i++ {
			zone := r.Form["zone[]"][i]
			if zone != "" {
				zones = append(zones, zone)
			}
		}
		if len(zones) == 0 {
			zones = types.GetZonesFromContainerZones(kubeClients)
		}

		newObjectStorage := types.ObjectStorageClaim{
			Name:      r.FormValue("name"),
			ProjectID: thisProject.ProjectID,
			Zones:     zones,
		}

		_, err = db.CreateObjectStorageForProject(adminDB, thisProject, newObjectStorage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// TODO: actually go and create the object storage

		http.Redirect(w, r, fmt.Sprintf("/project/%s", projectName), http.StatusSeeOther)
	})

	r.HandleFunc("POST /project/{projectName}/delete-object-storage", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		objectStorageName := r.FormValue("object-storage-name")
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		thisObjectStorage, err := db.GetObjectStorageByProjectAndName(adminDB, thisProject, objectStorageName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = db.SetObjectStorageAsDeactivating(adminDB, thisObjectStorage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// TODO: actually go and delete the object storage

		err = db.DeleteObjectStorageByProjectAndName(adminDB, thisProject, objectStorageName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/project/%s", projectName), http.StatusSeeOther)
	})

	r.HandleFunc("POST /project/{projectName}/share-with-user", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.PathValue("projectName")
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		thisProject, err := db.GetProjectByAccountAndName(adminDB, account, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		usernameToAdd := r.FormValue("username-to-add")
		err = db.AddUserToProjectByUsername(adminDB, thisProject, usernameToAdd)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/project/%s", projectName), http.StatusSeeOther)
	})

	r.HandleFunc("GET /account-settings", func(w http.ResponseWriter, r *http.Request) {
		respData := IAccountSettingsResponse{}

		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "account-settings.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: "Account settings"}

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// have to do this freaky thing so that users don't accidentally reload page and get a white screen
	r.HandleFunc("POST /account-settings", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/account-settings", http.StatusSeeOther)
	})

	r.HandleFunc("POST /account/generate-api-token", func(w http.ResponseWriter, r *http.Request) {
		respData := IAccountSettingsResponse{}

		fps := []string{
			path.Join("frontend", "templates", "components", "base.html"),
			path.Join("frontend", "templates", "pages", "account-settings.html"),
			path.Join("frontend", "templates", "components", "nav.html"),
		}

		tmpl, err := template.ParseFiles(fps...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		value := r.Context().Value(types.Account{})
		account := value.(types.Account)
		respData.Account = account
		respData.NavProps = NavProps{Account: account, PageTitle: "Account settings"}

		newAPITokenBytes, err := exec.Command("uuidgen").Output()
		if err != nil {
			nicerErr := fmt.Errorf("Error generating auth token: %v", err)
			http.Error(w, nicerErr.Error(), http.StatusInternalServerError)
			return
		}
		newAPIToken := strings.Join(strings.Fields(string(newAPITokenBytes)), "")

		// TODO: save auth token
		err = db.InsertNewAPITokenForAccount(adminDB, account, newAPIToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		respData.NewAPIToken = newAPIToken

		if err := tmpl.ExecuteTemplate(w, "base", respData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return r
}

package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lu1a/lcaas/core-service/types"
)

type numRange struct {
	low int
	hi  int
}

func InitialiseContainerZones(adminDB *sqlx.DB, kubeClients []types.ContainerZone) error {
	for _, client := range kubeClients {
		_, err := adminDB.Exec("INSERT INTO container_zone (name, default_routing_ip) VALUES($1, $2) ON CONFLICT DO NOTHING", client.Name, client.DefaultRoutingIP)
		if err != nil {
			return fmt.Errorf("Initialising container_zones failed: %w", err)
		}
	}
	return nil
}

func GetContainerZonesFromDB(adminDB *sqlx.DB) (containerZones []types.ContainerZone, err error) {
	err = adminDB.Select(&containerZones, "SELECT * FROM container_zone")
	if err != nil {
		return containerZones, fmt.Errorf("Getting container_zones failed: %w", err)
	}

	return containerZones, nil
}

func GetAccountByAPIToken(adminDB *sqlx.DB, apiToken string) (account types.Account, err error) {
	query := `
		SELECT account.* FROM account
		JOIN api_token ON account.account_id = api_token.account_id
		WHERE api_token.token = $1 AND account.deleted_at IS NULL
	`

	err = adminDB.Get(&account, query, apiToken)
	if err != nil {
		return account, err
	}

	return account, nil
}

func InsertNewAPITokenForAccount(adminDB *sqlx.DB, account types.Account, newAPIToken string) (err error) {
	tx, err := adminDB.Begin()
	if err != nil {
		return err
	}

	deletePreviousAPITokensQuery := `DELETE FROM api_token WHERE account_id = $1`
	createNewAPITokenQuery := `INSERT INTO api_token (token, account_id) VALUES ($1, $2)`

	_, err = tx.Exec(deletePreviousAPITokensQuery, account.AccountID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	_, err = tx.Exec(createNewAPITokenQuery, newAPIToken, account.AccountID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func GetAccountBySession(adminDB *sqlx.DB, sessionToken string) (account types.Account, err error) {
	query := `
		SELECT account.* FROM account
		JOIN session ON account.account_id = session.account_id
		WHERE session.token = $1 AND account.deleted_at IS NULL
	`

	err = adminDB.Get(&account, query, sessionToken)
	if err != nil {
		return account, err
	}

	return account, nil
}

func UpsertAccountViaGitHub(adminDB *sqlx.DB, accessToken, sessionToken string, gitHubUser types.GitHubAccountProfile) (err error) {
	// Check if gitHubUser exists
	var count int
	err = adminDB.Get(&count, "SELECT COUNT(*) FROM github_account_profile WHERE user_profile_id = $1", gitHubUser.UserProfileID)
	if err != nil {
		return fmt.Errorf("Checking if github user exists failed: %w", err)
	}

	// If it does, create a new session for the account
	if count > 0 {
		var sessionWithThisTokenCount int
		err = adminDB.Get(&sessionWithThisTokenCount, "SELECT COUNT(*) FROM session WHERE token = $1", sessionToken)
		if err != nil {
			return fmt.Errorf("Checking sessions with this token failed: %w", err)
		} else if sessionWithThisTokenCount > 0 {
			return nil
		}

		var account types.Account
		err = adminDB.Get(&account, "SELECT account.* FROM account JOIN github_account_profile ON account.account_id = github_account_profile.account_id WHERE github_account_profile.user_profile_id = $1", gitHubUser.UserProfileID)
		if err != nil {
			return fmt.Errorf("Getting existing account failed: %w", err)
		}

		_, err = adminDB.Exec("INSERT INTO session (token, account_id) VALUES($1, $2)", sessionToken, account.AccountID)
		if err != nil {
			return fmt.Errorf("Creating new session for existing account failed: %w", err)
		}

		return nil
	}

	// if it doesn't, create a new account, create a new gitHubUser, then create a new session for the account
	var accountID int
	err = adminDB.QueryRow("INSERT INTO account (name, username, email, location, avatar_url) VALUES($1, $2, $3, $4, $5) RETURNING account_id", gitHubUser.Name, gitHubUser.Login, gitHubUser.Email, gitHubUser.Location, gitHubUser.AvatarURL).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("Creating new account failed: %w", err)
	}
	err = initDefaultResourceUsagesForAccountID(adminDB, accountID)
	if err != nil {
		return err
	}
	gitHubUser.AccountID = accountID

	gitHubAccountProfileInsertQuery := `
		INSERT INTO github_account_profile (
			account_id,
			user_profile_id,
			avatar_url,
			bio,
			blog,
			company,
			created_at,
			email,
			events_url,
			followers,
			followers_url,
			following,
			following_url,
			gists_url,
			gravatar_id,
			hireable,
			html_url,
			location,
			login,
			name,
			node_id,
			organizations_url,
			public_gists,
			public_repos,
			received_events_url,
			repos_url,
			site_admin,
			starred_url,
			subscriptions_url,
			twitter_username,
			user_type,
			updated_at,
			url
		) VALUES (
			:account_id,
			:user_profile_id,
			:avatar_url,
			:bio,
			:blog,
			:company,
			:created_at,
			:email,
			:events_url,
			:followers,
			:followers_url,
			:following,
			:following_url,
			:gists_url,
			:gravatar_id,
			:hireable,
			:html_url,
			:location,
			:login,
			:name,
			:node_id,
			:organizations_url,
			:public_gists,
			:public_repos,
			:received_events_url,
			:repos_url,
			:site_admin,
			:starred_url,
			:subscriptions_url,
			:twitter_username,
			:user_type,
			:updated_at,
			:url
		)
	`
	_, err = adminDB.NamedExec(gitHubAccountProfileInsertQuery, gitHubUser)
	if err != nil {
		return fmt.Errorf("Creating new github profile failed: %w", err)
	}
	_, err = adminDB.Exec("INSERT INTO session (token, account_id) VALUES($1, $2)", sessionToken, gitHubUser.AccountID)
	if err != nil {
		return fmt.Errorf("Creating new session failed: %w", err)
	}

	return nil
}

func GetProjectsByAccount(adminDB *sqlx.DB, account types.Account) (projects []types.Project, err error) {
	query := `
		SELECT project.* FROM project
		JOIN account_project ON project.project_id = account_project.project_id
		WHERE account_project.account_id = $1 AND project.deleted_at IS NULL
	`

	err = adminDB.Select(&projects, query, account.AccountID)
	if err != nil {
		return projects, err
	}

	return projects, nil
}

func GetProjectByAccountAndName(adminDB *sqlx.DB, account types.Account, projectName string) (project types.Project, err error) {
	query := `
		SELECT project.* FROM project
		JOIN account_project ON project.project_id = account_project.project_id
		WHERE account_project.account_id = $1 AND project.name = $2 AND project.deleted_at IS NULL
	`

	err = adminDB.Get(&project, query, account.AccountID, projectName)
	if err != nil {
		return project, err
	}

	sharedUsernamesQuery := `
		SELECT account.username FROM account
		JOIN account_project ON account.account_id = account_project.account_id
		WHERE account_project.project_id = $1 AND account.deleted_at IS NULL
	`

	sharedUsernames := []string{}
	err = adminDB.Select(&sharedUsernames, sharedUsernamesQuery, project.ProjectID)
	if err != nil {
		return project, err
	}
	project.SharedUsernames = sharedUsernames

	return project, nil
}

func initDefaultResourceUsagesForAccountID(adminDB *sqlx.DB, accountID int) error {
	containerZones, err := GetContainerZonesFromDB(adminDB)
	if err != nil {
		return err
	}

	for _, containerZone := range containerZones {
		_, err := adminDB.Exec("INSERT INTO container_resource_usage_per_account_per_zone (zone_name, account_id) VALUES ($1, $2)", containerZone.Name, accountID)
		if err != nil {
			return fmt.Errorf("Initialising default resource usages for container_zone %s account %v failed: %w", containerZone.Name, accountID, err)
		}
	}

	return nil
}

func CreateProjectForAccount(adminDB *sqlx.DB, account types.Account, projectInput types.Project) (projectOutput types.Project, err error) {
	tx, err := adminDB.Begin()
	if err != nil {
		return projectOutput, err
	}

	createProjectQuery := `
        WITH inserted_project AS (
            INSERT INTO project (name, description)
            VALUES ($1, $2)
            RETURNING project_id
        )
        SELECT project_id FROM inserted_project
    `
	createAccountProjectMapQuery := `INSERT INTO account_project (account_id, project_id) VALUES ($1, $2)`
	initialBillingRowQuery := `INSERT INTO billing (current_credits, credits_delta, details, project_id) VALUES ($1, $2, $3, $4)`

	// Insert project and get its ID
	var projectID int
	err = tx.QueryRow(createProjectQuery, projectInput.Name, projectInput.Description).Scan(&projectID)
	if err != nil {
		_ = tx.Rollback()
		return projectOutput, err
	}

	// Insert into account_project table
	_, err = tx.Exec(createAccountProjectMapQuery, account.AccountID, projectID)
	if err != nil {
		_ = tx.Rollback()
		return projectOutput, err
	}

	// Insert inital billing row of 0 credits
	_, err = tx.Exec(initialBillingRowQuery, 0, 0, fmt.Sprintf("{\"project\": {\"id\": \"%d\", \"name\": \"%s\" }}", projectID, projectInput.Name), projectID)
	if err != nil {
		_ = tx.Rollback()
		return projectOutput, err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return projectOutput, err
	}

	// Populate projectOutput with the inserted project details
	projectOutput = projectInput
	projectOutput.ProjectID = projectID

	return projectOutput, nil
}

func AddUserToProjectByUsername(adminDB *sqlx.DB, project types.Project, usernameToAdd string) error {
	tx, err := adminDB.Begin()
	if err != nil {
		return err
	}

	findAccountByUsernameQuery := `SELECT account_id FROM account WHERE username = $1 AND account.deleted_at IS NULL`
	createLinkBetweenAccountAndProjectQuery := `INSERT INTO account_project (account_id, project_id) VALUES ($1, $2)`

	var foundAccountID int
	err = tx.QueryRow(findAccountByUsernameQuery, usernameToAdd).Scan(&foundAccountID)
	if err != nil {
		_ = tx.Rollback()
		return err
	} else if foundAccountID == 0 {
		return fmt.Errorf("There was no user found")
	}

	_, err = tx.Exec(createLinkBetweenAccountAndProjectQuery, foundAccountID, project.ProjectID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func CreateContainerClaimForProject(adminDB *sqlx.DB, account types.Account, project types.Project, containerInput types.ContainerClaim) (containerOutput types.ContainerClaim, err error) {
	if containerInput.CPUMilliCores == 0 {
		containerInput.CPUMilliCores = 100
	}
	if containerInput.MemoryMB == 0 {
		containerInput.MemoryMB = 256
	}

	createContainerQuery := `
		WITH inserted_container_claim AS (
			INSERT INTO container_claim (created_by_account_id, project_id, name, image_ref, image_tag, run_type, command, ports, target_ports, zones, env_var_names, cpu_millicores, memory_mb)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING container_claim_id
		)
		SELECT container_claim_id FROM inserted_container_claim
	`

	mayAccountFitThisContainerWithoutGoingOverResourceQuota, err := mayAccountFitThisContainerWithoutGoingOverResourceQuota(adminDB, containerInput, account)
	if err != nil {
		return containerOutput, err
	}
	if !mayAccountFitThisContainerWithoutGoingOverResourceQuota {
		return containerOutput, fmt.Errorf("If you were to deploy this, you'd exceed your allocated resources. Please lower the CPU or RAM.")
	}

	containerInput.Name, err = appendNumberToContainerNameIfExists(adminDB, project, containerInput.Name)
	if err != nil {
		return containerOutput, err
	}

	var containerID int
	err = adminDB.QueryRow(createContainerQuery, account.AccountID, project.ProjectID, containerInput.Name, containerInput.ImageRef, containerInput.ImageTag, containerInput.RunType, containerInput.Command, containerInput.Ports, containerInput.TargetPorts, containerInput.Zones, containerInput.EnvVarNames, containerInput.CPUMilliCores, containerInput.MemoryMB).Scan(&containerID)
	if err != nil {
		return containerOutput, err
	}

	containerOutput = containerInput
	containerOutput.CreatedByAccountID = account.AccountID
	containerOutput.ContainerClaimID = containerID

	return containerOutput, nil
}

func mayAccountFitThisContainerWithoutGoingOverResourceQuota(adminDB *sqlx.DB, containerInput types.ContainerClaim, account types.Account) (mayAccountFitThisContainerWithoutGoingOverResourceQuota bool, err error) {
	for _, zoneName := range containerInput.Zones {
		err = adminDB.Get(&mayAccountFitThisContainerWithoutGoingOverResourceQuota, `
		SELECT (
			cz.cpu_millicores / (SELECT COUNT(*) FROM account a WHERE a.suspended_at IS NULL AND a.deleted_at IS NULL)) > (ru.used_cpu_millicores + $1)
		FROM container_resource_usage_per_account_per_zone ru
		JOIN container_zone cz ON ru.zone_name = cz.name
		WHERE ru.zone_name = $2 AND ru.account_id = $3
		`, containerInput.CPUMilliCores, zoneName, account.AccountID)
		if err != nil {
			return false, fmt.Errorf("Determining whether this account may provision another resource failed: %w", err)
		}
		if !mayAccountFitThisContainerWithoutGoingOverResourceQuota {
			return false, nil
		}

		err = adminDB.Get(&mayAccountFitThisContainerWithoutGoingOverResourceQuota, `
		SELECT (
			cz.memory_mb / (SELECT COUNT(*) FROM account a WHERE a.suspended_at IS NULL AND a.deleted_at IS NULL)) > (ru.used_memory_mb + $1)
		FROM container_resource_usage_per_account_per_zone ru
		JOIN container_zone cz ON ru.zone_name = cz.name
		WHERE ru.zone_name = $2 AND ru.account_id = $3
		`, containerInput.MemoryMB, zoneName, account.AccountID)
		if err != nil {
			return false, fmt.Errorf("Determining whether this account may provision another resource failed: %w", err)
		}
	}

	return mayAccountFitThisContainerWithoutGoingOverResourceQuota, nil
}

func addToContainerResourceUsage(adminDB *sqlx.DB, container types.ContainerClaim) error {
	if container.CreatedByAccountID == 0 {
		return fmt.Errorf("Adding to container resource usage for account %v failed: there is no account ID", container.CreatedByAccountID)
	}
	for _, zoneName := range container.Zones {
		_, err := adminDB.Exec(`
		UPDATE container_resource_usage_per_account_per_zone
		SET used_cpu_millicores = used_cpu_millicores + $1, used_memory_mb = used_memory_mb + $2
		WHERE zone_name = $3 AND account_id = $4
		`, container.CPUMilliCores, container.MemoryMB, zoneName, container.CreatedByAccountID)
		if err != nil {
			return fmt.Errorf("Adding to container resource usage for account %v failed: %w", container.CreatedByAccountID, err)
		}
	}
	return nil
}

func removeFromContainerResourceUsage(adminDB *sqlx.DB, container types.ContainerClaim) error {
	if container.CreatedByAccountID == 0 {
		return fmt.Errorf("Removing container resource usage for account %v failed: there is no account ID", container.CreatedByAccountID)
	}
	for _, zoneName := range container.Zones {
		_, err := adminDB.Exec(`
		UPDATE container_resource_usage_per_account_per_zone
		SET used_cpu_millicores = used_cpu_millicores - $1, used_memory_mb = used_memory_mb - $2
		WHERE zone_name = $3 AND account_id = $4
		`, container.CPUMilliCores, container.MemoryMB, zoneName, container.CreatedByAccountID)
		if err != nil {
			if strings.Contains(err.Error(), "violates check constraint") { // when removing usage would make user dip below 0
				_, err := adminDB.Exec(`
				UPDATE container_resource_usage_per_account_per_zone
				SET used_cpu_millicores = 0, used_memory_mb = 0
				WHERE zone_name = $1 AND account_id = $2
				`, zoneName, container.CreatedByAccountID)
				if err != nil {
					return fmt.Errorf("Nuking resource usage for account %v failed: %w", container.CreatedByAccountID, err)
				}
			} else {
				// all regular errors
				return fmt.Errorf("Removing container resource usage for account %v failed: %w", container.CreatedByAccountID, err)
			}

		}
	}
	return nil
}

func appendNumberToContainerNameIfExists(adminDB *sqlx.DB, project types.Project, containerName string) (outputContainerName string, err error) {
	getLatestContainerNameQuery := `
		SELECT name FROM container_claim
		WHERE project_id = $1 AND name LIKE $2 AND status != 'inactive'
		ORDER BY created_at DESC
		LIMIT 1
	`

	err = adminDB.QueryRow(getLatestContainerNameQuery, project.ProjectID, containerName+"%").Scan(&outputContainerName)
	if err != nil && err != sql.ErrNoRows {
		return outputContainerName, err
	}

	// If there was nothing returned, just assume the container name
	if len(outputContainerName) == 0 {
		return containerName, nil
	}

	// Split the string by '-' delimiter
	parts := strings.Split(outputContainerName, "-")

	// Get the last part of the split string
	lastPart := parts[len(parts)-1]

	// Check if the last part is a number
	_, err = strconv.Atoi(lastPart)
	if err != nil {
		// If it's not a number, append "-1" to the input string
		return outputContainerName + "-1", nil
	}

	// If it's a number, increment it by 1 and append to the input string
	num, _ := strconv.Atoi(lastPart)
	num++
	newLastPart := strconv.Itoa(num)
	return strings.Join(parts[:len(parts)-1], "-") + "-" + newLastPart, nil
}

func GetContainersByProject(adminDB *sqlx.DB, project types.Project) (containers []types.ContainerClaim, err error) {
	query := `
		SELECT * FROM container_claim
		WHERE project_id = $1 AND deleted_at IS NULL
	`

	err = adminDB.Select(&containers, query, project.ProjectID)
	if err != nil {
		return containers, err
	}

	return containers, nil
}

func GetContainerByProjectAndName(adminDB *sqlx.DB, project types.Project, containerName string) (container types.ContainerClaim, err error) {
	query := `
		SELECT container_claim.* FROM container_claim
		WHERE container_claim.project_id = $1 AND container_claim.name = $2 AND deleted_at IS NULL
	`

	err = adminDB.Get(&container, query, project.ProjectID, containerName)
	if err != nil {
		return container, err
	}

	return container, nil
}

func SetContainerAsActivating(adminDB *sqlx.DB, container types.ContainerClaim) error {
	query := `
		UPDATE container_claim
		SET status = 'activating'
		WHERE container_claim_id = $1
	`

	_, err := adminDB.Exec(query, container.ContainerClaimID)
	if err != nil {
		return err
	}

	return nil
}

func SetContainerAsActive(adminDB *sqlx.DB, container types.ContainerClaim) error {
	query := `
		UPDATE container_claim
		SET status = 'active'
		WHERE container_claim_id = $1
	`

	_, err := adminDB.Exec(query, container.ContainerClaimID)
	if err != nil {
		return err
	}

	err = addToContainerResourceUsage(adminDB, container)
	if err != nil {
		return err
	}

	return nil
}

func SetContainerAsDeactivating(adminDB *sqlx.DB, container types.ContainerClaim) error {
	query := `
		UPDATE container_claim
		SET status = 'deactivating'
		WHERE container_claim_id = $1
	`

	_, err := adminDB.Exec(query, container.ContainerClaimID)
	if err != nil {
		return err
	}

	err = removeFromContainerResourceUsage(adminDB, container)
	if err != nil {
		return err
	}

	return nil
}

func SetContainerAsErrorState(adminDB *sqlx.DB, container types.ContainerClaim) error {
	query := `
		UPDATE container_claim
		SET status = 'error'
		WHERE container_claim_id = $1
	`

	_, err := adminDB.Exec(query, container.ContainerClaimID)
	if err != nil {
		return err
	}

	return nil
}

func DeleteContainerByProjectAndName(adminDB *sqlx.DB, project types.Project, containerName string) error {
	query := `
		UPDATE container_claim
		SET deleted_at = now(), status = 'inactive'
		WHERE project_id = $1 AND name = $2
	`

	_, err := adminDB.Exec(query, project.ProjectID, containerName)
	if err != nil {
		return err
	}

	return nil
}

func FindRandomFreePortAndSave(adminDB *sqlx.DB, project types.Project, containerClaim types.ContainerClaim, targetPort int64) (freePortAttempt int, err error) {
	KUBE_FREE_PORT_RANGE := numRange{10000, 60000}
	MAX_ATTEMPTS := 100

	tx, err := adminDB.Begin()
	if err != nil {
		return freePortAttempt, err
	}

	for range MAX_ATTEMPTS {
		// try to find a free port
		freePortAttempt = KUBE_FREE_PORT_RANGE.low + rand.Intn(KUBE_FREE_PORT_RANGE.hi-KUBE_FREE_PORT_RANGE.low)
		query := `
			SELECT COUNT(*) FROM container_claim
			WHERE $1 = ANY(ports) AND deleted_at IS NULL
		`
		var takenPortCount int
		err = tx.QueryRow(query, freePortAttempt).Scan(&takenPortCount)
		if err != nil {
			_ = tx.Rollback()
			return freePortAttempt, err
		}

		if takenPortCount != 0 {
			break
		}
	}
	if freePortAttempt == 0 {
		_ = tx.Rollback()
		return freePortAttempt, fmt.Errorf("Ran out of attempts to find a free port!")
	}

	// reserve this port by saving it
	savePortQuery := `
		UPDATE container_claim
		SET ports = array_replace(ports, $1, $2)
		WHERE project_id = $3 AND name = $4 AND deleted_at IS NULL
	`
	_, err = tx.Exec(savePortQuery, targetPort, freePortAttempt, project.ProjectID, containerClaim.Name)
	if err != nil {
		_ = tx.Rollback()
		return freePortAttempt, err
	}
	err = tx.Commit()
	if err != nil {
		return freePortAttempt, err
	}
	return freePortAttempt, nil
}

func SaveNodeIPOfRunningContainer(adminDB *sqlx.DB, project types.Project, containerClaim types.ContainerClaim, nodeIP string) error {
	query := `
		UPDATE container_claim
		SET node_ip = $1
		WHERE project_id = $2 AND name = $3
	`

	_, err := adminDB.Exec(query, nodeIP, project.ProjectID, containerClaim.Name)
	if err != nil {
		return err
	}

	return nil
}

func CreateUserDBClaimForProject(adminDB *sqlx.DB, project types.Project, userDBClaimInput types.UserDBClaim) (userDBClaimOutput types.UserDBClaim, err error) {
	createObjectStorageQuery := `
		WITH inserted_user_db_claim AS (
			INSERT INTO user_db_claim (project_id, zones)
			VALUES ($1, $2)
			RETURNING user_db_claim_id
		)
		SELECT user_db_claim_id FROM inserted_user_db_claim
	`

	var userDBClaimID int
	err = adminDB.QueryRow(createObjectStorageQuery, project.ProjectID, userDBClaimInput.Zones).Scan(&userDBClaimID)
	if err != nil {
		return userDBClaimOutput, err
	}

	userDBClaimOutput = userDBClaimInput
	userDBClaimOutput.UserDBClaimID = userDBClaimID

	return userDBClaimOutput, nil
}

func GetUserDBClaimsByProject(adminDB *sqlx.DB, project types.Project) (userDBClaims []types.UserDBClaim, err error) {
	query := `
		SELECT * FROM user_db_claim
		WHERE project_id = $1 AND deleted_at IS NULL
	`

	err = adminDB.Select(&userDBClaims, query, project.ProjectID)
	if err != nil {
		return userDBClaims, err
	}

	return userDBClaims, nil
}

func GetUserDBClaimByProject(adminDB *sqlx.DB, project types.Project) (userDBClaim types.UserDBClaim, err error) {
	query := `
		SELECT user_db_claim.* FROM user_db_claim
		WHERE user_db_claim.project_id = $1 AND deleted_at IS NULL
	`

	err = adminDB.Get(&userDBClaim, query, project.ProjectID)
	if err != nil {
		return userDBClaim, err
	}

	return userDBClaim, nil
}

func AddCredentialsToUserDBClaim(adminDB *sqlx.DB, userDBClaim types.UserDBClaim, newCreds types.UserDBClaimCredentials) error {
	query := `
		UPDATE user_db_claim
		SET credentials = $2
		WHERE user_db_claim_id = $1
	`
	newsCredsJSON, err := json.Marshal(&newCreds)
	if err != nil {
		return err
	}

	_, err = adminDB.Exec(query, userDBClaim.UserDBClaimID, newsCredsJSON)
	if err != nil {
		return err
	}

	return nil
}

func SetUserDBClaimAsActivating(adminDB *sqlx.DB, userDBClaim types.UserDBClaim) error {
	query := `
		UPDATE user_db_claim
		SET status = 'activating'
		WHERE user_db_claim_id = $1
	`

	_, err := adminDB.Exec(query, userDBClaim.UserDBClaimID)
	if err != nil {
		return err
	}

	return nil
}

func SetUserDBClaimAsActive(adminDB *sqlx.DB, userDBClaim types.UserDBClaim) error {
	query := `
		UPDATE user_db_claim
		SET status = 'active'
		WHERE user_db_claim_id = $1
	`

	_, err := adminDB.Exec(query, userDBClaim.UserDBClaimID)
	if err != nil {
		return err
	}

	return nil
}

func SetUserDBClaimAsDeactivating(adminDB *sqlx.DB, userDBClaim types.UserDBClaim) error {
	query := `
		UPDATE user_db_claim
		SET status = 'deactivating'
		WHERE user_db_claim_id = $1
	`

	_, err := adminDB.Exec(query, userDBClaim.UserDBClaimID)
	if err != nil {
		return err
	}

	return nil
}

func DeleteUserDBClaimByProject(adminDB *sqlx.DB, project types.Project) error {
	query := `
		UPDATE user_db_claim
		SET deleted_at = now(), status = 'inactive'
		WHERE project_id = $1
	`

	_, err := adminDB.Exec(query, project.ProjectID)
	if err != nil {
		return err
	}

	return nil
}

func CreateObjectStorageForProject(adminDB *sqlx.DB, project types.Project, objectStorageInput types.ObjectStorageClaim) (objectStorageOutput types.ObjectStorageClaim, err error) {
	createObjectStorageQuery := `
		WITH inserted_object_storage_claim AS (
			INSERT INTO object_storage_claim (project_id, name, zones)
			VALUES ($1, $2, $3)
			RETURNING object_storage_claim_id
		)
		SELECT object_storage_claim_id FROM inserted_object_storage_claim
	`

	var objectStorageClaimID int
	err = adminDB.QueryRow(createObjectStorageQuery, project.ProjectID, objectStorageInput.Name, objectStorageInput.Zones).Scan(&objectStorageClaimID)
	if err != nil {
		return objectStorageOutput, err
	}

	objectStorageOutput = objectStorageInput
	objectStorageOutput.ObjectStorageClaimID = objectStorageClaimID

	return objectStorageOutput, nil
}

func GetObjectStoragesByProject(adminDB *sqlx.DB, project types.Project) (objectStorages []types.ObjectStorageClaim, err error) {
	query := `
		SELECT * FROM object_storage_claim
		WHERE project_id = $1 AND deleted_at IS NULL
	`

	err = adminDB.Select(&objectStorages, query, project.ProjectID)
	if err != nil {
		return objectStorages, err
	}

	return objectStorages, nil
}

func GetObjectStorageByProjectAndName(adminDB *sqlx.DB, project types.Project, objectStorageName string) (objectStorage types.ObjectStorageClaim, err error) {
	query := `
		SELECT object_storage_claim.* FROM object_storage_claim
		WHERE object_storage_claim.project_id = $1 AND object_storage_claim.name = $2 AND deleted_at IS NULL
	`

	err = adminDB.Get(&objectStorage, query, project.ProjectID, objectStorageName)
	if err != nil {
		return objectStorage, err
	}

	return objectStorage, nil
}

func SetObjectStorageAsDeactivating(adminDB *sqlx.DB, objectStorage types.ObjectStorageClaim) error {
	query := `
		UPDATE object_storage_claim
		SET status = 'deactivating'
		WHERE object_storage_claim_id = $1
	`

	_, err := adminDB.Exec(query, objectStorage.ObjectStorageClaimID)
	if err != nil {
		return err
	}

	return nil
}

func DeleteObjectStorageByProjectAndName(adminDB *sqlx.DB, project types.Project, objectStorageName string) error {
	query := `
		UPDATE object_storage_claim
		SET deleted_at = now(), status = 'inactive'
		WHERE project_id = $1 AND name = $2
	`

	_, err := adminDB.Exec(query, project.ProjectID, objectStorageName)
	if err != nil {
		return err
	}

	return nil
}

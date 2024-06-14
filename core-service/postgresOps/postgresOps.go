package postgresOps

import (
	"fmt"
	"math/rand"
	"slices"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"
	"github.com/lu1a/lcaas/core-service/db"
	"github.com/lu1a/lcaas/core-service/types"
)

func CreateDatabaseForProject(log log.Logger, adminDB *sqlx.DB, userDBConnections []types.UserDB, project types.Project, userDBClaim types.UserDBClaim) error {
	err := db.SetUserDBClaimAsActivating(adminDB, userDBClaim)
	if err != nil {
		return err
	}

	for _, userDBConnection := range userDBConnections {
		if !slices.Contains(userDBClaim.Zones, userDBConnection.Zone) {
			continue
		}
		userDB, err := initDatabase(userDBConnection.DefaultParentEnvironmentURL())
		if err != nil {
			userDB.Close()
			return err
		}
		defer userDB.Close()

		unsanitaryCreateUserDBQuery := fmt.Sprintf(`CREATE DATABASE %s`, project.UserDBClaimName())
		_, err = userDB.Exec(unsanitaryCreateUserDBQuery)
		if err != nil {
			return err
		}

		newlyCreatedDB, err := initDatabase(userDBConnection.ConnWithSuffix(project.UserDBClaimName()))
		if err != nil {
			newlyCreatedDB.Close()
			return err
		}
		defer newlyCreatedDB.Close()

		generatedPassword := randSeq(10)

		unsanitaryCreateUserQuery := fmt.Sprintf(`CREATE USER %s WITH PASSWORD '%s'`, project.UserDBClaimRWUsername(), generatedPassword)
		_, err = userDB.Exec(unsanitaryCreateUserQuery)
		if err != nil {
			return err
		}
		unsanitaryGrantSchemaQuery := fmt.Sprintf(`ALTER DEFAULT PRIVILEGES IN SCHEMA PUBLIC GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s`, project.UserDBClaimRWUsername())
		_, err = newlyCreatedDB.Exec(unsanitaryGrantSchemaQuery)
		if err != nil {
			return err
		}
		unsanitaryGrantSchemaQuery2 := fmt.Sprintf(`GRANT CREATE, USAGE ON SCHEMA PUBLIC TO %s`, project.UserDBClaimRWUsername())
		_, err = newlyCreatedDB.Exec(unsanitaryGrantSchemaQuery2)
		if err != nil {
			return err
		}

		newCreds := types.Credentials{
			Username: project.UserDBClaimRWUsername(), Password: generatedPassword, AccessControlType: "rw",
		}
		err = db.AddCredentialsToUserDBClaim(adminDB, userDBClaim, types.UserDBClaimCredentials{
			Credentials: []types.Credentials{newCreds},
		})
		if err != nil {
			return err
		}
	}

	err = db.SetUserDBClaimAsActive(adminDB, userDBClaim)
	if err != nil {
		return err
	}
	return nil
}

func CreateNewUserForUserDB(log log.Logger, adminDB *sqlx.DB, userDBConnections []types.UserDB, project types.Project, userDBClaim types.UserDBClaim, newUsername string) error {
	for _, userDBConnection := range userDBConnections {
		if !slices.Contains(userDBClaim.Zones, userDBConnection.Zone) {
			continue
		}

		userDBConn, err := initDatabase(userDBConnection.ConnWithSuffix(project.UserDBClaimName()))
		if err != nil {
			userDBConn.Close()
			return err
		}
		defer userDBConn.Close()

		newUsersGeneratedPassword := randSeq(10)

		unsanitaryCreateUserQuery := fmt.Sprintf(`CREATE USER %s WITH PASSWORD '%s'`, newUsername, newUsersGeneratedPassword)
		_, err = userDBConn.Exec(unsanitaryCreateUserQuery)
		if err != nil {
			return err
		}
		unsanitaryGrantSchemaQuery := fmt.Sprintf(`ALTER DEFAULT PRIVILEGES IN SCHEMA PUBLIC GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s`, newUsername)
		_, err = userDBConn.Exec(unsanitaryGrantSchemaQuery)
		if err != nil {
			return err
		}
		unsanitaryGrantSchemaQuery2 := fmt.Sprintf(`GRANT CREATE, USAGE ON SCHEMA PUBLIC TO %s`, newUsername)
		_, err = userDBConn.Exec(unsanitaryGrantSchemaQuery2)
		if err != nil {
			return err
		}

		newCreds := append(userDBClaim.Credentials.Credentials, types.Credentials{
			Username: newUsername, Password: newUsersGeneratedPassword, AccessControlType: "rw",
		})
		err = db.AddCredentialsToUserDBClaim(adminDB, userDBClaim, types.UserDBClaimCredentials{
			Credentials: newCreds,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteDatabaseForProject(log log.Logger, adminDB *sqlx.DB, userDBConnections []types.UserDB, project types.Project, userDBClaim types.UserDBClaim) error {
	// set userDBClaim as deactivating
	err := db.SetUserDBClaimAsDeactivating(adminDB, userDBClaim)
	if err != nil {
		return err
	}

	for _, userDBConnection := range userDBConnections {
		if !slices.Contains(userDBClaim.Zones, userDBConnection.Zone) {
			continue
		}
		userDB, err := initDatabase(userDBConnection.ConnectionURL)
		if err != nil {
			userDB.Close()
			return err
		}
		defer userDB.Close()

		connInsideUserDB, err := initDatabase(userDBConnection.ConnWithSuffix(project.UserDBClaimName()))
		if err != nil {
			connInsideUserDB.Close()
			return err
		}
		defer connInsideUserDB.Close()

		unsanitaryDropSchemaQuery := `DROP SCHEMA IF EXISTS PUBLIC CASCADE`
		_, err = connInsideUserDB.Exec(unsanitaryDropSchemaQuery)
		if err != nil {
			return err
		}

		unsanitaryDropUserQuery := fmt.Sprintf(`DROP USER IF EXISTS %s`, project.UserDBClaimRWUsername())
		_, err = userDB.Exec(unsanitaryDropUserQuery)
		if err != nil {
			return err
		}

		unsanitaryDropUserDBQuery := fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, project.UserDBClaimName())
		_, err = userDB.Exec(unsanitaryDropUserDBQuery)
		if err != nil {
			return err
		}
	}

	// delete the userDBClaim in adminDB
	err = db.DeleteUserDBClaimByProject(adminDB, project)
	if err != nil {
		return err
	}
	return nil
}

func initDatabase(connURL string) (db *sqlx.DB, err error) {
	db, err = sqlx.Connect("postgres", connURL)
	if err != nil {
		return db, err
	}
	return db, nil
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789$_-!&")

func randSeq(n int) string {
	rand.Seed(time.Now().UnixNano())

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

package auth

import "github.com/lu1a/lcaas/core-service/types"

/*
Route: /api/auth
Type: query
*/
type IAPIAuthResponse struct {
	Account types.Account `json:"account"`
}

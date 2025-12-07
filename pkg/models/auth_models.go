package models

import (
	"github.com/golang-jwt/jwt/v5"
)

// OAuthUser represents a user shape embedded into JWT and context (non-DB)
// This is used to store user information in the JWT token
// @model OAuthUser
type OAuthUser struct {
	// @Description User ID
	// @example "bc198ec4-3f81-4729-ac5d-04b838d2ab3c"
	ID string `json:"id" example:"bc198ec4-3f81-4729-ac5d-04b838d2ab3c"`
	// @Description User email
	// @example "john.doe@example.com"
	Email string `json:"email" example:"john.doe@example.com"`
	// @Description User name
	// @example "John Doe"
	Name string `json:"name" example:"John Doe"`
	// @Description User provider
	// @example "msa"
	Provider string `json:"provider" example:"msa"`
}

// JWTClaims represents JWT token claims (non-DB)
// @model JWTClaims
type JWTClaims struct {
	// @Description User information
	// @example "OAuthUser"
	User OAuthUser `json:"user" example:"OAuthUser"`
	// @Description Scopes
	// @example ["read", "write"]
	Scopes []string `json:"scopes" example:"[\"read\", \"write\"]"`
	// @Description Registered claims
	// @example "RegisteredClaims"
	jwt.RegisteredClaims
}

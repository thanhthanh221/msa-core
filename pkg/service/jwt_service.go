package services

import (
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/thanhthanh221/msa-core/pkg/models"
)

type JWTService interface {
	GenerateToken(user models.OAuthUser, scopes []string, expiresIn time.Duration) (string, error)
	ValidateToken(tokenString string) (*models.JWTClaims, error)
	RefreshToken(tokenString string, expiresIn time.Duration) (string, error)
	ExtractUser(tokenString string) (*models.OAuthUser, error)
	BlacklistToken(tokenString string, expiry time.Duration) error
	IsTokenBlacklisted(tokenString string) (bool, error)
	GenerateRefreshToken(user models.OAuthUser) (string, error)
	ValidateRefreshToken(tokenString string) (string, error)
}

type jwtService struct {
	secretKey []byte
}

func NewJWTService(secretKey string) JWTService {
	return &jwtService{
		secretKey: []byte(secretKey),
	}
}

func (s *jwtService) GenerateToken(user models.OAuthUser, scopes []string, expiresIn time.Duration) (string, error) {
	claims := models.JWTClaims{
		User:   user,
		Scopes: scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "msa-backend",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *jwtService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(token *jwt.Token) (any, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*models.JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Check if token is expired
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("token expired")
	}

	return claims, nil
}

func (s *jwtService) RefreshToken(tokenString string, expiresIn time.Duration) (string, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// Generate new token with same user info but new expiry
	return s.GenerateToken(claims.User, claims.Scopes, expiresIn)
}

func (s *jwtService) ExtractUser(tokenString string) (*models.OAuthUser, error) {
	// Parse token without validation to extract claims
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &models.JWTClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*models.JWTClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return &claims.User, nil
}

func (s *jwtService) BlacklistToken(tokenString string, expiry time.Duration) error {
	if tokenString == "" {
		return errors.New("empty token")
	}
	expiresAt := time.Now().Add(expiry)
	blacklistedTokens.Store(tokenString, expiresAt)
	return nil
}

func (s *jwtService) IsTokenBlacklisted(tokenString string) (bool, error) {
	if tokenString == "" {
		return false, nil
	}
	if v, ok := blacklistedTokens.Load(tokenString); ok {
		if exp, ok2 := v.(time.Time); ok2 {
			if time.Now().Before(exp) {
				return true, nil
			}
			// expired - cleanup
			blacklistedTokens.Delete(tokenString)
		} else {
			// corrupted entry - cleanup
			blacklistedTokens.Delete(tokenString)
		}
	}
	return false, nil
}

// in-memory blacklist storage with TTL (token -> expiresAt)
var blacklistedTokens sync.Map

func (s *jwtService) GenerateRefreshToken(user models.OAuthUser) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"type":    "refresh",
		"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(), // 30 days
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

func (s *jwtService) ValidateRefreshToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return s.secretKey, nil
	})

	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid refresh token")
	}

	// Check token type
	if claims["type"] != "refresh" {
		return "", errors.New("invalid token type")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", errors.New("invalid user ID in refresh token")
	}

	return userID, nil
}

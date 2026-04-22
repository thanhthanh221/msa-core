package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/thanhthanh221/msa-core/pkg/infrastructure/redis"
	"github.com/thanhthanh221/msa-core/pkg/models"
)

// JWTService issues and validates JWT access/refresh tokens.
type JWTService interface {
	GenerateToken(user models.OAuthUser, scopes []string, issuer, sid string, expiresIn time.Duration) (string, error)
	GenerateRefreshToken(user models.OAuthUser, issuer, sid string, expiresIn time.Duration) (string, error)

	RefreshToken(tokenString string, expiresIn time.Duration) (string, error)

	ValidateToken(tokenString string) (*models.JWTClaims, error)
	ValidateRefreshToken(tokenString string) (string, error)
}

type jwtService struct {
	secretKey []byte
	redis     redis.RedisClient
}

const redisSessionKeyPrefix = "session:"

func sessionRedisKey(sid string) string { return redisSessionKeyPrefix + sid }

func NewJWTService(secretKey string, redisClient redis.RedisClient) JWTService {
	return &jwtService{
		secretKey: []byte(secretKey),
		redis:     redisClient,
	}
}

func (s *jwtService) GenerateToken(user models.OAuthUser, scopes []string, issuer, sid string, expiresIn time.Duration) (string, error) {
	claims := models.JWTClaims{
		User:   user,
		SID:    sid,
		Scopes: scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", err
	}
	return signed, nil
}

func (s *jwtService) GenerateRefreshToken(user models.OAuthUser, issuer, sid string, expiresIn time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": user.ID,
		"iss": issuer,
		"sid": sid,
		"typ": "refresh",
		"exp": time.Now().Add(expiresIn).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", err
	}
	return signed, nil
}

func (s *jwtService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			fmt.Println("unexpected signing method")
			return nil, errors.New("unexpected signing method")
		}
		return s.secretKey, nil
	})

	if err != nil {
		fmt.Println("error parsing token", err)
		return nil, err
	}

	claims, ok := token.Claims.(*models.JWTClaims)
	if !ok || !token.Valid {
		fmt.Println("invalid token claims")
		return nil, errors.New("invalid token")
	}

	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		fmt.Println("token expired")
		return nil, errors.New("token expired or invalid")
	}

	// Validate session existence by SID (logout invalidates immediately by deleting the session key).
	if claims.SID == "" {
		fmt.Println("missing session id")
		return nil, errors.New("missing session id")
	}
	if claims.Issuer == "" {
		fmt.Println("missing issuer")
		return nil, errors.New("missing issuer")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	exists, err := s.redis.Exists(ctx, sessionRedisKey(claims.SID))
	if err != nil {
		fmt.Println("error checking session existence", err)
		return nil, err
	}
	if !exists {
		fmt.Println("session expired")
		return nil, errors.New("session expired")
	}

	return claims, nil
}

func (s *jwtService) RefreshToken(tokenString string, expiresIn time.Duration) (string, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		fmt.Println("error refreshing token", err)
		return "", err
	}
	return s.GenerateToken(claims.User, claims.Scopes, claims.Issuer, claims.SID, expiresIn)
}

func (s *jwtService) ValidateRefreshToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return s.secretKey, nil
	})

	if err != nil {
		fmt.Println("error parsing refresh token", err)
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		fmt.Println("invalid refresh token")
		return "", err
	}

	if claims["typ"] != "refresh" {
		fmt.Println("invalid refresh token type")
		return "", err
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		fmt.Println("invalid refresh token claims")
		return "", err
	}

	return userID, nil
}

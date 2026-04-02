package services

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/thanhthanh221/msa-core/pkg/infrastructure/redis"
	"github.com/thanhthanh221/msa-core/pkg/models"
)

// JWTService issues and validates JWT access/refresh tokens.
type JWTService interface {
	GenerateToken(user models.OAuthUser, scopes []string, issuer string, expiresIn time.Duration) (string, error)
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
	redis     redis.RedisClient
}

const redisBlacklistKeyPrefix = "blacklist:token:"

func blacklistRedisKey(token string) string { return redisBlacklistKeyPrefix + token }

func NewJWTService(secretKey string, redisClient redis.RedisClient) JWTService {
	return &jwtService{
		secretKey: []byte(secretKey),
		redis:     redisClient,
	}
}

func (s *jwtService) GenerateToken(user models.OAuthUser, scopes []string, issuer string, expiresIn time.Duration) (string, error) {
	claims := models.JWTClaims{
		User:   user,
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

func (s *jwtService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(token *jwt.Token) (any, error) {
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
	return s.GenerateToken(claims.User, claims.Scopes, claims.Issuer, expiresIn)
}

func (s *jwtService) ExtractUser(tokenString string) (*models.OAuthUser, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &models.JWTClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*models.JWTClaims)
	if !ok {
		return nil, err
	}

	return &claims.User, nil
}

func (s *jwtService) BlacklistToken(tokenString string, expiry time.Duration) error {
	if tokenString == "" {
		return errors.New("empty token")
	}
	if s.redis == nil {
		return errors.New("redis client required for blacklist")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.redis.Set(ctx, blacklistRedisKey(tokenString), "1", expiry); err != nil {
		return err
	}
	return nil
}

func (s *jwtService) IsTokenBlacklisted(tokenString string) (bool, error) {
	if tokenString == "" {
		return false, nil
	}
	if s.redis == nil {
		return false, errors.New("redis client required for blacklist check")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	exists, err := s.redis.Exists(ctx, blacklistRedisKey(tokenString))
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *jwtService) GenerateRefreshToken(user models.OAuthUser) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"type":    "refresh",
		"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", err
	}
	return signed, nil
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
		return "", err
	}

	if claims["type"] != "refresh" {
		return "", err
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", err
	}

	return userID, nil
}

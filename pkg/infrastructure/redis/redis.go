package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type RedisClient interface {
	Set(ctx context.Context, key string, val any, exp time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	GetDel(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key string) error
	HMSet(ctx context.Context, key string, val any) error
	HMGet(ctx context.Context, key string, field string) (interface{}, error)
	HSet(ctx context.Context, key string, hKey any, val any) error
	HGet(ctx context.Context, key string, hkey string) (interface{}, error)
	HGetAll(ctx context.Context, key string) (map[string]interface{}, error)
	HDel(ctx context.Context, key string, hKey string) error
	Incr(ctx context.Context, key string) (int64, error)
	SetNX(ctx context.Context, key string, val any, exp time.Duration) (bool, error)
	Expire(ctx context.Context, key string, exp time.Duration) error
	ExpireNX(ctx context.Context, key string, exp time.Duration) (bool, error)
	HExists(ctx context.Context, key string, hkey string) (bool, error)
	HKeys(ctx context.Context, key string) ([]string, error)
	HValues(ctx context.Context, key string) ([]string, error)
	HLen(ctx context.Context, key string) (int64, error)
	HSetNX(ctx context.Context, key string, hKey string, val any) (bool, error)
	HIncrBy(ctx context.Context, key string, hKey string, incr int64) (int64, error)
	HIncrByFloat(ctx context.Context, key string, hKey string, incr float64) (float64, error)
	HSetStruct(ctx context.Context, key string, hKey string, data interface{}) error
	HGetStruct(ctx context.Context, key string, hKey string) (interface{}, error)
	HMSetStruct(ctx context.Context, key string, data map[string]interface{}) error
	HGetAllStruct(ctx context.Context, key string) (map[string]interface{}, error)
	GetAllKeyByPrefix(ctx context.Context, prefix string) ([]string, error)
	Exists(ctx context.Context, key string) (bool, error)
	Close() error
}

// redisClient implements RedisClient interface
type redisClient struct {
	cluster *redis.ClusterClient
	client  *redis.Client
	prefix  string
	tracer  trace.TracerProvider
}

// NewRedisClient creates a new Redis client instance with tracing support
func NewRedisClient(clusterEnv, address, password, prefix string, tracer trace.TracerProvider) RedisClient {
	rc := &redisClient{
		tracer: tracer,
	}

	// Set prefix
	if len(prefix) > 0 {
		rc.prefix = prefix + ":"
	}

	// Try cluster mode first
	if strings.TrimSpace(clusterEnv) != "" {
		addrs := []string{}
		for _, p := range strings.Split(clusterEnv, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				addrs = append(addrs, p)
			}
		}
		if len(addrs) > 0 {
			rc.cluster = redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:    addrs,
				Password: password,
			})
			return rc
		}
	}

	// Fallback to single instance
	if strings.TrimSpace(address) != "" {
		rc.client = redis.NewClient(&redis.Options{
			Addr:     address,
			Password: password,
		})
	}

	return rc
}

// trace creates a new span for Redis operations
func (r *redisClient) trace(ctx context.Context, operation string) (context.Context, trace.Span) {
	tracer := r.tracer.Tracer("redis.client")
	return tracer.Start(ctx, fmt.Sprintf("redis.%s", operation))
}

// getClient returns the appropriate client (cluster or single instance)
func (r *redisClient) getClient() redis.Cmdable {
	if r.cluster != nil {
		return r.cluster
	}
	return r.client
}

func (r *redisClient) Set(ctx context.Context, key string, val any, exp time.Duration) error {
	ctx, span := r.trace(ctx, "set")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "set"),
		attribute.Float64("redis.expiration_seconds", exp.Seconds()),
	)

	err := r.getClient().Set(ctx, fullKey, val, exp).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

func (r *redisClient) Get(ctx context.Context, key string) (string, error) {
	ctx, span := r.trace(ctx, "get")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "get"),
	)

	result, err := r.getClient().Get(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			span.SetStatus(codes.Ok, "key not found")
			return "", err
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetAttributes(attribute.String("redis.value_length", fmt.Sprintf("%d", len(result))))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) GetDel(ctx context.Context, key string) (string, error) {
	ctx, span := r.trace(ctx, "getdel")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "getdel"),
	)

	var val string
	var err error

	if r.cluster != nil {
		val, err = r.cluster.Get(ctx, fullKey).Result()
		if err == nil {
			err = r.cluster.Del(ctx, fullKey).Err()
		}
	} else {
		val, err = r.client.GetDel(ctx, fullKey).Result()
	}

	if err != nil {
		if err == redis.Nil {
			span.SetStatus(codes.Ok, "key not found")
			return "", err
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetStatus(codes.Ok, "success")
	return val, nil
}

func (r *redisClient) Del(ctx context.Context, key string) error {
	ctx, span := r.trace(ctx, "del")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "del"),
	)

	err := r.getClient().Del(ctx, fullKey).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

func (r *redisClient) HMSet(ctx context.Context, key string, val any) error {
	ctx, span := r.trace(ctx, "hmset")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "hmset"),
	)

	err := r.getClient().HMSet(ctx, fullKey, val).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

func (r *redisClient) HMGet(ctx context.Context, key string, field string) (interface{}, error) {
	ctx, span := r.trace(ctx, "hmget")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.field", field),
		attribute.String("redis.operation", "hmget"),
	)

	values, err := r.getClient().HMGet(ctx, fullKey, field).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if values[0] == nil {
		span.SetStatus(codes.Ok, "field not found")
		return nil, redis.Nil
	}

	span.SetStatus(codes.Ok, "success")
	return values[0], nil
}

func (r *redisClient) HSet(ctx context.Context, key string, hKey any, val any) error {
	ctx, span := r.trace(ctx, "hset")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.hash_key", fmt.Sprintf("%v", hKey)),
		attribute.String("redis.operation", "hset"),
	)

	err := r.getClient().HSet(ctx, fullKey, hKey, val).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

func (r *redisClient) HGet(ctx context.Context, key string, hkey string) (interface{}, error) {
	ctx, span := r.trace(ctx, "hget")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.hash_key", hkey),
		attribute.String("redis.operation", "hget"),
	)

	value := r.getClient().HGet(ctx, fullKey, hkey).Val()
	if value == "" {
		span.SetStatus(codes.Ok, "field not found")
		return nil, redis.Nil
	}

	span.SetStatus(codes.Ok, "success")
	return value, nil
}

func (r *redisClient) HGetAll(ctx context.Context, key string) (map[string]interface{}, error) {
	ctx, span := r.trace(ctx, "hgetall")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "hgetall"),
	)

	value := r.getClient().HGetAll(ctx, fullKey).Val()
	m := make(map[string]interface{})
	for k, v := range value {
		m[k] = v
	}

	span.SetAttributes(attribute.Int("redis.hash_size", len(m)))
	span.SetStatus(codes.Ok, "success")
	return m, nil
}

func (r *redisClient) HDel(ctx context.Context, key string, hKey string) error {
	ctx, span := r.trace(ctx, "hdel")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.hash_key", hKey),
		attribute.String("redis.operation", "hdel"),
	)

	err := r.getClient().HDel(ctx, fullKey, hKey).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

func (r *redisClient) Incr(ctx context.Context, key string) (int64, error) {
	ctx, span := r.trace(ctx, "incr")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "incr"),
	)

	result, err := r.getClient().Incr(ctx, fullKey).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	span.SetAttributes(attribute.Int64("redis.value", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) SetNX(ctx context.Context, key string, val any, exp time.Duration) (bool, error) {
	ctx, span := r.trace(ctx, "setnx")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "setnx"),
		attribute.Float64("redis.expiration_seconds", exp.Seconds()),
	)

	result, err := r.getClient().SetNX(ctx, fullKey, val, exp).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	span.SetAttributes(attribute.Bool("redis.set", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) Expire(ctx context.Context, key string, exp time.Duration) error {
	ctx, span := r.trace(ctx, "expire")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "expire"),
		attribute.Float64("redis.expiration_seconds", exp.Seconds()),
	)

	err := r.getClient().Expire(ctx, fullKey, exp).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

func (r *redisClient) ExpireNX(ctx context.Context, key string, exp time.Duration) (bool, error) {
	ctx, span := r.trace(ctx, "expirenx")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "expirenx"),
		attribute.Float64("redis.expiration_seconds", exp.Seconds()),
	)

	result, err := r.getClient().Expire(ctx, fullKey, exp).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	span.SetAttributes(attribute.Bool("redis.expired", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) HExists(ctx context.Context, key string, hkey string) (bool, error) {
	ctx, span := r.trace(ctx, "hexists")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.hash_key", hkey),
		attribute.String("redis.operation", "hexists"),
	)

	result, err := r.getClient().HExists(ctx, fullKey, hkey).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	span.SetAttributes(attribute.Bool("redis.exists", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) HKeys(ctx context.Context, key string) ([]string, error) {
	ctx, span := r.trace(ctx, "hkeys")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "hkeys"),
	)

	result, err := r.getClient().HKeys(ctx, fullKey).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.Int("redis.keys_count", len(result)))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) HValues(ctx context.Context, key string) ([]string, error) {
	ctx, span := r.trace(ctx, "hvalues")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "hvalues"),
	)

	result, err := r.getClient().HVals(ctx, fullKey).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.Int("redis.values_count", len(result)))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) HLen(ctx context.Context, key string) (int64, error) {
	ctx, span := r.trace(ctx, "hlen")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "hlen"),
	)

	result, err := r.getClient().HLen(ctx, fullKey).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	span.SetAttributes(attribute.Int64("redis.length", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) HSetNX(ctx context.Context, key string, hKey string, val any) (bool, error) {
	ctx, span := r.trace(ctx, "hsetnx")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.hash_key", hKey),
		attribute.String("redis.operation", "hsetnx"),
	)

	result, err := r.getClient().HSetNX(ctx, fullKey, hKey, val).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	span.SetAttributes(attribute.Bool("redis.set", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) HIncrBy(ctx context.Context, key string, hKey string, incr int64) (int64, error) {
	ctx, span := r.trace(ctx, "hincrby")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.hash_key", hKey),
		attribute.Int64("redis.increment", incr),
		attribute.String("redis.operation", "hincrby"),
	)

	result, err := r.getClient().HIncrBy(ctx, fullKey, hKey, incr).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	span.SetAttributes(attribute.Int64("redis.value", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) HIncrByFloat(ctx context.Context, key string, hKey string, incr float64) (float64, error) {
	ctx, span := r.trace(ctx, "hincrbyfloat")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.hash_key", hKey),
		attribute.Float64("redis.increment", incr),
		attribute.String("redis.operation", "hincrbyfloat"),
	)

	result, err := r.getClient().HIncrByFloat(ctx, fullKey, hKey, incr).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	span.SetAttributes(attribute.Float64("redis.value", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

// Helper function để lưu struct vào HSET
func (r *redisClient) HSetStruct(ctx context.Context, key string, hKey string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return r.HSet(ctx, key, hKey, string(jsonData))
}

// Helper function để lấy struct từ HSET
func (r *redisClient) HGetStruct(ctx context.Context, key string, hKey string) (interface{}, error) {
	return r.HGet(ctx, key, hKey)
}

// Helper function để lưu nhiều struct vào HSET
func (r *redisClient) HMSetStruct(ctx context.Context, key string, data map[string]interface{}) error {
	jsonData := make(map[string]interface{})
	for k, v := range data {
		if jsonBytes, err := json.Marshal(v); err == nil {
			jsonData[k] = string(jsonBytes)
		} else {
			jsonData[k] = v
		}
	}
	return r.HMSet(ctx, key, jsonData)
}

// Helper function để lấy tất cả struct từ HSET
func (r *redisClient) HGetAllStruct(ctx context.Context, key string) (map[string]interface{}, error) {
	return r.HGetAll(ctx, key)
}

func (r *redisClient) GetAllKeyByPrefix(ctx context.Context, prefix string) ([]string, error) {
	ctx, span := r.trace(ctx, "getallkeybyprefix")
	defer span.End()

	match := fmt.Sprintf("%s:*", prefix)
	span.SetAttributes(
		attribute.String("redis.prefix", prefix),
		attribute.String("redis.pattern", match),
		attribute.String("redis.operation", "getallkeybyprefix"),
	)

	var keys []string
	var mu sync.Mutex

	if r.cluster != nil {
		err := r.cluster.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			iter := client.Scan(ctx, 0, match, 0).Iterator()
			for iter.Next(ctx) {
				mu.Lock()
				keys = append(keys, iter.Val())
				mu.Unlock()
			}
			if err := iter.Err(); err != nil {
				return fmt.Errorf("error scanning keys on a master node: %w", err)
			}
			return nil
		})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
	} else {
		if r.client == nil {
			err := fmt.Errorf("redis is not initialized")
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		iter := r.client.Scan(ctx, 0, match, 0).Iterator()
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		if err := iter.Err(); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("error scanning keys: %w", err)
		}
	}

	span.SetAttributes(attribute.Int("redis.keys_count", len(keys)))
	span.SetStatus(codes.Ok, "success")
	return keys, nil
}

func (r *redisClient) Exists(ctx context.Context, key string) (bool, error) {
	ctx, span := r.trace(ctx, "exists")
	defer span.End()

	fullKey := r.prefix + key
	span.SetAttributes(
		attribute.String("redis.key", fullKey),
		attribute.String("redis.operation", "exists"),
	)

	exists, err := r.getClient().Exists(ctx, fullKey).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	result := exists > 0
	span.SetAttributes(attribute.Bool("redis.exists", result))
	span.SetStatus(codes.Ok, "success")
	return result, nil
}

func (r *redisClient) Close() error {
	if r.cluster != nil {
		return r.cluster.Close()
	}
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Helper functions for type-safe operations (updated to use context)
func HMGetTyped[E any](rc RedisClient, ctx context.Context, key string, field string) (*E, error) {
	value, err := rc.HMGet(ctx, key, field)
	if err != nil {
		return nil, err
	}
	var e E
	err = json.Unmarshal([]byte(value.(string)), &e)
	return &e, err
}

func HGetTyped[E any](rc RedisClient, ctx context.Context, key string, hkey string) (*E, error) {
	value, err := rc.HGet(ctx, key, hkey)
	if err != nil {
		return nil, err
	}
	var e E
	err = json.Unmarshal([]byte(value.(string)), &e)
	return &e, err
}

func HGetAllTyped[E any](rc RedisClient, ctx context.Context, key string) (map[string]E, error) {
	allData, err := rc.HGetAll(ctx, key)
	if err != nil {
		return nil, err
	}
	result := make(map[string]E)
	for k, v := range allData {
		var item E
		if err := json.Unmarshal([]byte(v.(string)), &item); err == nil {
			result[k] = item
		}
	}
	return result, nil
}

func HGetStructTyped[T any](rc RedisClient, ctx context.Context, key string, hKey string) (*T, error) {
	jsonStr, err := rc.HGetStruct(ctx, key, hKey)
	if err != nil {
		return nil, err
	}
	var result T
	err = json.Unmarshal([]byte(jsonStr.(string)), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func HGetAllStructTyped[T any](rc RedisClient, ctx context.Context, key string) (map[string]T, error) {
	return HGetAllTyped[T](rc, ctx, key)
}

package redisconn

import (
	"context"
	"log/slog"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var (
	sessionKeyPrefix = "session:"
	authTokenPrefix  = "authToken:"
)

type RedisConn struct {
	client *redis.Client
}

func NewRedisConn(ctx context.Context, addr string) (*RedisConn, error) {
	r := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})

	err := r.Ping(ctx).Err()
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "redis connection is successful")

	return &RedisConn{
		client: r,
	}, nil
}

func (r *RedisConn) CreateAuthToken(ctx context.Context) (string, error) {
	authToken, err := generateCryptoToken(ctx)
	if err != nil {
		return "", err
	}

	key := authTokenPrefix + authToken

	expiry := 60 * time.Minute

	err = r.client.Set(ctx, key, 0, expiry).Err()
	if err != nil {
		return "", err
	}

	return authToken, nil
}

// verifyAndDelToken is used to delete a token after it has beeen verified
// as authentic.
func (r *RedisConn) VerifyAndDelToken(ctx context.Context, token string) error {
	key := authTokenPrefix + token

	err := r.client.Get(ctx, key).Err()
	if err != nil {
		return err
	}

	// Delete the auth token immediately after verification
	// since it is no longer needed
	err = r.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	return nil
}

func (r *RedisConn) CreateSessionToken(
	ctx context.Context,
	email string,
) (string, error) {
	token, err := generateCryptoToken(ctx)
	if err != nil {
		return "", err
	}

	key := sessionKeyPrefix + token

	expiry := 24 * time.Hour

	err = r.client.Set(ctx, key, email, expiry).Err()
	if err != nil {
		return "", err
	}

	return token, err
}

func (r *RedisConn) GetSessionToken(
	ctx context.Context,
	token string,
) (string, error) {
	key := sessionKeyPrefix + token

	return r.client.Get(ctx, key).Result()
}

func (r *RedisConn) DeleteSessionToken(
	ctx context.Context,
	token string,
) error {
	sessToken := sessionKeyPrefix + token

	return r.client.Del(ctx, sessToken).Err()
}

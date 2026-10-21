package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"lab1/internal/app/config"
)

// Ключи Redis: префикс сервиса, чтобы не пересекаться с чужими данными.
const (
	servicePrefix      = "interplanet."
	jwtBlacklistPrefix = "jwt.blacklist."
)

type Client struct {
	client *goredis.Client
}

func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	c := goredis.NewClient(&goredis.Options{
		Addr:        cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Username:    cfg.User,
		Password:    cfg.Password,
		DB:          0,
		DialTimeout: cfg.DialTimeout,
		ReadTimeout: cfg.ReadTimeout,
	})

	if _, err := c.Ping(ctx).Result(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("не удалось подключиться к redis: %w", err)
	}
	return &Client{client: c}, nil
}

func (c *Client) Close() error { return c.client.Close() }

func blacklistKey(jti string) string {
	return servicePrefix + jwtBlacklistPrefix + jti
}

// RevokeJWT добавляет токен (по его jti) в blacklist.
// В значение пишем id пользователя, чтобы в redis-cli было видно, чей это токен.
// ttl = сколько токену осталось жить: после этого запись исчезнет сама,
// потому что просроченный токен и так не пройдет проверку.
func (c *Client) RevokeJWT(ctx context.Context, jti string, userID int, ttl time.Duration) error {
	return c.client.Set(ctx, blacklistKey(jti), userID, ttl).Err()
}

// IsJWTRevoked проверяет, есть ли токен в blacklist.
func (c *Client) IsJWTRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := c.client.Exists(ctx, blacklistKey(jti)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

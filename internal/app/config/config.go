package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	ServiceHost string
	ServicePort int

	JWT   JWTConfig
	Redis RedisConfig
}

// JWTConfig - настройки подписи и срока жизни токена.
type JWTConfig struct {
	Secret    string        // ключ подписи HS256
	ExpiresIn time.Duration // срок жизни токена, например "1h"
}

// RedisConfig - подключение к Redis (там лежит blacklist токенов после logout).
type RedisConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

func NewConfig() (*Config, error) {
	var err error

	configName := "config"
	_ = godotenv.Load()
	if os.Getenv("CONFIG_NAME") != "" {
		configName = os.Getenv("CONFIG_NAME")
	}

	viper.SetConfigName(configName)
	viper.SetConfigType("toml")
	viper.AddConfigPath("config")
	viper.AddConfigPath(".")
	viper.WatchConfig()

	err = viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	cfg := &Config{}           // создаем объект конфига
	err = viper.Unmarshal(cfg) // читаем информацию из файла,
	// конвертируем и затем кладем в нашу переменную cfg
	if err != nil {
		return nil, err
	}

	// Секреты можно не хранить в репозитории: переменные окружения (или .env) важнее config.toml.
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("не задан JWT.Secret (config.toml или переменная JWT_SECRET)")
	}
	if cfg.JWT.ExpiresIn <= 0 {
		cfg.JWT.ExpiresIn = time.Hour
	}
	if cfg.Redis.Host == "" || cfg.Redis.Port == 0 {
		return nil, fmt.Errorf("не заданы Redis.Host и Redis.Port в config.toml")
	}

	log.Info("config parsed")

	return cfg, nil
}

package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"localhost:8082"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
	User        string        `yaml:"user" env-required:"true"`
	Password    string        `yaml:"password" env-required:"true" env:"HTTP_SERVER_PASSWORD"`
}

type Config struct {
	Env         string `yaml:"env" env-default:"local"`
	StoragePath string `yaml:"storage_path" env-required:"true"`
	HTTPServer  `yaml:"http_server"`
}

func MustLoad(log *slog.Logger) *Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Error("CONFIG_PATH is required")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Error("config file not found", slog.String("path", configPath))
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Error("cannot read config", slog.Any("error", err))
	}

	return &cfg
}

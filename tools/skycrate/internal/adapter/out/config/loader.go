package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func Load(configPath string) (Config, error) {
	path, err := resolvePath(configPath)
	if err != nil {
		return Config{}, err
	}

	loader := viper.New()
	loader.SetConfigFile(path)
	loader.SetConfigType("yaml")
	if err := loader.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var raw fileConfig
	if err := loader.Unmarshal(&raw); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	validated, err := validate(raw)
	if err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}

	return validated, nil
}

func validate(raw fileConfig) (Config, error) {
	if strings.TrimSpace(raw.Bucket) == "" {
		return Config{}, errors.New("bucket must not be empty")
	}
	if strings.TrimSpace(raw.Bucket) != raw.Bucket {
		return Config{}, errors.New("bucket must not contain surrounding whitespace")
	}
	if len(raw.Categories) == 0 {
		return Config{}, errors.New("categories must not be empty")
	}

	categories := make(map[string]string, len(raw.Categories))
	for category, mapping := range raw.Categories {
		categories[category] = mapping.Tier
	}

	return Config{Bucket: raw.Bucket, Categories: categories}, nil
}

package config

import "github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"

type Config struct {
	Bucket     string
	Region     string
	Categories map[string]storage.Tier
}

type fileConfig struct {
	Bucket     string                    `mapstructure:"bucket"`
	Region     string                    `mapstructure:"region"`
	Categories map[string]categoryConfig `mapstructure:"categories"`
}

type categoryConfig struct {
	Tier string `mapstructure:"tier"`
}

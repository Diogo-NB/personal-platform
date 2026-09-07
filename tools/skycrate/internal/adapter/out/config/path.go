package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func DefaultPath() (string, error) {
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}

	return filepath.Join(configDirectory, "skycrate", "config.yaml"), nil
}

func resolvePath(configPath string) (string, error) {
	if configPath != "" {
		if strings.TrimSpace(configPath) != configPath {
			return "", errors.New("config path must not contain surrounding whitespace")
		}
		return configPath, nil
	}

	return DefaultPath()
}

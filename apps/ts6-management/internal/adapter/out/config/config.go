package config

import (
	"errors"
	"os"
)

type Config struct {
	ClusterARN  string
	ServiceName string
}

func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	configuration := Config{
		ClusterARN:  getenv("CLUSTER_ARN"),
		ServiceName: getenv("SERVICE_NAME"),
	}

	if configuration.ClusterARN == "" {
		return Config{}, errors.New("config: cluster arn is required")
	}
	if configuration.ServiceName == "" {
		return Config{}, errors.New("config: service name is required")
	}

	return configuration, nil
}

package config

type Config struct {
	Bucket     string
	Categories map[string]string
}

type fileConfig struct {
	Bucket     string                    `mapstructure:"bucket"`
	Categories map[string]categoryConfig `mapstructure:"categories"`
}

type categoryConfig struct {
	Tier string `mapstructure:"tier"`
}

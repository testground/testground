package api

// EnvConfig represents an environment configuration read from an .env.toml
// file.
type EnvConfig struct {
	AWS             AWSConfig            `toml:"aws"`
	BuildStrategies map[string]ConfigMap `toml:"build_strategies"`
	RunStrategies   map[string]ConfigMap `toml:"run_strategies"`
}

type AWSConfig struct {
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Region          string `toml:"region"`
}

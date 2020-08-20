package config

type ConfigMap map[string]interface{}

// EnvConfig contains the environment configuration. It is populated by
// coalescing values from these sources, in descending order of precedence:
//
//  1. environment variables.
//  2. env.toml.
//  3. default fallbacks.
type EnvConfig struct {
	dirs Directories

	AWS       AWSConfig            `toml:"aws"`
	DockerHub DockerHubConfig      `toml:"dockerhub"`
	Builders  map[string]ConfigMap `toml:"builders"`
	Runners   map[string]ConfigMap `toml:"runners"`
	Daemon    DaemonConfig         `toml:"daemon"`
	Client    ClientConfig         `toml:"client"`
}

func (e EnvConfig) Dirs() Directories {
	return e.dirs
}

type AWSConfig struct {
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Region          string `toml:"region"`
}

type DockerHubConfig struct {
	Repo        string `toml:"repo"`
	Username    string `toml:"username"`
	AccessToken string `toml:"access_token"`
}

type DaemonConfig struct {
	Listen    string   `toml:"listen"`
	Tokens    []string `toml:"tokens"`
	Workers   int      `toml:"workers"`
	QueueSize int      `toml:"queue_size"`
}

type ClientConfig struct {
	Endpoint string `toml:"endpoint"`
	Token    string `toml:"token"`
}

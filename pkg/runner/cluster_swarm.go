package runner

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/docker/docker/api/types/filters"

	"github.com/ipfs/testground/pkg/aws"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/util"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"

	"github.com/imdario/mergo"
)

var (
	_ api.Runner = &ClusterSwarmRunner{}
)

// ClusterSwarmRunnerConfig is the configuration object of this runner. Boolean
// values are expressed in a way that zero value (false) is the default setting.
type ClusterSwarmRunnerConfig struct {
	// LogLevel sets the log level in the test containers (default: not set).
	LogLevel string `toml:"log_level"`

	// DockerEndpoint is the URL of the docker swarm manager endpoint, e.g.
	// "tcp://manager:2376"
	DockerEndpoint string `toml:"docker_endpoint"`

	// DockerTLS indicates whether client TLS is enabled.
	DockerTLS bool `toml:"docker_tls"`

	// DockerTLSCACertPath is the path to the CA Certificate. Only used if
	// DockerTLS = true.
	DockerTLSCACertPath string `toml:"docker_tls_ca_cert_path"`

	// DockerTLSCertPath is the path to our client cert, signed by the CA. Only
	// used if DockerTLS = true.
	DockerTLSCertPath string `toml:"docker_tls_cert_path"`

	// DockerTLSKeyPath is our private key. Only used if DockerTLS = true.
	DockerTLSKeyPath string `toml:"docker_tls_key_path"`
}

// defaultClusterSwarmConfig is the default configuration. Incoming configurations will be
// merged with this object.
var defaultClusterSwarmConfig = ClusterSwarmRunnerConfig{}

// ClusterSwarmRunner is a runner that creates a Docker service to launch as
// many replicated instances of a container as the run job indicates.
type ClusterSwarmRunner struct{}

// TODO runner option to keep containers alive instead of deleting them after
// the test has run.
func (*ClusterSwarmRunner) Run(input *api.RunInput) (*api.RunOutput, error) {
	var (
		image = input.ArtifactPath
		seq   = input.Seq
		log   = logging.S().With("runner", "cluster:swarm", "run_id", input.RunID)

		// global timeout of 1 minute.
		ctx, cancelFn = context.WithTimeout(context.Background(), 1*time.Minute)
	)

	defer cancelFn()

	var (
		cfg   = defaultClusterSwarmConfig
		incfg = input.RunnerConfig.(*ClusterSwarmRunnerConfig)
	)

	// Merge the incoming configuration with the default configuration.
	if err := mergo.Merge(&cfg, incfg, mergo.WithOverride); err != nil {
		return nil, fmt.Errorf("error while merging configurations: %w", err)
	}

	// Sanity check.
	if seq < 0 || seq >= len(input.TestPlan.TestCases) {
		return nil, fmt.Errorf("invalid test case seq %d for plan %s", seq, input.TestPlan.Name)
	}

	// Get the test case.
	testcase := input.TestPlan.TestCases[seq]

	// Build a runenv.
	runenv := &runtime.RunEnv{
		TestPlan:           input.TestPlan.Name,
		TestCase:           testcase.Name,
		TestRun:            input.RunID,
		TestCaseSeq:        seq,
		TestInstanceCount:  input.Instances,
		TestInstanceParams: input.Parameters,
	}

	// Serialize the runenv into env variables to pass to docker.
	env := util.ToOptionsSlice(runenv.ToEnvVars())

	// Set the log level if provided in cfg.
	if cfg.LogLevel != "" {
		env = append(env, "LOG_LEVEL="+cfg.LogLevel)
	}

	// Create a docker client.
	var opts []client.Opt
	if cfg.DockerTLS {
		opts = append(opts, client.WithTLSClientConfig(cfg.DockerTLSCACertPath, cfg.DockerTLSCertPath, cfg.DockerTLSKeyPath))
	}

	opts = append(opts, client.WithHost(cfg.DockerEndpoint), client.WithAPIVersionNegotiation())
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}

	var (
		sname    = fmt.Sprintf("tg-%s-%s-%s", input.TestPlan.Name, testcase.Name, input.RunID)
		replicas = uint64(input.Instances)
	)

	// first check if redis is running.
	svcs, err := cli.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("name", "testground-redis")),
	})

	if len(svcs) == 0 {
		return nil, fmt.Errorf("testground-redis service doesn't exist in the swarm cluster; aborting")
	}

	log.Infow("creating service", "name", sname, "image", image, "replicas", replicas)

	spec := swarm.ServiceSpec{
		Networks: []swarm.NetworkAttachmentConfig{
			{Target: "data"},
			{Target: "control"},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
		Annotations: swarm.Annotations{Name: sname},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image: image,
				Env:   env,
			},
			RestartPolicy: &swarm.RestartPolicy{
				Condition: swarm.RestartPolicyConditionNone,
			},
			Resources: &swarm.ResourceRequirements{
				Reservations: &swarm.Resources{
					MemoryBytes: 60 * 1024 * 1024,
				},
				Limits: &swarm.Resources{
					MemoryBytes: 30 * 1024 * 1024,
				},
			},
			Placement: &swarm.Placement{
				MaxReplicas: 200,
			},
		},
	}

	logging.S().Infof("fetching an authorization token from AWS ECR")

	// Get an authorization token from ECR
	auth, err := aws.ECR.GetAuthToken(input.EnvConfig.AWS)
	if err != nil {
		return nil, err
	}

	logging.S().Infof("fetched an authorization token from AWS ECR")

	scopts := types.ServiceCreateOptions{
		QueryRegistry:       true,
		EncodedRegistryAuth: aws.ECR.EncodeAuthToken(auth),
	}

	logging.S().Infow("creating the service on docker swarm", "name", sname, "image", image, "replicas", replicas)

	resp, err := cli.ServiceCreate(ctx, spec, scopts)
	if err != nil {
		return nil, err
	}

	logging.S().Infow("service created successfully", "id", resp.ID)

	return &api.RunOutput{RunnerID: resp.ID}, nil
}

func (*ClusterSwarmRunner) ID() string {
	return "cluster:swarm"
}

func (*ClusterSwarmRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(ClusterSwarmRunnerConfig{})
}

func (*ClusterSwarmRunner) CompatibleBuilders() []string {
	return []string{"docker:go"}
}

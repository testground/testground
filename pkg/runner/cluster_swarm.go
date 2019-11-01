package runner

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/util"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"

	"github.com/imdario/mergo"
)

var (
	_ Runner = &ClusterSwarmRunner{}
)

// ClusterSwarmRunnerConfig is the configuration object of this runner. Boolean
// values are expressed in a way that zero value (false) is the default setting.
type ClusterSwarmRunnerConfig struct {
	// LogLevel sets the log level in the test containers (default: not set).
	LogLevel string `toml:"log_level"`
}

// defaultClusterSwarmConfig is the default configuration. Incoming configurations will be
// merged with this object.
var defaultClusterSwarmConfig = ClusterSwarmRunnerConfig{
}

// ClusterSwarmRunner is a runner that creates a Docker service to launch as
// many replicated instances of a container as the run job indicates.
type ClusterSwarmRunner struct{}

// TODO runner option to keep containers alive instead of deleting them after
// the test has run.
func (*ClusterSwarmRunner) Run(input *Input) (*Output, error) {
	var (
		// image    = input.ArtifactPath
		seq      = input.Seq
		deferred []func() error
		log      = logging.S().With("runner", "cluster:swarm", "run_id", input.ID)
	)

	defer func() {
		for i := len(deferred) - 1; i >= 0; i-- {
			if err := deferred[i](); err != nil {
				log.Errorw("error while calling deferred functions", "error", err)
			}
		}
	}()

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
		TestRun:            input.ID,
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

  // Temp Redis fix
	env = append(env, "REDIS_HOST=172.31.14.166")

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	// Start service
	// name := fmt.Sprintf("tg-%s-%s-%s", input.TestPlan.Name, testcase.Name, input.ID)
	name := fmt.Sprintf("tg-%s-%s", input.TestPlan.Name, testcase.Name)

	log.Infow("creating service", "name", name)

  // docker service create --replicas 1 --name helloworld alpine ping docker.com

  replicas := uint64(input.Instances)
  spec := swarm.ServiceSpec{
    Annotations: swarm.Annotations{
      Name: name,
    },
    TaskTemplate: swarm.TaskSpec{
      ContainerSpec: &swarm.ContainerSpec{
        Image: "909427826938.dkr.ecr.us-west-2.amazonaws.com/testground:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855@sha256:e175f10c2fc0545ede1de08458dffbea5b3efb3c023963028eac9129f4fd5920",
        Env: env,
      },
      RestartPolicy: &swarm.RestartPolicy{
        Condition: swarm.RestartPolicyConditionNone,
      },
      Networks: []swarm.NetworkAttachmentConfig{
        {
          Target: "hw6dcms11qhf3iv3rr2j8a2vb",
        },
      },
    },
    Mode: swarm.ServiceMode{
      Replicated: &swarm.ReplicatedService{
        Replicas: &replicas,
      },
    },
  }

  scopts := types.ServiceCreateOptions{
    // QueryRegistry: true
  }

  ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
  _, err = cli.ServiceCreate(ctx, spec, scopts)
  cancel()

  if err != nil {
    panic(err)
  }

	return nil, nil
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

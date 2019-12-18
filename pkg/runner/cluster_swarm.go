package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/docker/docker/api/types/network"
	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/aws"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/pkg/util"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

var (
	_ api.Runner = &ClusterSwarmRunner{}
)

// ClusterSwarmRunnerConfig is the configuration object of this runner. Boolean
// values are expressed in a way that zero value (false) is the default setting.
type ClusterSwarmRunnerConfig struct {
	// LogLevel sets the log level in the test containers (default: not set).
	LogLevel string `toml:"log_level"`

	// Background avoids tailing the output of containers, and displaying it as
	// log messages (default: true).
	Background bool `toml:"background"`

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

	// KeepService keeps the service after all instances have finished and
	// all logs have been piped. Only used when running in foreground mode
	// (default is background mode).
	KeepService bool `toml:"keep_service"`
}

// ClusterSwarmRunner is a runner that creates a Docker service to launch as
// many replicated instances of a container as the run job indicates.
type ClusterSwarmRunner struct{}

// TODO runner option to keep containers alive instead of deleting them after
// the test has run.
func (*ClusterSwarmRunner) Run(ctx context.Context, input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
	var (
		image = input.ArtifactPath
		seq   = input.Seq
		log   = logging.S().With("runner", "cluster:swarm", "run_id", input.RunID)
		cfg   = *input.RunnerConfig.(*ClusterSwarmRunnerConfig)
	)

	// global timeout of 1 minute for the scheduling.
	ctx, cancelFn := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelFn()

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
		TestSidecar:        true,
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

	const maxNameLength = 63
	var shortRunID string
	prefix := fmt.Sprintf("tg-%s-%s-", input.TestPlan.Name, testcase.Name)
	suffix := "-default"
	availableChars := maxNameLength - len(prefix) - len(suffix)
	if availableChars < 4 {
		return nil, fmt.Errorf("test plan name + test case name too long")
	}
	if len(input.RunID) > availableChars {
		shortRunID = input.RunID[len(input.RunID)-availableChars:]
	} else {
		shortRunID = input.RunID
	}

	var (
		sname    = prefix + shortRunID
		replicas = uint64(input.Instances)
	)

	// first check if redis is running.
	svcs, err := cli.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("name", "testground-redis")),
	})

	if err != nil {
		return nil, err
	} else if len(svcs) == 0 {
		return nil, fmt.Errorf("testground-redis service doesn't exist in the swarm cluster; aborting")
	}

	// Create the data network.
	log.Infow("creating data network", "name", sname)

	networkSpec := types.NetworkCreate{
		Driver:         "overlay",
		CheckDuplicate: true,
		// EnableIPv6:     true, // TODO(steb): this breaks.
		Internal:   true,
		Attachable: true,
		Scope:      "swarm",
		IPAM: &network.IPAM{
			Driver: "default",
			Config: []network.IPAMConfig{{
				Subnet:  "192.168.0.0/16",
				Gateway: "192.168.0.1",
			}},
		},
		Labels: map[string]string{
			"testground.plan":     input.TestPlan.Name,
			"testground.testcase": testcase.Name,
			"testground.runid":    input.RunID,
			"testground.name":     "default", // default name. TODO: allow multiple networks.
		},
	}

	networkResp, err := cli.NetworkCreate(ctx, sname+suffix, networkSpec)
	if err != nil {
		return nil, err
	}

	networkID := networkResp.ID
	defer func() {
		if cfg.KeepService {
			log.Info("skipping removing the data network due to user request")
			// if we are keeping the service, we must also keep the network.
			return
		}
		err = retry(5, 1*time.Second, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			return cli.NetworkRemove(ctx, networkID)
		})
		if err != nil {
			log.Errorw("couldn't remove network", "network", networkID, "err", err)
		}
	}()

	log.Infow("network created successfully", "id", networkID)

	// Create the service.
	log.Infow("creating service", "name", sname, "image", image, "replicas", replicas)

	serviceSpec := swarm.ServiceSpec{
		Networks: []swarm.NetworkAttachmentConfig{
			{Target: "control"},
			{Target: networkID},
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
				Labels: map[string]string{
					"testground.plan":     input.TestPlan.Name,
					"testground.testcase": testcase.Name,
					"testground.runid":    input.RunID,
				},
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
				MaxReplicas: 10000,
				Constraints: []string{
					"node.labels.TGRole==worker",
				},
			},
		},
	}

	logging.S().Infof("fetching an authorization token from AWS ECR")

	// Get an authorization token from AWS ECR.
	auth, err := aws.ECR.GetAuthToken(input.EnvConfig.AWS)
	if err != nil {
		return nil, err
	}

	logging.S().Infof("fetched an authorization token from AWS ECR")

	scopts := types.ServiceCreateOptions{
		QueryRegistry: true,
		// the registry auth will be propagated to all docker swarm nodes so
		// they can fetch the image properly.
		EncodedRegistryAuth: aws.ECR.EncodeAuthToken(auth),
	}

	logging.S().Infow("creating the service on docker swarm", "name", sname, "image", image, "replicas", replicas)

	// Now create the docker swarm service.
	serviceResp, err := cli.ServiceCreate(ctx, serviceSpec, scopts)
	if err != nil {
		return nil, err
	}

	serviceID := serviceResp.ID

	logging.S().Infow("service created successfully", "id", serviceID)

	out := &api.RunOutput{RunnerID: serviceID}

	// If we are running in background mode, return immediately.
	if cfg.Background {
		return out, nil
	}

	// Tail the service until all instances are done, then remove the service if
	// the flag has been set.
	rc, err := cli.ServiceLogs(context.Background(), serviceID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      "2019-01-01T00:00:00",
		Follow:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed while tailing logs: %w", err)
	}

	// Docker multiplexes STDOUT and STDERR streams inside the single IO stream
	// returned by ServiceLogs. We need to use docker functions to separate
	// those strands, and because we don't care about treating STDOUT and STDERR
	// separately, we consolidate them into the same io.Writer.
	rpipe, wpipe := io.Pipe()
	go func() {
		_, err := stdcopy.StdCopy(wpipe, wpipe, rc)
		_ = wpipe.CloseWithError(err)
	}()

	// This goroutine monitors the state of tasks every two seconds. When all
	// tasks are shutdown, we are done here. We close the logs io.ReadCloser,
	// which in turns signals that the runner is now finished.
	go func() {
		var errCnt int

		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()

		for range tick.C {
			tctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			tasks, err := cli.TaskList(tctx, types.TaskListOptions{
				Filters: filters.NewArgs(filters.Arg("service", serviceID)),
			})
			cancel()

			if err != nil {
				if errCnt++; errCnt > 3 {
					rc.Close()
					return
				}
			}

			var finished uint64
			status := make(map[swarm.TaskState]uint64, replicas)
			for _, t := range tasks {
				s := t.Status.State
				switch status[s]++; s {
				case swarm.TaskStateShutdown, swarm.TaskStateComplete, swarm.TaskStateFailed, swarm.TaskStateRejected:
					finished++
				}
			}

			logging.S().Infow("task status", "status", status)

			if finished == replicas {
				rc.Close()
				return
			}
		}
	}()

	scanner := bufio.NewScanner(rpipe)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if !cfg.KeepService {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		logging.S().Infow("removing service", "service", serviceID)

		if err := cli.ServiceRemove(ctx, serviceID); err == nil {
			logging.S().Infow("service removed", "service", serviceID)
		} else {
			logging.S().Errorf("removing the service failed: %w", err)
		}
	} else {
		log.Info("skipping removing the service due to user request")
	}

	return out, nil
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

func retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

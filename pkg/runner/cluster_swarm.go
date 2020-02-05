package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/aws"
	"github.com/ipfs/testground/pkg/conv"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
	"golang.org/x/sync/errgroup"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
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
		seq = input.Seq
		log = logging.S().With("runner", "cluster:swarm", "run_id", input.RunID)
		cfg = *input.RunnerConfig.(*ClusterSwarmRunnerConfig)
	)

	// global timeout of 1 minute for the scheduling.
	ctx, cancelFn := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelFn()

	// Sanity check.
	if seq < 0 || seq >= len(input.TestPlan.TestCases) {
		return nil, fmt.Errorf("invalid test case seq %d for plan %s", seq, input.TestPlan.Name)
	}

	// Get the test case.
	var (
		testcase = input.TestPlan.TestCases[seq]
		parent   = fmt.Sprintf("tg-%s-%s-%s", input.TestPlan.Name, testcase.Name, input.RunID)
	)

	// Build a runenv.
	template := runtime.RunEnv{
		TestPlan:          input.TestPlan.Name,
		TestCase:          testcase.Name,
		TestRun:           input.RunID,
		TestCaseSeq:       seq,
		TestInstanceCount: input.TotalInstances,
		TestSidecar:       true,
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

	// first check if redis is running.
	svcs, err := cli.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("name", "testground-redis")),
	})

	if err != nil {
		return nil, err
	} else if len(svcs) == 0 {
		return nil, fmt.Errorf("testground-redis service doesn't exist in the swarm cluster; aborting")
	}

	// We can't create a network for every testplan on the same range,
	// so we check how many networks we have and decide based on this number
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "testground.name=default")),
	})
	if err != nil {
		return nil, err
	}

	subnet, gateway, err := nextDataNetwork(len(networks))
	if err != nil {
		return nil, err
	}

	template.TestSubnet = &runtime.IPNet{IPNet: *subnet}

	// Create the data network.
	log.Infow("creating data network", "parent", parent, "subnet", subnet)

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
				Subnet:  subnet.String(),
				Gateway: gateway,
			}},
		},
		Labels: map[string]string{
			"testground.plan":     input.TestPlan.Name,
			"testground.testcase": testcase.Name,
			"testground.run_id":   input.RunID,
			"testground.name":     "default", // default name. TODO: allow multiple networks.
		},
	}

	networkResp, err := cli.NetworkCreate(ctx, parent+"-default", networkSpec)
	if err != nil {
		return nil, err
	}

	networkID := networkResp.ID
	log.Infow("network created successfully", "id", networkID)

	defer func() {
		if cfg.KeepService || cfg.Background {
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

	logging.S().Infof("fetching an authorization token from AWS ECR")

	// Get an authorization token from AWS ECR.
	auth, err := aws.ECR.GetAuthToken(input.EnvConfig.AWS)
	if err != nil {
		return nil, err
	}

	logging.S().Infof("fetched an authorization token from AWS ECR")

	services := make(map[string]int, len(input.Groups))
	for _, g := range input.Groups {
		runenv := template
		runenv.TestGroupID = g.ID
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestInstanceParams = g.Parameters

		// Serialize the runenv into env variables to pass to docker.
		env := conv.ToOptionsSlice(runenv.ToEnvVars())

		// Set the log level if provided in cfg.
		if cfg.LogLevel != "" {
			env = append(env, "LOG_LEVEL="+cfg.LogLevel)
		}

		// Create the service.
		log.Infow("creating service", "parent", parent, "group", g.ID, "image", g.ArtifactPath, "replicas", g.Instances)

		cnt := (uint64)(runenv.TestGroupInstanceCount)
		serviceSpec := swarm.ServiceSpec{
			Networks: []swarm.NetworkAttachmentConfig{
				{Target: "control"},
				{Target: networkID},
			},
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{
					Replicas: &cnt,
				},
			},
			Annotations: swarm.Annotations{Name: parent},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: &swarm.ContainerSpec{
					Image: g.ArtifactPath,
					Env:   env,
					Labels: map[string]string{
						"testground.plan":     input.TestPlan.Name,
						"testground.testcase": testcase.Name,
						"testground.run_id":   input.RunID,
						"testground.groupid":  g.ID,
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

		scopts := types.ServiceCreateOptions{
			QueryRegistry: true,
			// the registry auth will be propagated to all docker swarm nodes so
			// they can fetch the image properly.
			EncodedRegistryAuth: aws.ECR.EncodeAuthToken(auth),
		}

		logging.S().Infow("creating the service on docker swarm", "parent", parent, "group", g.ID, "image", g.ArtifactPath, "replicas", g.Instances)

		// Now create the docker swarm service.
		serviceResp, err := cli.ServiceCreate(ctx, serviceSpec, scopts)
		if err != nil {
			return nil, err
		}

		logging.S().Infow("service created successfully", "id", serviceResp.ID)

		services[serviceResp.ID] = g.Instances
	}

	// If we are running in background mode, return immediately.
	if cfg.Background {
		return &api.RunOutput{RunID: input.RunID}, nil
	}

	// Docker multiplexes STDOUT and STDERR streams inside the single IO stream
	// returned by ServiceLogs. We need to use docker functions to separate
	// those strands, and because we don't care about treating STDOUT and STDERR
	// separately, we consolidate them into the same io.Writer.
	rpipe, wpipe := io.Pipe()

	// Tail all services until all instances are done, then remove the service
	// if the flag has been set.
	errgrp, ctx := errgroup.WithContext(ctx)
	for service, count := range services {
		rc, err := cli.ServiceLogs(context.Background(), service, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Since:      "2019-01-01T00:00:00",
			Follow:     true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed while tailing logs: %w", err)
		}

		// Pipe logs to the merging pipe.
		errgrp.Go(func() error {
			// StdCopy reads until EOF, which is triggered by closing rc when the service has finished (below).
			_, err := stdcopy.StdCopy(wpipe, wpipe, rc)
			return err
		})

		// This goroutine monitors the state of tasks every two seconds. When all
		// tasks are shutdown, we are done here. We close the logs io.ReadCloser,
		// which in turns signals that the runner is now finished.
		errgrp.Go(func(service string, count int) func() error {
			return func() error {
				tick := time.NewTicker(2 * time.Second)
				defer tick.Stop()
				defer rc.Close()

				for range tick.C {
					var finished int
					tasks, err := cli.TaskList(ctx, types.TaskListOptions{
						Filters: filters.NewArgs(filters.Arg("service", service)),
					})

					if err != nil {
						return err
					}

					status := make(map[swarm.TaskState]uint64, count)
					for _, t := range tasks {
						s := t.Status.State
						switch status[s]++; s {
						case swarm.TaskStateShutdown, swarm.TaskStateComplete, swarm.TaskStateFailed, swarm.TaskStateRejected:
							finished++
						}
					}
					logging.S().Infow("task status", "service", service, "status", status)
					if finished == count {
						break
					}
				}
				return nil
			}
		}(service, count))

		go func() {
			err := errgrp.Wait()
			_ = wpipe.CloseWithError(err)
		}()
	}

	scanner := bufio.NewScanner(rpipe)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if !cfg.KeepService {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		for service := range services {
			logging.S().Infow("removing service", "service", service)

			if err := cli.ServiceRemove(ctx, service); err == nil {
				logging.S().Infow("service removed", "service", service)
			} else {
				logging.S().Errorf("removing the service failed: %w", err)
			}
		}
	} else {
		log.Info("skipping removing the service due to user request")
	}

	return &api.RunOutput{RunID: input.RunID}, nil
}

func (*ClusterSwarmRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, w io.Writer) error {
	return errors.New("unimplemented")
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

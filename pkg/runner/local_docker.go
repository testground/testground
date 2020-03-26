package runner

import (
	"context"
	_ "errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"

	"github.com/ipfs/testground/pkg/api"
	"github.com/ipfs/testground/pkg/conv"
	"github.com/ipfs/testground/pkg/docker"
	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/hashicorp/go-multierror"
	"github.com/imdario/mergo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const InfraMaxFilesUlimit int64 = 1048576

var (
	_ api.Runner        = (*LocalDockerRunner)(nil)
	_ api.Healthchecker = (*LocalDockerRunner)(nil)
	_ api.Terminatable  = (*LocalDockerRunner)(nil)
)

// LocalDockerRunnerConfig is the configuration object of this runner. Boolean
// values are expressed in a way that zero value (false) is the default setting.
type LocalDockerRunnerConfig struct {
	// KeepContainers retains test containers even after they exit (default:
	// false).
	KeepContainers bool `toml:"keep_containers"`
	// LogLevel sets the log level in the test containers (default: not set).
	LogLevel string `toml:"log_level"`
	// Unstarted creates the containers without starting them (default: false).
	Unstarted bool `toml:"no_start"`
	// Background avoids tailing the output of containers, and displaying it as
	// log messages (default: false).
	Background bool `toml:"background"`
}

// defaultConfig is the default configuration. Incoming configurations will be
// merged with this object.
var defaultConfig = LocalDockerRunnerConfig{
	KeepContainers: false,
	Unstarted:      false,
	Background:     false,
}

// LocalDockerRunner is a runner that manually stands up as many docker
// containers as instances the run job indicates.
//
// It creates a user-defined bridge, to which it attaches a redis service, and
// all the containers that belong to this test case. It then monitors all test
// containers, and destroys the setup once all workloads are done.
//
// What we do here is slightly similar to what Docker Compose does, but we can't
// use the latter because it's a python program and it doesn't expose a network
// API.
type LocalDockerRunner struct {
	lk sync.RWMutex

	outputsDir string
}

func (r *LocalDockerRunner) Healthcheck(fix bool, engine api.Engine, writer io.Writer) (*api.HealthcheckReport, error) {
	r.lk.Lock()
	defer r.lk.Unlock()

	// This context must be long, because some fixes will end up downloading
	// Docker images.
	ctx, cancel := context.WithTimeout(engine.Context(), 5*time.Minute)
	defer cancel()

	log := logging.S().With("runner", "local:docker")

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	report := api.HealthcheckReport{}
	hcHelper := ErrgroupHealthcheckHelper{report: &report}

	hcHelper.Enlist("control-network",
		DockerNetworkChecker(ctx,
			log,
			cli,
			"testground-control"),
		DockerNetworkFixer(ctx,
			log,
			cli))

	// prometheus built from Dockerfile.
	hcHelper.Enlist("local-prometheus",
		DefaultContainerChecker(ctx,
			log,
			cli,
			"testground-prometheus"),
		CustomContainerFixer(ctx,
			log,
			cli,
			filepath.Join(engine.EnvConfig().SrcDir, "infra/docker/testground-prometheus"),
			"testground-prometheus",
			"testground-prometheus:latest",
			"testground-control",
			[]string{"9090:9090"},
			false))

	// pushgateway
	hcHelper.Enlist("local-pushgateway",
		DefaultContainerChecker(ctx,
			log,
			cli,
			"testground-pushgateway"),
		DefaultContainerFixer(ctx,
			log,
			cli,
			"testground-pushgateway",
			"prom/pushgateway",
			"testground-control",
			[]string{"9091:9091"},
			true))

	// grafana
	hcHelper.Enlist("local-grafana",
		DefaultContainerChecker(ctx,
			log,
			cli,
			"testground-grafana"),
		DefaultContainerFixer(ctx,
			log,
			cli,
			"testground-grafana",
			"bitnami/grafana",
			"testground-control",
			[]string{"3000:3000"},
			true))

	// redis
	hcHelper.Enlist("local-redis",
		DefaultContainerChecker(ctx,
			log,
			cli,
			"testground-redis"),
		DefaultContainerFixer(ctx,
			log,
			cli,
			"testground-redis",
			"redis",
			"testground-control",
			[]string{"6379:6379"},
			true))

	hcHelper.Enlist("local-redis-exporter",
		DefaultContainerChecker(ctx,
			log,
			cli,
			"testground-redis-exporter"),
		DefaultContainerFixer(ctx,
			log,
			cli,
			"testground-redis",
			"redis",
			"testground-control",
			[]string{"1921:1921"},
			true,
			"--redis.addr",
			"redis://testground-redis:6379"))

	// RunChecks will fill the report and return any errors.
	err = hcHelper.RunChecks(ctx, fix)

	return &report, err
}

// func (r *LocalDockerRunner) Healthcheck(fix bool, engine api.Engine, writer io.Writer) (*api.HealthcheckReport, error) {
// 	r.lk.Lock()
// 	defer r.lk.Unlock()
//
// 	// Reset state.
// 	r.controlNetworkID = ""
// 	r.outputsDir = ""
//
// 	// This context must be long, because some fixes will end up downloading
// 	// Docker images.
// 	ctx, cancel := context.WithTimeout(engine.Context(), 5*time.Minute)
// 	defer cancel()
//
// 	log := logging.S().With("runner", "local:docker")
//
// 	// Create a docker client.
// 	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	var (
// 		ctrlNetCheck                api.HealthcheckItem
// 		grafanaContainerCheck       api.HealthcheckItem
// 		outputsDirCheck             api.HealthcheckItem
// 		prometheusContainerCheck    api.HealthcheckItem
// 		pushgatewayContainerCheck   api.HealthcheckItem
// 		redisContainerCheck         api.HealthcheckItem
// 		redisExporterContainerCheck api.HealthcheckItem
// 		sidecarContainerCheck       api.HealthcheckItem
// 	)
//
// 	networks, err := docker.CheckBridgeNetwork(ctx, log, cli, "testground-control")
// 	if err == nil {
// 		switch len(networks) {
// 		case 0:
// 			msg := "control network: not created"
// 			ctrlNetCheck = api.HealthcheckItem{Name: "control-network", Status: api.HealthcheckStatusFailed, Message: msg}
// 		default:
// 			msg := "control network: exists"
// 			ctrlNetCheck = api.HealthcheckItem{Name: "control-network", Status: api.HealthcheckStatusOK, Message: msg}
// 			r.controlNetworkID = networks[0].ID
// 		}
// 	} else {
// 		msg := fmt.Sprintf("control network errored: %s", err)
// 		ctrlNetCheck = api.HealthcheckItem{Name: "control-network", Status: api.HealthcheckStatusAborted, Message: msg}
// 	}
//
// 	ci, err := docker.CheckContainer(ctx, log, cli, "testground-grafana")
// 	if err == nil {
// 		switch {
// 		case ci == nil:
// 			msg := "grafana container: non-existent"
// 			grafanaContainerCheck = api.HealthcheckItem{Name: "grafana-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		case ci.State.Running:
// 			msg := "grafana container: running"
// 			grafanaContainerCheck = api.HealthcheckItem{Name: "grafana-container", Status: api.HealthcheckStatusOK, Message: msg}
// 		default:
// 			msg := fmt.Sprintf("grafana container: status %s", ci.State.Status)
// 			grafanaContainerCheck = api.HealthcheckItem{Name: "grafana-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		}
// 	} else {
// 		msg := fmt.Sprintf("grafana container errored: %s", err)
// 		grafanaContainerCheck = api.HealthcheckItem{Name: "grafana-container", Status: api.HealthcheckStatusAborted, Message: msg}
// 	}
//
// 	// Ensure the outputs dir exists.
// 	r.outputsDir = filepath.Join(engine.EnvConfig().WorkDir(), "local_docker", "outputs")
// 	if _, err := os.Stat(r.outputsDir); err == nil {
// 		msg := "outputs directory exists"
// 		outputsDirCheck = api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusOK, Message: msg}
// 	} else if os.IsNotExist(err) {
// 		msg := "outputs directory does not exist"
// 		outputsDirCheck = api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusFailed, Message: msg}
// 	} else {
// 		msg := fmt.Sprintf("failed to stat outputs directory: %s", err)
// 		outputsDirCheck = api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusAborted, Message: msg}
// 	}
//
// 	ci, err = docker.CheckContainer(ctx, log, cli, "testground-prometheus")
// 	if err == nil {
// 		switch {
// 		case ci == nil:
// 			msg := "prometheus container: non-existent"
// 			prometheusContainerCheck = api.HealthcheckItem{Name: "prometheus-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		case ci.State.Running:
// 			msg := "prometheus container: running"
// 			prometheusContainerCheck = api.HealthcheckItem{Name: "prometheus-container", Status: api.HealthcheckStatusOK, Message: msg}
// 		default:
// 			msg := fmt.Sprintf("prometheus container: status %s", ci.State.Status)
// 			prometheusContainerCheck = api.HealthcheckItem{Name: "prometheus-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		}
// 	} else {
// 		msg := fmt.Sprintf("prometheus container errored: %s", err)
// 		prometheusContainerCheck = api.HealthcheckItem{Name: "prometheus-container", Status: api.HealthcheckStatusAborted, Message: msg}
// 	}
//
// 	ci, err = docker.CheckContainer(ctx, log, cli, "prometheus-pushgateway")
// 	if err == nil {
// 		switch {
// 		case ci == nil:
// 			msg := "pushgateway container: non-existent"
// 			pushgatewayContainerCheck = api.HealthcheckItem{Name: "pushgateway-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		case ci.State.Running:
// 			msg := "pushgateway container: running"
// 			pushgatewayContainerCheck = api.HealthcheckItem{Name: "pushgateway-container", Status: api.HealthcheckStatusOK, Message: msg}
// 		default:
// 			msg := fmt.Sprintf("pushgateway container: status %s", ci.State.Status)
// 			pushgatewayContainerCheck = api.HealthcheckItem{Name: "pushgateway-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		}
// 	} else {
// 		msg := fmt.Sprintf("pushgateway container errored: %s", err)
// 		pushgatewayContainerCheck = api.HealthcheckItem{Name: "pushgateway-container", Status: api.HealthcheckStatusAborted, Message: msg}
// 	}
//
// 	ci, err = docker.CheckContainer(ctx, log, cli, "testground-redis")
// 	if err == nil {
// 		switch {
// 		case ci == nil:
// 			msg := "redis container: non-existent"
// 			redisContainerCheck = api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		case ci.State.Running:
// 			msg := "redis container: running"
// 			redisContainerCheck = api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusOK, Message: msg}
// 		default:
// 			msg := fmt.Sprintf("redis container: status %s", ci.State.Status)
// 			redisContainerCheck = api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		}
// 	} else {
// 		msg := fmt.Sprintf("redis container errored: %s", err)
// 		redisContainerCheck = api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusAborted, Message: msg}
// 	}
//
// 	ci, err = docker.CheckContainer(ctx, log, cli, "testground-redis-exporter")
// 	if err == nil {
// 		switch {
// 		case ci == nil:
// 			msg := "redis-exporter container: non-existent"
// 			redisExporterContainerCheck = api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		case ci.State.Running:
// 			msg := "redis-exporter container: running"
// 			redisExporterContainerCheck = api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusOK, Message: msg}
// 		default:
// 			msg := fmt.Sprintf("redis-exporter container: status %s", ci.State.Status)
// 			redisExporterContainerCheck = api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		}
// 	} else {
// 		msg := fmt.Sprintf("redis-exporter container errored: %s", err)
// 		redisExporterContainerCheck = api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusAborted, Message: msg}
// 	}
//
// 	ci, err = docker.CheckContainer(ctx, log, cli, "testground-sidecar")
// 	if err == nil {
// 		switch {
// 		case ci == nil:
// 			msg := "sidecar container: non-existent"
// 			sidecarContainerCheck = api.HealthcheckItem{Name: "sidecar-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		case ci.State.Running:
// 			msg := "sidecar container: running"
// 			sidecarContainerCheck = api.HealthcheckItem{Name: "sidecar-container", Status: api.HealthcheckStatusOK, Message: msg}
// 		default:
// 			msg := fmt.Sprintf("sidecar container: status %s", ci.State.Status)
// 			sidecarContainerCheck = api.HealthcheckItem{Name: "sidecar-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 		}
// 	} else {
// 		msg := fmt.Sprintf("sidecar container errored: %s", err)
// 		sidecarContainerCheck = api.HealthcheckItem{Name: "sidecar-container", Status: api.HealthcheckStatusAborted, Message: msg}
// 	}
//
// 	report := &api.HealthcheckReport{
// 		Checks: []api.HealthcheckItem{
// 			ctrlNetCheck,
// 			grafanaContainerCheck,
// 			outputsDirCheck,
// 			prometheusContainerCheck,
// 			pushgatewayContainerCheck,
// 			redisContainerCheck,
// 			redisExporterContainerCheck,
// 			sidecarContainerCheck,
// 		},
// 	}
//
// 	if !fix {
// 		return report, nil
// 	}
//
// 	// FIX LOGIC ====================
//
// 	var fixes []api.HealthcheckItem
//
// 	if ctrlNetCheck.Status != api.HealthcheckStatusOK {
// 		id, err := ensureControlNetwork(ctx, cli, log)
// 		if err == nil {
// 			r.controlNetworkID = id
// 			msg := "control network created successfully"
// 			it := api.HealthcheckItem{Name: "control-network", Status: api.HealthcheckStatusOK, Message: msg}
// 			fixes = append(fixes, it)
// 		} else {
// 			msg := fmt.Sprintf("failed to create control network: %s", err)
// 			it := api.HealthcheckItem{Name: "control-network", Status: api.HealthcheckStatusFailed, Message: msg}
// 			fixes = append(fixes, it)
// 		}
// 	}
//
// 	if grafanaContainerCheck.Status != api.HealthcheckStatusOK {
// 		switch r.controlNetworkID {
// 		case "":
// 			msg := "omitted creation of grafana container; no control network"
// 			it := api.HealthcheckItem{Name: "grafana-container", Status: api.HealthcheckStatusOmitted, Message: msg}
// 			fixes = append(fixes, it)
// 		default:
// 			if err == nil {
// 				_, err := ensureInfraContainer(ctx, cli, log, "testground-grafana", "bitnami/grafana", r.controlNetworkID, true)
// 				if err == nil {
// 					msg := "grafana container created successfully"
// 					it := api.HealthcheckItem{Name: "grafana-container", Status: api.HealthcheckStatusOK, Message: msg}
// 					fixes = append(fixes, it)
// 				} else {
// 					msg := fmt.Sprintf("failed to create grafana container: %s", err)
// 					it := api.HealthcheckItem{Name: "grafana-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 					fixes = append(fixes, it)
// 				}
// 			} else {
// 				msg := fmt.Sprintf("failed to create grafana image: %s", err)
// 				it := api.HealthcheckItem{Name: "grafana-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 				fixes = append(fixes, it)
// 			}
// 		}
// 	}
//
// 	if outputsDirCheck.Status != api.HealthcheckStatusOK {
// 		if err := os.MkdirAll(r.outputsDir, 0777); err == nil {
// 			msg := "outputs dir created successfully"
// 			it := api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusOK, Message: msg}
// 			fixes = append(fixes, it)
// 		} else {
// 			msg := fmt.Sprintf("failed to create outputs dir: %s", err)
// 			it := api.HealthcheckItem{Name: "outputs-dir", Status: api.HealthcheckStatusFailed, Message: msg}
// 			fixes = append(fixes, it)
// 		}
// 	}
//
// 	if prometheusContainerCheck.Status != api.HealthcheckStatusOK {
// 		switch r.controlNetworkID {
// 		case "":
// 			msg := "omitted creation of prometheus container; no control network"
// 			it := api.HealthcheckItem{Name: "prometheus-container", Status: api.HealthcheckStatusOmitted, Message: msg}
// 			fixes = append(fixes, it)
// 		default:
// 			_, err := docker.EnsureImage(ctx, log, cli, &docker.BuildImageOpts{
// 				Name: "testground-prometheus",
// 				// This is the location of the pre-configured prometheus used by the local docker runner.
// 				BuildCtx: filepath.Join(engine.EnvConfig().SrcDir, "infra/docker/testground-prometheus"),
// 			})
//
// 			if err == nil {
// 				_, err := ensureInfraContainer(ctx, cli, log, "testground-prometheus", "testground-prometheus:latest", r.controlNetworkID, false)
// 				if err == nil {
// 					msg := "prometheus container created successfully"
// 					it := api.HealthcheckItem{Name: "prometheus-container", Status: api.HealthcheckStatusOK, Message: msg}
// 					fixes = append(fixes, it)
// 				} else {
// 					msg := fmt.Sprintf("failed to create prometheus container: %s", err)
// 					it := api.HealthcheckItem{Name: "prometheus-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 					fixes = append(fixes, it)
// 				}
// 			} else {
// 				msg := fmt.Sprintf("failed to create prometheus image: %s", err)
// 				it := api.HealthcheckItem{Name: "prometheus-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 				fixes = append(fixes, it)
// 			}
// 		}
// 	}
//
// 	if pushgatewayContainerCheck.Status != api.HealthcheckStatusOK {
// 		switch r.controlNetworkID {
// 		case "":
// 			msg := "omitted creation of pushgateway container; no control network"
// 			it := api.HealthcheckItem{Name: "pushgateway-container", Status: api.HealthcheckStatusOmitted, Message: msg}
// 			fixes = append(fixes, it)
// 		default:
// 			_, err := ensureInfraContainer(ctx, cli, log, "prometheus-pushgateway", "prom/pushgateway", r.controlNetworkID, true)
// 			if err == nil {
// 				msg := "pushgateway container created successfully"
// 				it := api.HealthcheckItem{Name: "pushgateway-container", Status: api.HealthcheckStatusOK, Message: msg}
// 				fixes = append(fixes, it)
// 			} else {
// 				msg := fmt.Sprintf("failed to create pushgateway container: %s", err)
// 				it := api.HealthcheckItem{Name: "pushgateway-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 				fixes = append(fixes, it)
// 			}
// 		}
// 	}
//
// 	if redisContainerCheck.Status != api.HealthcheckStatusOK {
// 		switch r.controlNetworkID {
// 		case "":
// 			msg := "omitted creation of redis container; no control network"
// 			it := api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusOmitted, Message: msg}
// 			fixes = append(fixes, it)
// 		default:
// 			_, err := ensureInfraContainer(ctx, cli, log, "testground-redis", "redis", r.controlNetworkID, true)
// 			if err == nil {
// 				msg := "redis container created successfully"
// 				it := api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusOK, Message: msg}
// 				fixes = append(fixes, it)
// 			} else {
// 				msg := fmt.Sprintf("failed to create redis container: %s", err)
// 				it := api.HealthcheckItem{Name: "redis-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 				fixes = append(fixes, it)
// 			}
// 		}
// 	}
//
// 	if redisExporterContainerCheck.Status != api.HealthcheckStatusOK {
// 		switch r.controlNetworkID {
// 		case "":
// 			msg := "omitted creation of redis-exporter container; no control network"
// 			it := api.HealthcheckItem{Name: "redis-exporter-container", Status: api.HealthcheckStatusOmitted, Message: msg}
// 			fixes = append(fixes, it)
// 		default:
// 			// Redis exporter arguments
// 			args := []string{
// 				"--redis.addr",
// 				"redis://testground-redis:6379",
// 			}
// 			_, err := ensureInfraContainer(ctx, cli, log, "testground-redis-exporter", "bitnami/redis-exporter", r.controlNetworkID, true, args...)
// 			if err == nil {
// 				msg := "redis-exporter container created successfully"
// 				it := api.HealthcheckItem{Name: "redis-exporter-container", Status: api.HealthcheckStatusOK, Message: msg}
// 				fixes = append(fixes, it)
// 			} else {
// 				msg := fmt.Sprintf("failed to create redis-exporter container: %s", err)
// 				it := api.HealthcheckItem{Name: "redis-exporter-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 				fixes = append(fixes, it)
// 			}
// 		}
// 	}
//
// 	if sidecarContainerCheck.Status != api.HealthcheckStatusOK {
// 		switch r.controlNetworkID {
// 		case "":
// 			msg := "omitted creation of sidecar container; no control network"
// 			it := api.HealthcheckItem{Name: "sidecar-container", Status: api.HealthcheckStatusOmitted, Message: msg}
// 			fixes = append(fixes, it)
// 		default:
// 			_, err := ensureSidecarContainer(ctx, cli, r.outputsDir, log, r.controlNetworkID)
// 			if err == nil {
// 				msg := "control network created successfully"
// 				it := api.HealthcheckItem{Name: "sidecar-container", Status: api.HealthcheckStatusOK, Message: msg}
// 				fixes = append(fixes, it)
// 			} else {
// 				msg := fmt.Sprintf("failed to create control network: %s", err)
//
// 				if err == errors.New("image not found") {
// 					msg += "; docker image ipfs/testground not found, run `make docker-ipfs-testground`"
// 				}
//
// 				it := api.HealthcheckItem{Name: "sidecar-container", Status: api.HealthcheckStatusFailed, Message: msg}
// 				fixes = append(fixes, it)
// 			}
// 		}
// 	}
//
// 	report.Fixes = fixes
// 	return report, nil
// }

func (r *LocalDockerRunner) Run(ctx context.Context, input *api.RunInput, ow io.Writer) (*api.RunOutput, error) {
	// Grab a read lock. This will allow many runs to run simultaneously, but
	// they will be exclusive of state-altering healthchecks.
	r.lk.RLock()
	defer r.lk.RUnlock()

	var (
		seq = input.Seq
		log = logging.S().With("runner", "local:docker", "run_id", input.RunID)
		err error
	)

	// Sanity check.
	if seq < 0 || seq >= len(input.TestPlan.TestCases) {
		return nil, fmt.Errorf("invalid test case seq %d for plan %s", seq, input.TestPlan.Name)
	}

	// Get the test case.
	testcase := input.TestPlan.TestCases[seq]

	// Create a docker client.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	// Build a template runenv.
	template := runtime.RunParams{
		TestPlan:          input.TestPlan.Name,
		TestCase:          testcase.Name,
		TestRun:           input.RunID,
		TestCaseSeq:       seq,
		TestInstanceCount: input.TotalInstances,
		TestSidecar:       true,
		TestOutputsPath:   "/outputs",
		TestStartTime:     time.Now(),
	}

	// Create a data network.
	dataNetworkID, subnet, err := newDataNetwork(ctx, cli, logging.S(), &template, "default")
	if err != nil {
		return nil, err
	}

	template.TestSubnet = &runtime.IPNet{IPNet: *subnet}

	// Merge the incoming configuration with the default configuration.
	cfg := defaultConfig
	if err := mergo.Merge(&cfg, input.RunnerConfig, mergo.WithOverride); err != nil {
		return nil, fmt.Errorf("error while merging configurations: %w", err)
	}

	var containers []string
	for _, g := range input.Groups {
		runenv := template
		runenv.TestGroupInstanceCount = g.Instances
		runenv.TestGroupID = g.ID
		runenv.TestInstanceParams = g.Parameters

		// Serialize the runenv into env variables to pass to docker.
		env := conv.ToOptionsSlice(runenv.ToEnvVars())

		// Set the log level if provided in cfg.
		if cfg.LogLevel != "" {
			env = append(env, "LOG_LEVEL="+cfg.LogLevel)
		}

		// Create the run output directory and write the runenv.
		runDir := filepath.Join(r.outputsDir, input.TestPlan.Name, input.RunID, g.ID)
		if err := os.MkdirAll(runDir, 0777); err != nil {
			return nil, err
		}

		// Start as many containers as group instances.
		for i := 0; i < g.Instances; i++ {
			// <outputs_dir>/<plan>/<run_id>/<group_id>/<instance_number>
			odir := filepath.Join(r.outputsDir, input.TestPlan.Name, input.RunID, g.ID, strconv.Itoa(i))
			err = os.MkdirAll(odir, 0777)
			if err != nil {
				err = fmt.Errorf("failed to create outputs dir %s: %w", odir, err)
				break
			}

			name := fmt.Sprintf("tg-%s-%s-%s-%s-%d", input.TestPlan.Name, testcase.Name, input.RunID, g.ID, i)
			log.Infow("creating container", "name", name)

			ccfg := &container.Config{
				Image: g.ArtifactPath,
				Env:   env,
				Labels: map[string]string{
					"testground.purpose":  "plan",
					"testground.plan":     input.TestPlan.Name,
					"testground.testcase": testcase.Name,
					"testground.run_id":   input.RunID,
					"testground.group_id": g.ID,
				},
			}

			hcfg := &container.HostConfig{
				NetworkMode:     container.NetworkMode("testground-control"),
				PublishAllPorts: true,
				Mounts: []mount.Mount{{
					Type:   mount.TypeBind,
					Source: odir,
					Target: runenv.TestOutputsPath,
				}},
			}

			// Create the container.
			var res container.ContainerCreateCreatedBody
			res, err = cli.ContainerCreate(ctx, ccfg, hcfg, nil, name)
			if err != nil {
				break
			}

			containers = append(containers, res.ID)

			// TODO: Remove this when we get the sidecar working. It'll do this for us.
			err = attachContainerToNetwork(ctx, cli, res.ID, dataNetworkID)
			if err != nil {
				break
			}
		}
	}

	if !cfg.KeepContainers {
		defer func() {
			_ = deleteContainers(cli, log, containers)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := cli.NetworkRemove(ctx, dataNetworkID); err != nil {
				log.Errorw("removing network", "network", dataNetworkID, "error", err)
			}
		}()
	}

	// If an error occurred interim, abort.
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if cfg.Unstarted {
		return &api.RunOutput{RunID: input.RunID}, nil
	}

	var (
		doneCh    = make(chan error, 2)
		started   = make(chan string, len(containers))
		ratelimit = make(chan struct{}, 16)
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Infow("starting containers", "count", len(containers))

	g, gctx := errgroup.WithContext(ctx)
	for _, id := range containers {
		id := id
		f := func() error {
			ratelimit <- struct{}{}
			defer func() { <-ratelimit }()

			log.Infow("starting container", "id", id)

			err := cli.ContainerStart(ctx, id, types.ContainerStartOptions{})
			if err == nil {
				log.Debugw("started container", "id", id)
				select {
				case <-gctx.Done():
				default:
					started <- id
				}
			}
			return err
		}
		g.Go(f)
	}

	// Wait until we're done to close the started channel.
	go func() {
		err := g.Wait()
		close(started)

		if err != nil {
			log.Error(err)
			doneCh <- err
		} else {
			log.Infow("started containers", "count", len(containers))
		}
	}()

	if !cfg.Background {
		pretty := NewPrettyPrinter()

		// This goroutine takes started containers and attaches them to the pretty printer.
		go func() {
		Outer:
			for {
				select {
				case id, more := <-started:
					if !more {
						break Outer
					}

					stream, err := cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
						ShowStdout: true,
						ShowStderr: true,
						Since:      "2019-01-01T00:00:00",
						Follow:     true,
					})

					if err != nil {
						doneCh <- err
						return
					}

					rstdout, wstdout := io.Pipe()
					rstderr, wstderr := io.Pipe()
					go func() {
						_, err := stdcopy.StdCopy(wstdout, wstderr, stream)
						_ = wstdout.CloseWithError(err)
						_ = wstderr.CloseWithError(err)
					}()

					pretty.Manage(id[0:12], rstdout, rstderr)

				case <-ctx.Done():
					// yield if we're been cancelled.
					doneCh <- ctx.Err()
					return
				}
			}

			select {
			case err := <-pretty.Wait():
				doneCh <- err
			case <-ctx.Done():
				log.Error(ctx) // yield if we're been cancelled.
				doneCh <- ctx.Err()
			}
		}()
	}

	select {
	case err = <-doneCh:
	case <-ctx.Done():
		err = ctx.Err()
	}

	return &api.RunOutput{RunID: input.RunID}, err
}

func deleteContainers(cli *client.Client, log *zap.SugaredLogger, ids []string) (err error) {
	log.Infow("deleting containers", "ids", ids)

	ratelimit := make(chan struct{}, 16)

	errs := make(chan error)
	for _, id := range ids {
		go func(id string) {
			ratelimit <- struct{}{}
			defer func() { <-ratelimit }()

			log.Infow("deleting container", "id", id)
			errs <- cli.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
		}(id)
	}

	var merr *multierror.Error
	for i := 0; i < len(ids); i++ {
		if err := <-errs; err != nil {
			log.Errorw("failed while deleting container", "error", err)
			merr = multierror.Append(merr, <-errs)
		}
	}
	close(errs)
	return merr.ErrorOrNil()
}

func ensureControlNetwork(ctx context.Context, cli *client.Client, log *zap.SugaredLogger) (id string, err error) {
	return docker.EnsureBridgeNetwork(
		ctx,
		log, cli,
		"testground-control",
		// making internal=false enables us to expose ports to the host (e.g.
		// pprof and prometheus). by itself, it would allow the container to
		// access the Internet, and therefore would break isolation, but since
		// we have sidecar overriding the default Docker ip routes, and
		// suppressing such traffic, we're safe.
		false,
		network.IPAMConfig{
			Subnet:  controlSubnet,
			Gateway: controlGateway,
		},
	)
}

func newDataNetwork(ctx context.Context, cli *client.Client, log *zap.SugaredLogger, env *runtime.RunParams, name string) (id string, subnet *net.IPNet, err error) {
	// Find a free network.
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg(
				"label",
				"testground.name=default",
			),
		),
	})
	if err != nil {
		return "", nil, err
	}

	subnet, gateway, err := nextDataNetwork(len(networks))
	if err != nil {
		return "", nil, err
	}

	id, err = docker.NewBridgeNetwork(
		ctx,
		cli,
		fmt.Sprintf("tg-%s-%s-%s-%s", env.TestPlan, env.TestCase, env.TestRun, name),
		true,
		map[string]string{
			"testground.plan":     env.TestPlan,
			"testground.testcase": env.TestCase,
			"testground.run_id":   env.TestRun,
			"testground.name":     name,
		},
		network.IPAMConfig{
			Subnet:  subnet.String(),
			Gateway: gateway,
		},
	)
	return id, subnet, err
}

// ensure container is started
func ensureInfraContainer(ctx context.Context, cli *client.Client, log *zap.SugaredLogger, containerName string, imageName string, networkID string, pull bool, cmds ...string) (id string, err error) {
	container, _, err := docker.EnsureContainer(ctx, log, cli, &docker.EnsureContainerOpts{
		ContainerName: containerName,
		ContainerConfig: &container.Config{
			Image: imageName,
			Cmd:   cmds,
		},
		HostConfig: &container.HostConfig{
			NetworkMode:     container.NetworkMode(networkID),
			PublishAllPorts: true,
			Resources: container.Resources{
				Ulimits: []*units.Ulimit{
					{Name: "nofile", Hard: InfraMaxFilesUlimit, Soft: InfraMaxFilesUlimit},
				},
			},
		},
		PullImageIfMissing: pull,
	})
	if err != nil {
		return "", err
	}

	return container.ID, err

}

// ensureSidecarContainer ensures there's a testground-sidecar container started.
func ensureSidecarContainer(ctx context.Context, cli *client.Client, workDir string, log *zap.SugaredLogger, controlNetworkID string) (id string, err error) {
	dockerSock := "/var/run/docker.sock"
	if host := cli.DaemonHost(); strings.HasPrefix(host, "unix://") {
		dockerSock = host[len("unix://"):]
	} else {
		log.Warnf("guessing docker socket as %s", dockerSock)
	}
	container, _, err := docker.EnsureContainer(ctx, log, cli, &docker.EnsureContainerOpts{
		ContainerName: "testground-sidecar",
		ContainerConfig: &container.Config{
			Image:      "ipfs/testground:latest",
			Entrypoint: []string{"testground"},
			Cmd:        []string{"sidecar", "--runner", "docker", "--pprof"},
			Env:        []string{"REDIS_HOST=testground-redis", "GODEBUG=gctrace=1"},
		},
		HostConfig: &container.HostConfig{
			PublishAllPorts: true,
			PortBindings:    nat.PortMap{"6060": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "0"}}},
			NetworkMode:     container.NetworkMode(controlNetworkID),
			// To lookup namespaces. Can't use SandboxKey for some reason.
			PidMode: "host",
			// We need _both_ to actually get a network namespace handle.
			// We may be able to drop sys_admin if we drop netlink
			// sockets that we're not using.
			CapAdd: []string{"NET_ADMIN", "SYS_ADMIN"},
			// needed to talk to docker.
			Mounts: []mount.Mount{{
				Type:   mount.TypeBind,
				Source: dockerSock,
				Target: "/var/run/docker.sock",
			}},
			Resources: container.Resources{
				Ulimits: []*units.Ulimit{
					{Name: "nofile", Hard: InfraMaxFilesUlimit, Soft: InfraMaxFilesUlimit},
				},
			},
		},
		PullImageIfMissing: false, // Don't pull from Docker Hub
	})
	if err != nil {
		return "", err
	}

	return container.ID, err
}

func (*LocalDockerRunner) CollectOutputs(ctx context.Context, input *api.CollectionInput, w io.Writer) error {
	basedir := filepath.Join(input.EnvConfig.WorkDir(), "local_docker", "outputs")
	return zipRunOutputs(ctx, basedir, input, w)
}

// attachContainerToNetwork attaches the provided container to the specified
// network.
func attachContainerToNetwork(ctx context.Context, cli *client.Client, containerID string, networkID string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return cli.NetworkConnect(ctx, networkID, containerID, nil)
}

//nolint this function is unused, but it may come in handy.
func detachContainerFromNetwork(ctx context.Context, cli *client.Client, containerID string, networkID string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return cli.NetworkDisconnect(ctx, networkID, containerID, true)
}

func (*LocalDockerRunner) ID() string {
	return "local:docker"
}

func (*LocalDockerRunner) ConfigType() reflect.Type {
	return reflect.TypeOf(LocalDockerRunnerConfig{})
}

func (*LocalDockerRunner) CompatibleBuilders() []string {
	return []string{"docker:go"}
}

// This method deletes the testground containers.
// It does *not* delete any downloaded images or networks.
// I'll leave a friendly message for how to do a more complete cleanup.
func (*LocalDockerRunner) TerminateAll(ctx context.Context) error {
	log := logging.S()
	log.Info("terminate local:docker requested")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	// Build two separate queries: one for infrastructure containers, another
	// for test plan containers. The former, we match by container name. The
	// latter, we match by the `testground.purpose` label, which we apply to all
	// plan containers managed by testground label.

	// Build query for runner infrastructure containers.
	infraOpts := types.ContainerListOptions{}
	infraOpts.Filters = filters.NewArgs()
	infraOpts.Filters.Add("name", "testground-sidecar")
	infraOpts.Filters.Add("name", "testground-redis")
	infraOpts.Filters.Add("name", "testground-goproxy")

	// Build query for testground plans that are still running.
	planOpts := types.ContainerListOptions{}
	planOpts.Filters = filters.NewArgs()
	planOpts.Filters.Add("label", "testground.purpose=plan")

	infracontainers, err := cli.ContainerList(ctx, infraOpts)
	if err != nil {
		return fmt.Errorf("failed to list infrastructure containers: %w", err)
	}
	plancontainers, err := cli.ContainerList(ctx, planOpts)
	if err != nil {
		return fmt.Errorf("failed to list test plan containers: %w", err)
	}

	containers := make([]string, 0, len(infracontainers)+len(plancontainers))
	for _, container := range infracontainers {
		containers = append(containers, container.ID)
	}
	for _, container := range plancontainers {
		containers = append(containers, container.ID)
	}

	err = deleteContainers(cli, log, containers)
	if err != nil {
		return fmt.Errorf("failed to list testground containers: %w", err)
	}

	log.Info("to delete networks and images, you may want to run `docker system prune`")
	return nil
}

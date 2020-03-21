package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"

	"go.uber.org/zap"
)

// EnsureVolumeOpts is used to construct the volume request.
// https://github.com/moby/moby/blob/master/api/types/volume/volume_create.go
type EnsureVolumeOpts struct {
	Name       string
	DriverOpts map[string]string
	Labels     map[string]string
	Driver     string
}

// EnsureContainer ensures the volume is created.
// If another volume exists with the same name, nothing is created, regardless of
// any other options passed.
func EnsureVolume(ctx context.Context, log *zap.SugaredLogger, cli *client.Client,
	opts *EnsureVolumeOpts) (volume *types.Volume, created bool, err error) {
	log = log.With("volume_name", opts.Name)

	log.Debug("checking state of volume")

	// Check whether volume exists.
	volumes, err := cli.VolumeList(ctx, filters.Args{})
	if err != nil {
		return nil, false, err
	}

	for _, v := range volumes.Volumes {
		if v.Name == opts.Name { // We found a match, volume exists.
			log.Info("found existing volume")
			return v, false, nil
		}
	}

	log.Infof("creating new docker volume")

	volCreate := volumetypes.VolumeCreateBody{
		Name:       opts.Name,
		DriverOpts: opts.DriverOpts,
		Labels:     opts.Labels,
		Driver:     opts.Driver,
	}

	vol, err := cli.VolumeCreate(ctx, volCreate)
	if err != nil {
		log.Warnw("could not create volume", "error", err)
		return nil, false, err
	}
	return &vol, true, nil
}

package gardener

import (
	"errors"
	"net/url"
	"os"
	"os/exec"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_spec"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const RawRootFSScheme = "raw"

type CommandFactory func(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd

type VolumeProvider struct {
	VolumeCreator VolumeCreator
	VolumeDestroyMetricsGC
	prepareRootfsCmdFactory func(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) *exec.Cmd
	commandRunner           commandrunner.CommandRunner
	containerRootUID        int
	containerRootGID        int
}

func NewVolumeProvider(creator VolumeCreator, manager VolumeDestroyMetricsGC, prepareRootfsCmdFactory CommandFactory, commandrunner commandrunner.CommandRunner, rootUID, rootGID int) *VolumeProvider {
	return &VolumeProvider{
		VolumeCreator:           creator,
		VolumeDestroyMetricsGC:  manager,
		prepareRootfsCmdFactory: prepareRootfsCmdFactory,
		commandRunner:           commandrunner,
		containerRootUID:        rootUID,
		containerRootGID:        rootGID,
	}
}

type VolumeCreator interface {
	Create(log lager.Logger, handle string, spec rootfs_spec.Spec) (specs.Spec, error)
}

func (v *VolumeProvider) Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error) {
	path := spec.Image.URI
	if path == "" {
		path = spec.RootFSPath
	} else if spec.RootFSPath != "" {
		return specs.Spec{}, errors.New("Cannot provide both Image.URI and RootFSPath")
	}

	rootFSURL, err := url.Parse(path)
	if err != nil {
		return specs.Spec{}, err
	}

	var baseConfig specs.Spec

	if rootFSURL.Scheme == RawRootFSScheme {
		baseConfig.Root = &specs.Root{Path: rootFSURL.Path}
		baseConfig.Process = &specs.Process{}
		return baseConfig, nil
	}

	baseConfig, err = v.VolumeCreator.Create(log.Session("volume-creator"), spec.Handle, rootfs_spec.Spec{
		RootFS:     rootFSURL,
		Username:   spec.Image.Username,
		Password:   spec.Image.Password,
		QuotaSize:  int64(spec.Limits.Disk.ByteHard),
		QuotaScope: spec.Limits.Disk.Scope,
		Namespaced: !spec.Privileged,
	})
	if err != nil {
		return specs.Spec{}, err
	}

	if err := v.mkdirChownStuff(!spec.Privileged, baseConfig); err != nil {
		return specs.Spec{}, err
	}

	return baseConfig, nil
}

func (v *VolumeProvider) mkdirChownStuff(namespaced bool, spec specs.Spec) error {
	var uid, gid int = 0, 0
	if namespaced {
		uid = v.containerRootUID
		gid = v.containerRootGID
	}

	if err := v.mkdirAs(
		spec.Root.Path, uid, gid, 0755, true,
		"dev", "proc", "sys",
	); err != nil {
		return err
	}

	if err := v.mkdirAs(
		spec.Root.Path, uid, gid, 0777, false,
		"tmp",
	); err != nil {
		return err
	}

	return nil
}

func (v *VolumeProvider) mkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error {
	return v.commandRunner.Run(v.prepareRootfsCmdFactory(
		rootFSPathFile,
		uid,
		gid,
		mode,
		recreate,
		paths...,
	))
}

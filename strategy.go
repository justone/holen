package main

import (
	"fmt"

	"github.com/kr/pretty"
	"github.com/pkg/errors"
)

type StrategyCommon struct {
	System
	Logger
	ConfigGetter
	Downloader
	Runner
}

func (sc *StrategyCommon) Templater(version string, archMap map[string]string, system System) Templater {
	archKey := fmt.Sprintf("%s_%s", system.OS(), system.Arch())
	sc.Debugf("Arch key: %s", archKey)
	value := archMap[archKey]
	return Templater{
		Version:    version,
		OS:         system.OS(),
		Arch:       system.Arch(),
		MappedArch: value,
	}
}

type Strategy interface {
	Run([]string) error
}

type DockerData struct {
	Version     string            `yaml:"version"`
	Image       string            `yaml:"image"`
	MountPwd    bool              `yaml:"mount_pwd"`
	DockerConn  bool              `yaml:"docker_conn"`
	Interactive bool              `yaml:"interactive"`
	ArchMap     map[string]string `yaml:"arch_map"`
}

type DockerStrategy struct {
	*StrategyCommon
	Data DockerData
}

type BinaryData struct {
	Version string            `yaml:"version"`
	BaseUrl string            `yaml:"base_url"`
	ArchMap map[string]string `yaml:"arch_map"`
}

type BinaryStrategy struct {
	*StrategyCommon
	Data BinaryData
}

func (ds DockerStrategy) Run(args []string) error {

	// TODO: template Image

	var err error

	temp := ds.Templater(ds.Data.Version, ds.Data.ArchMap, ds.System)
	ds.Debugf("templater: %# v", pretty.Formatter(temp))

	image, err := temp.Template(ds.Data.Image)
	if err != nil {
		return errors.Wrap(err, "unable to template image name")
	}

	err = ds.PullDockerImage(image)
	if err != nil {
		return errors.Wrap(err, "can't pull image")
	}

	ds.RunCommand("docker", append([]string{"run", image}, args...))
	if err != nil {
		return errors.Wrap(err, "can't run image")
	}

	return nil
}

func (bs BinaryStrategy) Run(args []string) error {
	// var err error

	temp := bs.Templater(bs.Data.Version, bs.Data.ArchMap, bs.System)
	bs.Debugf("templater: %# v", pretty.Formatter(temp))

	url, err := temp.Template(bs.Data.BaseUrl)
	if err != nil {
		return errors.Wrap(err, "unable to template url")
	}

	err = bs.DownloadFile(url, "local/jq")
	if err != nil {
		return errors.Wrap(err, "can't download binary")
	}

	bs.RunCommand("local/jq", args)
	if err != nil {
		return errors.Wrap(err, "can't run image")
	}

	return nil
}

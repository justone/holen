package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/kr/pretty"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type ManifestFinder interface {
	Find(string) (*Manifest, error)
}

type DefaultManifestFinder struct {
	Logger
	ConfigGetter
}

func NewManifestFinder() (*DefaultManifestFinder, error) {
	conf, err := NewDefaultConfigClient()
	if err != nil {
		return nil, err
	}

	logger := &LogrusLogger{}
	return &DefaultManifestFinder{
		Logger:       logger,
		ConfigGetter: conf,
	}, nil
}

func (dmf DefaultManifestFinder) Find(utility NameVer) (*Manifest, error) {
	md := ManifestData{}

	file := fmt.Sprintf("manifests/%s.yaml", utility.Name)

	dmf.Infof("attemting to load: %s", file)
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "problems with reading file")
	}

	err = yaml.Unmarshal([]byte(data), &md)
	if err != nil {
		return nil, errors.Wrap(err, "problems with unmarshal")
	}

	manifest := &Manifest{
		Logger:       dmf.Logger,
		ConfigGetter: dmf.ConfigGetter,
		Data:         md,
	}
	dmf.Debugf("manifest found: %# v", pretty.Formatter(manifest))

	return manifest, nil
}

type ManifestData struct {
	Name       string `yaml:"name"`
	Strategies map[string]map[interface{}]interface{}
}

type Manifest struct {
	Logger
	ConfigGetter
	Data ManifestData
}

func (m *Manifest) LoadStrategy(utility NameVer) (Strategy, error) {

	var strat Strategy

	// default
	priority := "docker,binary"

	if configPriority, err := m.Get("strategy.priority"); err == nil && len(configPriority) > 0 {
		priority = configPriority
	}

	m.Debugf("Priority order: %s", priority)

	var selectedStrategy string
	var foundStrategy map[interface{}]interface{}
	for _, try := range strings.Split(priority, ",") {
		try = strings.TrimSpace(try)
		if strategy, strategy_ok := m.Data.Strategies[try]; strategy_ok {
			selectedStrategy = try
			foundStrategy = strategy
			break
		}
	}

	if len(selectedStrategy) == 0 {
		return strat, fmt.Errorf("No strategy found, tried %s.", priority)
	}

	var selectedVersion map[interface{}]interface{}
	versions := foundStrategy["versions"].([]interface{})

	if len(utility.Version) > 0 {
		found := false
		for _, verInfo := range versions {
			if verInfo.(map[interface{}]interface{})["version"] == utility.Version {
				selectedVersion = verInfo.(map[interface{}]interface{})
				found = true
			}
		}
		if !found {
			return strat, fmt.Errorf("Unable to find version %s", utility.Version)
		}
	} else {
		selectedVersion = versions[0].(map[interface{}]interface{})
	}

	delete(foundStrategy, "versions")
	// fmt.Printf("%v\n", strategy)
	// fmt.Printf("%v\n", versions)
	// fmt.Printf("%v\n", selectedVersion)
	final := mergeMaps(foundStrategy, selectedVersion)
	// fmt.Printf("%v\n", final)

	// handle common keys
	orig_arch_map, arch_map_ok := final["arch_map"]

	arch_map := make(map[string]string)
	if arch_map_ok {
		for k, v := range orig_arch_map.(map[interface{}]interface{}) {
			arch_map[k.(string)] = v.(string)
		}
	}

	conf, err := NewDefaultConfigClient()
	if err != nil {
		return strat, err
	}

	runner := &DefaultRunner{m.Logger}
	commonUtility := &StrategyCommon{
		System:       &DefaultSystem{},
		Logger:       m.Logger,
		ConfigGetter: conf,
		Downloader:   &DefaultDownloader{m.Logger, runner},
		Runner:       runner,
	}

	// handle strategy specific keys
	if selectedStrategy == "docker" {
		mount_pwd, mount_pwd_ok := final["mount_pwd"]
		docker_conn, docker_conn_ok := final["docker_conn"]
		interactive, interactive_ok := final["interactive"]
		image, image_ok := final["image"]

		if !image_ok {
			return strat, errors.New("At least 'image' needed for docker strategy to work")
		}

		strat = DockerStrategy{
			StrategyCommon: commonUtility,
			Data: DockerData{
				Version:     final["version"].(string),
				Image:       image.(string),
				MountPwd:    mount_pwd_ok && mount_pwd.(bool),
				DockerConn:  docker_conn_ok && docker_conn.(bool),
				Interactive: !interactive_ok || interactive.(bool),
				ArchMap:     arch_map,
			},
		}
	} else if selectedStrategy == "binary" {
		base_url, base_url_ok := final["base_url"]

		if !base_url_ok {
			return strat, errors.New("At least 'base_url' needed for binary strategy to work")
		}

		strat = BinaryStrategy{
			StrategyCommon: commonUtility,
			Data: BinaryData{
				Version: final["version"].(string),
				BaseUrl: base_url.(string),
				ArchMap: arch_map,
			},
		}
	}

	m.Debugf("using strategy: %# v", pretty.Formatter(strat))

	return strat, nil
}

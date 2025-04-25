/*
 * Copyright 2021-2025 Hewlett Packard Enterprise Development LP
 * Other additional copyright holders may be indicated within.
 *
 * The entirety of this work is licensed under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 *
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// The legacy overlay reverts 'nnf-deploy init' to pre-helm behavior.
var legacyOverlay string = "overlay-legacy.yaml"

var sysCfgPath string

type System struct {
	Name                string   `yaml:"name"`
	Aliases             []string `yaml:"aliases,flow,omitempty"`
	Overlays            []string `yaml:"overlays,omitempty,flow"`
	SystemConfiguration string   `yaml:"systemConfiguration,flow"`
	K8sHost             string   `yaml:"k8sHost,flow,omitempty"`
	K8sPort             string   `yaml:"k8sPort,flow,omitempty"`
}

type SystemConfigFile struct {
	Systems []System `yaml:"systems"`
}

func FindSystem(name, configPath string) (*System, error) {
	config, err := ReadConfig(configPath)
	if err != nil {
		return nil, err
	}

	for _, system := range config.Systems {
		if system.Name == name {
			return &system, nil
		}
		for _, alias := range system.Aliases {
			if alias == name {
				return &system, nil
			}
		}
	}

	return nil, fmt.Errorf("System '%s' Not Found", name)
}

func ReadConfig(path string) (*SystemConfigFile, error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read system config: %v", err)
	}

	config := new(SystemConfigFile)
	if err := yaml.Unmarshal(configFile, config); err != nil {
		return nil, fmt.Errorf("invalid system config yaml: %v", err)
	}

	sysCfgPath = path

	if err := config.Verify(); err != nil {
		return nil, fmt.Errorf("invalid system config: %v", err)
	}

	return config, nil
}

func (config *SystemConfigFile) Verify() error {
	knownNames := make(map[string]bool)
	knownAlias := make(map[string]bool)

	for _, system := range config.Systems {

		// Make sure system names only appear once
		if _, found := knownNames[system.Name]; found {
			return fmt.Errorf("system name '%s' declared more than once in '%s'", system.Name, sysCfgPath)
		}
		knownNames[system.Name] = true

		// Make sure alias only appear once
		for _, alias := range system.Aliases {
			if _, found := knownAlias[alias]; found {
				return fmt.Errorf("alias '%s' declared more than once in '%s'", alias, sysCfgPath)
			}
			knownAlias[alias] = true
		}

		// Verify the individual components in the system (e.g. rabbits, computes)
		if err := system.Verify(); err != nil {
			return err
		}
	}

	return nil
}

func (system *System) Verify() error {
	knownAliases := make(map[string]bool)
	knownOverlays := make(map[string]bool)

	// Aliases
	for _, alias := range system.Aliases {
		if _, found := knownAliases[alias]; found {
			return fmt.Errorf("alias '%s' declared more than once for system '%s' in '%s'", alias, system.Name, sysCfgPath)
		}
		knownAliases[alias] = true
	}

	// Overlays
	if len(system.Overlays) < 1 {
		return fmt.Errorf("no overlays declared for system '%s' in '%s'", system.Name, sysCfgPath)
	}
	for _, overlay := range system.Overlays {
		if _, found := knownOverlays[overlay]; found {
			return fmt.Errorf("overlay'%s' declared more than once for system '%s' in '%s'", overlay, system.Name, sysCfgPath)
		}
		knownOverlays[overlay] = true
	}

	return nil
}

type ComputesList []string
type Rabbits map[string]ComputesList
type SystemConfigurationCRType map[string]interface{}

func ReadSystemConfigurationCR(crPath string) (SystemConfigurationCRType, error) {
	data := make(SystemConfigurationCRType)

	sysConfigYaml, err := os.ReadFile(crPath)
	if err != nil {
		return nil, fmt.Errorf("could not read SystemConfiguration CR file: %v", err)
	}
	err = yaml.Unmarshal([]byte(sysConfigYaml), &data)
	return data, err
}

func (data SystemConfigurationCRType) RabbitsAndComputes() Rabbits {
	perRabbit := make(Rabbits)

	storageNodes := data["spec"].(SystemConfigurationCRType)["storageNodes"]
	for _, storageNode := range storageNodes.([]interface{}) {
		rabbit := storageNode.(SystemConfigurationCRType)["name"]
		access := storageNode.(SystemConfigurationCRType)["computesAccess"]

		var computes ComputesList
		if access != nil {
			for _, compute := range access.([]interface{}) {
				cname := compute.(SystemConfigurationCRType)["name"]
				computes = append(computes, cname.(string))
			}
		}
		perRabbit[rabbit.(string)] = computes
	}
	return perRabbit
}

func (data SystemConfigurationCRType) ExternalComputes() ComputesList {
	var computes ComputesList

	computeNodes := data["spec"].(SystemConfigurationCRType)["externalComputeNodes"]
	if computeNodes != nil {
		for _, compute := range computeNodes.([]interface{}) {
			cname := compute.(SystemConfigurationCRType)["name"]
			computes = append(computes, cname.(string))
		}
	}

	return computes
}

type RepositoryConfigFile struct {
	Repositories       []Repository        `yaml:"repositories"`
	BuildConfig        BuildConfiguration  `yaml:"buildConfiguration"`
	ThirdPartyServices []ThirdPartyService `yaml:"thirdPartyServices"`
}

type Repository struct {
	Name            string   `yaml:"name"`
	Overlays        []string `yaml:"overlays,flow"`
	Development     string   `yaml:"development"`
	Master          string   `yaml:"master"`
	UseRemoteK      bool     `yaml:"useRemoteK,omitempty"`
	RemoteReference struct {
		Build string `yaml:"build"`
		Url   string `yaml:"url"`
	} `yaml:"remoteReference,omitempty"`
}

type BuildConfiguration struct {
	Env []struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	} `yaml:"env"`
}

type ThirdPartyService struct {
	Name          string `yaml:"name"`
	UseRemoteF    bool   `yaml:"useRemoteF,omitempty"`
	UseRemoteFTar bool   `yaml:"useRemoteFTar,omitempty"`
	UseRemoteKTar bool   `yaml:"useRemoteKTar,omitempty"`
	Kustomization string `yaml:"kustomization,omitempty"`
	Url           string `yaml:"url"`
	WaitCmd       string `yaml:"waitCmd,omitempty"`
	UseHelm       bool   `yaml:"useHelm,omitempty"`
	HelmCmd       string `yaml:"helmCmd,omitempty"`
}

func readConfigFile(configPath string) (*RepositoryConfigFile, error) {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		configFile, err = os.ReadFile(filepath.Join("..", configPath))
		if err != nil {
			return nil, err
		}
	}
	config := new(RepositoryConfigFile)
	if err := yaml.Unmarshal(configFile, config); err != nil {
		return nil, err
	}
	return config, nil
}

func FindRepository(configPath string, module string) (*Repository, *BuildConfiguration, error) {

	config, err := readConfigFile(configPath)
	if err != nil {
		return nil, nil, err
	}

	mergeOverlay := true
	configOverlay, err := readConfigFile(legacyOverlay)
	if err != nil {
		if os.IsNotExist(err) {
			mergeOverlay = false
		} else {
			return nil, nil, err
		}
	}
	if mergeOverlay {
		for _, svc := range configOverlay.Repositories {
			for idx := range config.Repositories {
				if config.Repositories[idx].Name == svc.Name {
					config.Repositories[idx].UseRemoteK = svc.UseRemoteK
				}
			}
		}
	}

	for _, repository := range config.Repositories {
		if module == repository.Name {
			return &repository, &config.BuildConfig, nil
		}
	}

	return nil, nil, fmt.Errorf("Repository '%s' Not Found", module)
}

func GetThirdPartyServices(configPath string) ([]ThirdPartyService, error) {
	config, err := readConfigFile(configPath)
	if err != nil {
		return nil, err
	}
	mergeOverlay := true
	configOverlay, err := readConfigFile(legacyOverlay)
	if err != nil {
		if os.IsNotExist(err) {
			mergeOverlay = false
		} else {
			return nil, err
		}
	}
	if mergeOverlay {
		for _, svc := range configOverlay.ThirdPartyServices {
			for idx := range config.ThirdPartyServices {
				if config.ThirdPartyServices[idx].Name == svc.Name {
					config.ThirdPartyServices[idx].UseRemoteF = svc.UseRemoteF
					config.ThirdPartyServices[idx].UseRemoteFTar = svc.UseRemoteFTar
					config.ThirdPartyServices[idx].UseRemoteKTar = svc.UseRemoteKTar
					config.ThirdPartyServices[idx].UseHelm = svc.UseHelm
					break
				}
			}
		}
	}
	return config.ThirdPartyServices, nil
}

type Daemon struct {
	Name           string `yaml:"name"`
	Bin            string `yaml:"bin"`
	BuildCmd       string `yaml:"buildCmd"`
	Repository     string `yaml:"repository"`
	Path           string `yaml:"path"`
	ServiceAccount struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"serviceAccount,omitempty"`
	ExtraArgs   string   `yaml:"extraArgs,omitempty"`
	Environment []EnvVar `yaml:"env,omitempty"`
}

type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value,omitempty"`
}

type DaemonConfigFile struct {
	Daemons []Daemon `yaml:"daemons"`
}

func EnumerateDaemons(configPath string, handleFn func(Daemon) error) error {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	config := new(DaemonConfigFile)
	if err := yaml.Unmarshal(configFile, config); err != nil {
		return err
	}

	for _, daemon := range config.Daemons {
		if err := handleFn(daemon); err != nil {
			return err
		}
	}

	return nil
}

type Library struct {
	Name       string `yaml:"name"`
	BuildCmd   string `yaml:"buildCmd"`
	Repository string `yaml:"repository"`
	Library    LibEnt `yaml:"lib,omitempty"`
	// Path is the path within the build repository that has the library and its header.
	Path   string `yaml:"path"`
	Secret struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"secret,omitempty"`
}

type LibEnt struct {
	// Name is the name of a library's .a or .so.
	Name string `yaml:"name"`
	// Dest is where Name should be copied to on the destination host.
	Dest string `yaml:"dest"`
}

type HeaderEnt struct {
	// Name is the name of a .h file.
	Name string `yaml:"name"`
	// Dest is where Name should be copied to on the destination host.
	Dest string `yaml:"dest"`
}

type LibraryConfigFile struct {
	Libraries []Library `yaml:"libraries"`
}

func EnumerateLibraries(configPath string, handleFn func(Library) error) error {
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	config := new(LibraryConfigFile)
	if err := yaml.Unmarshal(configFile, config); err != nil {
		return err
	}

	for _, library := range config.Libraries {
		if err := handleFn(library); err != nil {
			return err
		}
	}

	return nil
}

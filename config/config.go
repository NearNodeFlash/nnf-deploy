/*
 * Copyright 2021, 2022 Hewlett Packard Enterprise Development LP
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

	"gopkg.in/yaml.v2"
)

type System struct {
	Name      string                    `yaml:"name"`
	Aliases   []string                  `yaml:"aliases,flow,omitempty"`
	Overlays  []string                  `yaml:"overlays,omitempty,flow"`
	Master    string                    `yaml:"master"`
	Workers   []string                  `yaml:"workers,flow,omitempty"`
	Rabbits   map[string]map[int]string `yaml:"rabbits,flow"`
	Overrides map[string][]string       `yaml:"overrides,omitempty"`
}

type SystemConfigFile struct {
	Systems []System `yaml:"systems"`
}

func FindSystem(name string) (*System, error) {
	configFile, err := os.ReadFile("config/systems.yaml")
	if err != nil {
		return nil, err
	}

	config := new(SystemConfigFile)
	if err := yaml.Unmarshal(configFile, config); err != nil {
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

type RepositoryConfigFile struct {
	Repositories []Repository `yaml:"repositories"`
}

type Repository struct {
	Name        string
	Overlays    []string `yaml:",flow"`
	Development string
	Master      string
}

func FindRepository(module string) (*Repository, error) {

	configFile, err := os.ReadFile("config/repositories.yaml")
	if err != nil {
		configFile, err = os.ReadFile("../config/repositories.yaml")
		if err != nil {
			return nil, err
		}
	}

	config := new(RepositoryConfigFile)
	if err := yaml.Unmarshal(configFile, config); err != nil {
		return nil, err
	}

	for _, repository := range config.Repositories {
		if module == repository.Name {
			return &repository, nil
		}
	}

	return nil, fmt.Errorf("Repository '%s' Not Found", module)
}

type Daemon struct {
	Name            string `yaml:"name"`
	Bin             string `yaml:"bin"`
	Repository      string `yaml:"repository"`
	Path            string `yaml:"path"`
	SkipNnfNodeName bool   `yaml:"skipNnfNodeName"`
	ServiceAccount  struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"serviceAccount,omitempty"`
}

type DaemonConfigFile struct {
	Daemons []Daemon `yaml:"daemons"`
}

func EnumerateDaemons(handleFn func(Daemon) error) error {
	configFile, err := os.ReadFile("config/daemons.yaml")
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

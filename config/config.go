package config

import (
	_ "embed"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type System struct {
	Name    string              `yaml:"name"`
	Aliases []string            `yaml:"aliases,flow,omitempty"`
	Overlay string              `yaml:"overlay,omitempty"`
	Master  string              `yaml:"master"`
	Workers []string            `yaml:"workers,flow,omitempty"`
	Rabbits map[string][]string `yaml:"rabbits,flow"`
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

type ArtifactoryConfigFile struct {
	Repositories []Repository `yaml:"repositories"`
}

type Repository struct {
	Name        string
	Development string
	Master      string
}

func FindRepository(module string) (*Repository, error) {

	configFile, err := os.ReadFile("config/artifactory.yaml")
	if err != nil {
		return nil, err
	}

	config := new(ArtifactoryConfigFile)
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

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Saves          string `yaml:"saves"`
	SavesAutoStrip bool   `yaml:"saves_autostrip"`
	Data           string `yaml:"data"`
	Images         string `yaml:"images"`
}

var GlobalConfig *Config
var Verbose bool

func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	GlobalConfig = cfg
	return nil
}

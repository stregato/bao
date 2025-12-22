package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

func LoadTestConfig(tb testing.TB, label string) StoreConfig {
	if tb != nil {
		tb.Helper()
	}

	configFile := getConfigFilePath()
	data, err := os.ReadFile(configFile)
	if err != nil {
		fail(tb, "failed to read store configs from %s: %v", configFile, err)
	}

	var configs []StoreConfig
	if err := yaml.Unmarshal(data, &configs); err != nil {
		fail(tb, "failed to parse store configs in %s: %v", configFile, err)
	}

	for _, c := range configs {
		if c.Id == label {
			return c
		}
	}

	fail(tb, "store config with id %q not found in %s", label, configFile)
	return StoreConfig{}
}

func getConfigFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("cannot get user home dir: %v", err))
	}
	return filepath.Join(homeDir, ".config", "baoConfig.yaml")
}

func fail(tb testing.TB, format string, args ...interface{}) {
	if tb != nil {
		tb.Fatalf(format, args...)
	}
	panic(fmt.Sprintf(format, args...))
}

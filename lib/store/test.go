package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

func LoadTestConfig(tb testing.TB, label string) StoreConfig {
	allTestConfigs := LoadAllTestConfigs(tb)
	config, ok := allTestConfigs[label]
	if !ok {
		fail(tb, "no store config found with label '%s'", label)
	}
	return config
}

func LoadTestStore(tb testing.TB, label string) Store {
	if tb != nil {
		tb.Helper()
	}
	config := LoadTestConfig(tb, label)
	store, err := Open(config)
	if err != nil {
		fail(tb, "failed to open test store with label '%s': %v", label, err)
	}
	return store
}

func LoadAllTestConfigs(tb testing.TB) map[string]StoreConfig {
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

	configMap := make(map[string]StoreConfig)
	for _, c := range configs {
		configMap[c.Id] = c
	}
	return configMap
}

func getConfigFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("cannot get user home dir: %v", err))
	}
	homeConfig := filepath.Join(homeDir, ".config", "baotest.yaml")
	if _, err := os.Stat(homeConfig); err == nil {
		return homeConfig
	} else if !os.IsNotExist(err) {
		return homeConfig
	}

	cwd, err := os.Getwd()
	if err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			candidate := filepath.Join(dir, "baotest.yaml")
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
			if parent := filepath.Dir(dir); parent == dir {
				break
			}
		}
	}

	return homeConfig
}

func fail(tb testing.TB, format string, args ...interface{}) {
	if tb != nil {
		tb.Fatalf(format, args...)
	}
	panic(fmt.Sprintf(format, args...))
}

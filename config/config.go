package config

import (
	"embed"
	"encoding/json"

	"github.com/pablor21/gonnotation/logger"
	"gopkg.in/yaml.v3"
)

//go:embed config.yml
var defaultConfigFile embed.FS

type Config struct {
	Scanning ScanningConfig   `json:"scanning" yaml:"scanning"`
	LogLevel *logger.LogLevel `json:"logLevel" yaml:"logLevel"`
}

// ScanMode defines what gets scanned
type ScanMode string

const (
	ScanModeAll        ScanMode = "all"        // Scan all items
	ScanModeReferenced ScanMode = "referenced" // Scan only referenced items
	ScanModeNone       ScanMode = "none"       // Don't scan
	ScanModeDisabled   ScanMode = "disabled"   // Don't scan (alias for none)
)

// type OutOfScopeAction string

// const (
// 	OutOfScopeActionIgnore OutOfScopeAction = "ignore" // Ignore out-of-scope items
// 	OutOfScopeFail         OutOfScopeAction = "fail"   // Return an error for out-of-scope items
// 	OutOfScopeActionWarn   OutOfScopeAction = "warn"   // Log a warning for out-of-scope items
// )

type ScanningConfig struct {
	Packages    []string    `json:"packages" yaml:"packages"`
	ScanOptions ScanOptions `json:"scan_options" yaml:"scan_options"`
	// OutOfScopeAction OutOfScopeAction `json:"out_of_scope_action" yaml:"out_of_scope_action"`
}

type ScanOptions struct {
	Structs       ScanMode `json:"structs" yaml:"structs"`
	StructMethods ScanMode `json:"struct_methods" yaml:"struct_methods"`
	Interfaces    ScanMode `json:"interfaces" yaml:"interfaces"`
	Functions     ScanMode `json:"functions" yaml:"functions"`
	Enums         ScanMode `json:"enums" yaml:"enums"`
}

func NewDefaultConfig() *Config {
	// parse default config from embedded file
	config, err := LoadConfigFromFS(defaultConfigFile, "config.yml")
	if err != nil {
		panic("failed to load default config: " + err.Error())
	}
	return config
}

func LoadConfigFromFS(fs embed.FS, path string) (*Config, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadConfigFromYAML(data)
}

func LoadConfigFromYAML(data []byte) (*Config, error) {
	var config Config
	err := yamlUnmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func LoadConfigFromJSON(data []byte) (*Config, error) {
	var config Config
	err := jsonUnmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
func yamlUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

package parser

// GenStrategy determines output file strategy
type GenStrategy string

const (
	GenStrategySingle    GenStrategy = "single"    // One file for all types
	GenStrategyMultiple  GenStrategy = "multiple"  // Separate file per type
	GenStrategyPackage   GenStrategy = "package"   // Separate file per package
	GenStrategyFollow    GenStrategy = "follow"    // One schema per Go source file
	GenStrategyNamespace GenStrategy = "namespace" // Separate file per namespace (format-specific)
)

// Config is the main configuration structure
// Core settings are at root level, format generator-specific settings are under generators
type Config struct {
	// Inline core configuration (at root level)
	CoreConfig `yaml:",inline"`

	// Extends allows inheriting from another config file
	// Can be a single file path or an array of file paths
	// Paths are relative to the current config file's directory
	Extends any `yaml:"extends,omitempty"`

	// Generate to generate (e.g., ["graphql", "openapi"])
	Generate []string `yaml:"generate"`

	// Plugin-specific configurations (map of plugin name to raw YAML)
	Plugins map[string]any `yaml:"plugins,omitempty"`
}

func NewConfigWithDefaults() *Config {
	coreConfig := NewCoreConfig()
	return &Config{
		CoreConfig: *coreConfig,
		Generate:   []string{"graphql"},
		Plugins:    make(map[string]any),
	}
}

// MergeConfigs merges two configs, with override taking precedence
func MergeConfigs(base, override *Config) *Config {
	// Don't start with defaults - start empty
	result := &Config{
		CoreConfig: CoreConfig{
			CommonConfig: CommonConfig{},
		},
		Plugins: make(map[string]any),
	}

	// Start with base
	if base != nil {
		result.CoreConfig = base.CoreConfig
		result.Generate = append([]string{}, base.Generate...)
		result.Plugins = DeepCopyGenerators(base.Plugins)
	}

	// Override with override config
	if override != nil {
		// Merge core config pointer fields (only override if set in override)
		result.CoreConfig = MergeCoreConfig(result.CoreConfig, override.CoreConfig)

		// Override specs if set (and different from default)
		// if len(override.Generate) > 0 && !isDefaultSpecs(override.Generate) {
		// 	result.Generate = append([]string{}, override.Generate...)
		// }

		// Merge generators (deep merge maps)
		if override.Plugins != nil {
			if result.Plugins == nil {
				result.Plugins = make(map[string]any)
			}
			for key, value := range override.Plugins {
				// Deep merge generator configs
				if existingValue, exists := result.Plugins[key]; exists {
					if existingMap, ok := existingValue.(map[string]any); ok {
						if valueMap, ok := value.(map[string]any); ok {
							result.Plugins[key] = DeepMergeMap(existingMap, valueMap)
							continue
						}
					}
				}
				result.Plugins[key] = value
			}
		}

		// Preserve override's config dir
		if override.ConfigDir != "" {
			result.ConfigDir = override.ConfigDir
		}
	}

	return result
}

// DeepMergeMap merges two maps, with override taking precedence
func DeepMergeMap(base, override map[string]any) map[string]any {
	result := DeepCopyMap(base)
	for k, v := range override {
		if existingValue, exists := result[k]; exists {
			if existingMap, ok := existingValue.(map[string]any); ok {
				if valueMap, ok := v.(map[string]any); ok {
					result[k] = DeepMergeMap(existingMap, valueMap)
					continue
				}
			}
		}
		result[k] = v
	}
	return result
} // MergeCoreConfig merges CoreConfig, only overriding non-nil pointer fields
func MergeCoreConfig(base, override CoreConfig) CoreConfig {
	result := base

	// Override pointer fields only if they're set in override
	if len(override.Packages) > 0 {
		result.Packages = override.Packages
	}
	if override.AnnotationPrefix != nil {
		result.AnnotationPrefix = override.AnnotationPrefix
	}
	if override.StructTagName != nil {
		result.StructTagName = override.StructTagName
	}
	if override.EnableGenerics != nil {
		result.EnableGenerics = override.EnableGenerics
	}
	if override.LogLevel != nil {
		result.LogLevel = override.LogLevel
	}
	if override.AutoGenerate != nil {
		result.AutoGenerate = override.AutoGenerate
	}
	if override.Validator != nil {
		result.Validator = override.Validator
	}
	if override.ScanOptions != nil {
		result.ScanOptions = override.ScanOptions
	}

	return result
}

// DeepCopyGenerators creates a deep copy of the generators map
func DeepCopyGenerators(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	result := make(map[string]any)
	for k, v := range src {
		// For map values, we'll do a shallow copy
		// YAML unmarshaling ensures these are map[string]any trees
		if m, ok := v.(map[string]any); ok {
			result[k] = DeepCopyMap(m)
		} else {
			result[k] = v
		}
	}
	return result
}

// DeepCopyMap creates a deep copy of a map[string]any
func DeepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	result := make(map[string]any)
	for k, v := range src {
		switch val := v.(type) {
		case map[string]any:
			result[k] = DeepCopyMap(val)
		case []any:
			newSlice := make([]any, len(val))
			copy(newSlice, val)
			result[k] = newSlice
		default:
			result[k] = v
		}
	}
	return result
}

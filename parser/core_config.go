package parser

// LogLevel defines the logging verbosity
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelNone  LogLevel = "none"
)

// ScanMode defines what gets scanned
type ScanMode string

const (
	ScanModeAll        ScanMode = "all"        // Scan all items
	ScanModeReferenced ScanMode = "referenced" // Scan only referenced items
	ScanModeNone       ScanMode = "none"       // Don't scan
	ScanModeDisabled   ScanMode = "disabled"   // Don't scan (alias for none)
)

// CLIConfig holds CLI-specific configuration
type CLIConfig struct {
	Watcher *WatcherConfig `yaml:"watcher,omitempty"`
}

// WatcherConfig holds file watcher configuration
type WatcherConfig struct {
	Enabled         bool     `yaml:"enabled,omitempty"`
	DebounceMs      int      `yaml:"debounce_ms,omitempty"`
	AdditionalPaths []string `yaml:"additional_paths,omitempty"`
	IgnorePatterns  []string `yaml:"ignore_patterns,omitempty"`
}

// ScanOptions controls what gets scanned from source files
type ScanOptions struct {
	Enums          ScanMode `yaml:"enums"`
	Structs        ScanMode `yaml:"structs"`
	Interfaces     ScanMode `yaml:"interfaces"`
	Functions      ScanMode `yaml:"functions"`
	TypeFunctions  ScanMode `yaml:"type_functions"`
	FunctionBodies ScanMode `yaml:"function_bodies"`
}

// IsEnabled returns true if the scan mode is not none or disabled
func (s ScanMode) IsEnabled() bool {
	return s != ScanModeNone && s != ScanModeDisabled && s != ""
}

// IsAll returns true if the scan mode is all
func (s ScanMode) IsAll() bool {
	return s == ScanModeAll
}

// IsReferenced returns true if the scan mode is referenced
func (s ScanMode) IsReferenced() bool {
	return s == ScanModeReferenced
}

// Merge creates a new ScanOptions by merging this with override values
// Non-empty values in override take precedence
func (s *ScanOptions) Merge(override *ScanOptions) *ScanOptions {
	if s == nil && override == nil {
		return &ScanOptions{
			Enums:          ScanModeAll,
			Structs:        ScanModeAll,
			Functions:      ScanModeAll,
			Interfaces:     ScanModeAll,
			TypeFunctions:  ScanModeAll,
			FunctionBodies: ScanModeAll,
		}
	}
	if s == nil {
		return override
	}
	if override == nil {
		return s
	}

	merged := &ScanOptions{
		Enums:          s.Enums,
		Structs:        s.Structs,
		Interfaces:     s.Interfaces,
		Functions:      s.Functions,
		TypeFunctions:  s.TypeFunctions,
		FunctionBodies: s.FunctionBodies,
	}

	// Override with format generator-specific values if set
	if override.Enums != "" {
		merged.Enums = override.Enums
	}
	if override.Structs != "" {
		merged.Structs = override.Structs
	}
	if override.Interfaces != "" {
		merged.Interfaces = override.Interfaces
	}
	if override.Functions != "" {
		merged.Functions = override.Functions
	}
	if override.TypeFunctions != "" {
		merged.TypeFunctions = override.TypeFunctions
	}

	return merged
}

// CommonConfig holds configuration that can be overridden at the plugin level
// All fields are pointers to allow detecting when they're explicitly set vs using defaults
type CommonConfig struct {
	// Plugin-level overrides for core config
	Packages           []string     `yaml:"packages,omitempty"`
	AnnotationPrefix   *string      `yaml:"annotation_prefix,omitempty"`
	AnnotationPrefixes []string     `yaml:"annotation_prefixes,omitempty"`
	StructTagName      *string      `yaml:"struct_tag_name,omitempty"`
	EnableGenerics     *bool        `yaml:"enable_generics,omitempty"`
	LogLevel           *LogLevel    `yaml:"log_level,omitempty"`
	ScanOptions        *ScanOptions `yaml:"scan_options,omitempty"`

	// Auto-generation configuration (shared across plugins)
	AutoGenerate *AutoGenerateConfig `yaml:"auto_generate,omitempty"`

	// CLI configuration
	CLI *CLIConfig `yaml:"cli,omitempty"`

	// Validation configuration for annotations and schema
	Validator *ValidationConfig `yaml:"validator,omitempty"`
}

// CoreConfig holds format-agnostic configuration
type CoreConfig struct {
	// Embed common configuration (with pointer fields)
	CommonConfig `yaml:",inline"`

	// Internal (not serialized)
	ConfigDir string `yaml:"-"`
}

func NewCoreConfig() *CoreConfig {
	return &CoreConfig{
		CommonConfig: CommonConfig{
			Packages:         []string{},
			AnnotationPrefix: Ptr("@"),
			StructTagName:    Ptr(""),
			EnableGenerics:   Ptr(true),
			LogLevel:         Ptr(LogLevelInfo),

			// Scanning options
			ScanOptions: &ScanOptions{
				Enums:         ScanModeAll,
				Structs:       ScanModeAll,
				Functions:     ScanModeNone,
				Interfaces:    ScanModeNone,
				TypeFunctions: ScanModeNone,
			},
			AutoGenerate: NewAutoGenerateConfig(),

			// CLI configuration
			CLI: &CLIConfig{
				Watcher: &WatcherConfig{
					Enabled:         false,
					DebounceMs:      500,
					AdditionalPaths: []string{},
					IgnorePatterns:  []string{"vendor", "node_modules", ".git"},
				},
			},
		},
	}
}

func (c *CoreConfig) Normalize() {
	if c.AnnotationPrefix == nil {
		c.AnnotationPrefix = Ptr("@")
	}
	if c.LogLevel == nil {
		c.LogLevel = Ptr(LogLevelInfo)
	}
	if c.ScanOptions == nil {
		c.ScanOptions = &ScanOptions{
			Enums:         ScanModeAll,
			Structs:       ScanModeAll,
			Functions:     ScanModeAll,
			Interfaces:    ScanModeAll,
			TypeFunctions: ScanModeNone,
		}
	}
}

package parser

// AutoGenerateStrategy defines the strategy for auto-generating schemas
type AutoGenerateStrategy string

const (
	AutoGenNone       AutoGenerateStrategy = "none"       // Only generate annotated schemas
	AutoGenReferenced AutoGenerateStrategy = "referenced" // Generate schemas referenced by annotated schemas
	AutoGenAll        AutoGenerateStrategy = "all"        // Generate all schemas found
	AutoGenPatterns   AutoGenerateStrategy = "patterns"   // Generate based on patterns only
)

// OutOfScopeAction defines how to handle types that are out of scope
type OutOfScopeAction string

const (
	OutOfScopeWarn    OutOfScopeAction = "warn"    // Warn but continue
	OutOfScopeFail    OutOfScopeAction = "fail"    // Fail immediately
	OutOfScopeIgnore  OutOfScopeAction = "ignore"  // Silently ignore
	OutOfScopeExclude OutOfScopeAction = "exclude" // Exclude from generation
)

// AutoGenerateConfig controls automatic schema generation
// This is shared across all plugins and can be overridden at plugin level
type AutoGenerateConfig struct {
	// Enable auto-generation
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Strategy: "none", "referenced", "all", "patterns"
	Strategy AutoGenerateStrategy `yaml:"strategy" json:"strategy"`

	// Maximum depth for transitive type references (default: 1)
	// 0 = unlimited, 1 = direct references only, 2 = references of references, etc.
	MaxDepth int `yaml:"max_depth" json:"max_depth"`

	// Patterns to include (glob-style patterns matching package/type)
	// Example: "*/models/*", "*/dto/*"
	Patterns []string `yaml:"patterns,omitempty" json:"patterns,omitempty"`

	// Patterns to exclude (higher priority than include patterns)
	// Example: "*/internal/*", "*/vendor/*"
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty" json:"exclude_patterns,omitempty"`

	// Only auto-generate types that are referenced by annotated types
	OnlyReferencedByAnnotated bool `yaml:"only_referenced_by_annotated" json:"only_referenced_by_annotated"`

	// Auto-generate embedded struct types
	IncludeEmbedded bool `yaml:"include_embedded" json:"include_embedded"`

	// Auto-generate field types
	IncludeFieldTypes bool `yaml:"include_field_types" json:"include_field_types"`

	// Action for out-of-scope types: "warn", "fail", "ignore", "exclude"
	OutOfScopeAction string `yaml:"out_of_scope_action" json:"out_of_scope_action"`

	// Type to use for unresolved generic type parameters
	UnresolvedGenericType string `yaml:"unresolved_generic_type" json:"unresolved_generic_type"`

	// Suppress out-of-scope warnings for common type parameters (T, K, V, etc.)
	SuppressGenericTypeWarnings bool `yaml:"suppress_generic_type_warnings" json:"suppress_generic_type_warnings"`
}

// NewAutoGenerateConfig creates a new config with default values
func NewAutoGenerateConfig() *AutoGenerateConfig {
	return &AutoGenerateConfig{
		Enabled:                     true,
		Strategy:                    AutoGenReferenced,
		MaxDepth:                    1,
		Patterns:                    []string{},
		ExcludePatterns:             []string{"*/vendor/*", "*/*_test.go"},
		OnlyReferencedByAnnotated:   true,
		IncludeEmbedded:             true,
		IncludeFieldTypes:           true,
		OutOfScopeAction:            "warn",
		UnresolvedGenericType:       "",
		SuppressGenericTypeWarnings: false,
	}
}

// Merge merges plugin-level config with root-level config
// Plugin-level settings override root-level settings
func (c *AutoGenerateConfig) Merge(override *AutoGenerateConfig) *AutoGenerateConfig {
	if override == nil {
		return c
	}

	merged := *c // Copy root config

	// Override with plugin-level settings if provided
	// Note: we need to check if values were explicitly set vs defaults
	// For now, we'll do simple override (any non-zero value overrides)

	if override.Strategy != "" {
		merged.Strategy = override.Strategy
	}

	if override.MaxDepth != 0 || (override.MaxDepth == 0 && override.Strategy == AutoGenAll) {
		merged.MaxDepth = override.MaxDepth
	}

	if len(override.Patterns) > 0 {
		merged.Patterns = override.Patterns
	}

	if len(override.ExcludePatterns) > 0 {
		merged.ExcludePatterns = override.ExcludePatterns
	}

	// Boolean fields - tricky because we can't tell if false is explicit or default
	// For now, we'll just override
	merged.Enabled = override.Enabled
	merged.OnlyReferencedByAnnotated = override.OnlyReferencedByAnnotated
	merged.IncludeEmbedded = override.IncludeEmbedded
	merged.IncludeFieldTypes = override.IncludeFieldTypes
	merged.SuppressGenericTypeWarnings = override.SuppressGenericTypeWarnings

	if override.OutOfScopeAction != "" {
		merged.OutOfScopeAction = override.OutOfScopeAction
	}

	if override.UnresolvedGenericType != "" {
		merged.UnresolvedGenericType = override.UnresolvedGenericType
	}

	return &merged
}

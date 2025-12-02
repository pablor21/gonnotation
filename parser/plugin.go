package parser

import (
	"github.com/pablor21/gonnotation/annotations"
)

// GeneratedFile represents a single generated output file
type GeneratedFile struct {
	// Path is the relative path where the file should be written
	// For single strategy: just the filename
	// For multiple/package/follow: relative path including directories
	Path string

	// Content is the file content
	Content []byte

	// Metadata holds additional information about the file
	Metadata map[string]any
}

// GeneratedOutput represents the output from a plugin
type GeneratedOutput struct {
	// Files contains all generated files
	Files []*GeneratedFile

	// IsSingleFile indicates if this is a single-file output (for backward compatibility)
	IsSingleFile bool
}

// Plugin defines the interface for schema generators
type Plugin interface {
	// Name returns plugin identifier
	Name() string

	// Specs returns supported spec types (e.g., ["graphql"], ["openapi"])
	Specs() []string

	// Definitions returns supported annotations and struct tags for the plugin
	Definitions() annotations.PluginDefinitions

	// AcceptsAnnotation checks if format generator recognizes an annotation
	AcceptsAnnotation(name string) bool

	// Generate produces schema from parsed data (deprecated: use GenerateMulti for multi-file support)
	Generate(ctx *GenerationContext) ([]byte, error)

	// GenerateMulti produces multiple schema files from parsed data
	// Returns GeneratedOutput with one or more files depending on generation strategy
	GenerateMulti(ctx *GenerationContext) (*GeneratedOutput, error)

	// ValidateConfig validates format generator configuration
	ValidateConfig(config any) error
}

// PluginRegistry manages registered plugins
type PluginRegistry struct {
	plugins      map[string]Plugin
	specToPlugin map[string]Plugin
}

// NewPluginRegistry creates a registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins:      make(map[string]Plugin),
		specToPlugin: make(map[string]Plugin),
	}
}

// RegisterPlugin adds a plugin
func (r *PluginRegistry) RegisterPlugin(plugin Plugin) {
	r.plugins[plugin.Name()] = plugin
	for _, spec := range plugin.Specs() {
		r.specToPlugin[spec] = plugin
	}
}

// GetByName retrieves plugin by name
func (r *PluginRegistry) GetByName(name string) (Plugin, bool) {
	plugin, ok := r.plugins[name]
	return plugin, ok
}

// GetBySpec retrieves plugin by spec
func (r *PluginRegistry) GetBySpec(spec string) (Plugin, bool) {
	plugin, ok := r.specToPlugin[spec]
	return plugin, ok
}

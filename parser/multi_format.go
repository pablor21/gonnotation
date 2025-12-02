package parser

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// MultiFormatGenerator coordinates multi-format schema generation
type MultiFormatGenerator struct {
	config       *Config
	orchestrator *Orchestrator
}

func NewMultiFormatGenerator(config *Config) *MultiFormatGenerator {
	return &MultiFormatGenerator{
		config:       config,
		orchestrator: NewOrchestrator(&config.CoreConfig),
	}
}

func (mfg *MultiFormatGenerator) RegisterPlugin(plugin Plugin) {
	mfg.orchestrator.RegisterPlugin(plugin)
}

func (mfg *MultiFormatGenerator) GetOrchestrator() *Orchestrator {
	return mfg.orchestrator
}

func (mfg *MultiFormatGenerator) GetConfig() *Config {
	return mfg.config
}

func (mfg *MultiFormatGenerator) Generate() error {
	for _, spec := range mfg.config.Generate {
		// Get format generator-specific config from the map
		pluginConfig := mfg.config.Plugins[spec]

		// Generate using multi-file strategy
		output, err := mfg.orchestrator.GenerateMulti(spec, pluginConfig)
		if err != nil {
			return fmt.Errorf("generate %s: %w", spec, err)
		}

		// Write all generated files
		if output != nil && len(output.Files) > 0 {
			for _, file := range output.Files {
				// The file path is already resolved by the generator
				outputPath := file.Path

				// If path is not absolute, it's relative to current directory
				if !filepath.IsAbs(outputPath) {
					outputPath = filepath.Clean(outputPath)
				}

				// Ensure directory exists
				dir := filepath.Dir(outputPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("create directory %s: %w", dir, err)
				}

				// Write file
				if err := os.WriteFile(outputPath, file.Content, 0644); err != nil {
					return fmt.Errorf("write file %s: %w", outputPath, err)
				}

				slog.Info(fmt.Sprintf("Generated %s: %s (%d bytes)", spec, outputPath, len(file.Content)))
			}
		}
	}

	return nil
}

// // containsDirective checks if output already includes a directive marker
// func containsDirective(output []byte, marker string) bool {
// 	return len(output) > 0 && string(output) != "" && filepath.Base(marker) == marker && stringIndex(string(output), marker) >= 0
// }

// // stringIndex returns index of substr or -1 (avoid importing strings for minimal patch scope)
// func stringIndex(haystack, needle string) int {
// 	for i := 0; i+len(needle) <= len(haystack); i++ {
// 		if haystack[i:i+len(needle)] == needle {
// 			return i
// 		}
// 	}
// 	return -1
// }

package gonnotation

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablor21/gonnotation/config"
	"github.com/pablor21/gonnotation/logger"
	"github.com/pablor21/gonnotation/types"
	"github.com/pablor21/gonnotation/utils"
	"golang.org/x/tools/go/packages"
)

// Process processes Go packages with default configuration
func Process() (*types.ProcessResult, error) {
	defaultConfig := config.NewDefaultConfig()
	return ProcessWithConfig(defaultConfig)
}

// ProcessWithConfig processes Go packages with the provided configuration
func ProcessWithConfig(config *config.Config) (*types.ProcessResult, error) {
	ctx := &types.ProcessContext{
		Config:     config,
		Logger:     logger.NewDefaultLogger(),
		ModulePath: detectModulePath(),
	}
	return ProcessWithContext(ctx)
}

// ProcessWithContext processes Go packages using the provided context
func ProcessWithContext(ctx *types.ProcessContext) (*types.ProcessResult, error) {
	packages, err := loadPackages(ctx)
	if err != nil {
		return nil, err
	}
	return parsePackages(ctx, packages)
}

// loadPackages loads the packages specified in the configuration
func loadPackages(ctx *types.ProcessContext) ([]*packages.Package, error) {
	patterns := ctx.Config.Scanning.Packages

	packages, err := utils.LoadPackages(patterns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load packages: %w", err)
	}
	return packages, nil
}

// parsePackages parses all loaded packages and returns the result
func parsePackages(ctx *types.ProcessContext, packages []*packages.Package) (*types.ProcessResult, error) {
	res := types.NewParseResult()
	for _, pkg := range packages {
		err := res.ParsePackage(ctx, pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse package %s: %w", pkg.PkgPath, err)
		}
	}

	// Update usage flags for all types after parsing is complete
	for _, typeInfo := range res.Elements {
		types.UpdateUsageFlags(typeInfo)
	}

	// Update include types for all types after parsing is complete
	res.UpdateIncludeTypes()

	// Calculate depths for all types after parsing and usage tracking is complete
	res.CalculateTypeDepths()

	// Update the result with the final processed types (after all modifications)
	// maps.Copy(res.Elements, res.Elements)

	return res, nil
}

// detectModulePath tries to detect the module path from go.mod or working directory
func detectModulePath() string {
	// Try to find go.mod file
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Look for go.mod in current directory and parent directories
	dir := wd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Read go.mod file to extract module path
			if content, err := os.ReadFile(goModPath); err == nil {
				lines := strings.SplitSeq(string(content), "\n")
				for line := range lines {
					line = strings.TrimSpace(line)
					if after, ok := strings.CutPrefix(line, "module "); ok {
						return strings.TrimSpace(after)
					}
				}
			}
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root directory
		}
		dir = parent
	}

	// Fallback: try to infer from GOPATH
	if gopath := build.Default.GOPATH; gopath != "" {
		srcDir := filepath.Join(gopath, "src")
		if rel, err := filepath.Rel(srcDir, wd); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}

	return ""
}

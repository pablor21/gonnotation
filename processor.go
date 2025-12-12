package gonnotation

import (
	"fmt"

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
		Config: config,
		Logger: logger.NewDefaultLogger(),
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
	return res, nil
}

package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pablor21/gonnotation/annotations"
)

// ValidationAction defines how to handle validation errors
type ValidationAction string

const (
	ValidationActionDisabled ValidationAction = "disabled"
	ValidationActionWarn     ValidationAction = "warn"
	ValidationActionFail     ValidationAction = "fail"
)

// ValidationMode defines the strictness of validation
type ValidationMode string

const (
	ValidationModeDisabled ValidationMode = "disabled"
	ValidationModeLax      ValidationMode = "lax"
	ValidationModeStrict   ValidationMode = "strict"
)

// ValidationConfig contains validation configuration
type ValidationConfig struct {
	Action              ValidationAction
	ValidateAnnotations ValidationMode
	ValidateSchema      ValidationMode
}

// ValidationError represents a validation error
type ValidationError struct {
	Location string // Where the error occurred (e.g., "User.Email @schema")
	Message  string // Error message
	Severity string // "error" or "warning"
}

// Validator validates annotations and schemas
type Validator struct {
	config *ValidationConfig
	specs  []annotations.AnnotationSpec
	errors []ValidationError
}

// NewValidator creates a new validator
func NewValidator(config *ValidationConfig, specs []annotations.AnnotationSpec) *Validator {
	if config == nil {
		config = &ValidationConfig{
			Action:              ValidationActionDisabled,
			ValidateAnnotations: ValidationModeDisabled,
			ValidateSchema:      ValidationModeDisabled,
		}
	}
	return &Validator{
		config: config,
		specs:  specs,
		errors: make([]ValidationError, 0),
	}
}

// ValidateAnnotation validates a single annotation
func (v *Validator) ValidateAnnotation(ann annotations.Annotation, location string) {
	if v.config.ValidateAnnotations == ValidationModeDisabled {
		return
	}

	// Find annotation spec
	var annotationSpec *annotations.AnnotationSpec
	for i := range v.specs {
		if strings.EqualFold(v.specs[i].Name, ann.Name) {
			annotationSpec = &v.specs[i]
			break
		}
		// Check aliases
		for _, alias := range v.specs[i].Aliases {
			if strings.EqualFold(alias, ann.Name) {
				annotationSpec = &v.specs[i]
				break
			}
		}
		// Check global aliases
		for _, alias := range v.specs[i].GlobalAliases {
			if strings.EqualFold(alias, ann.Name) {
				annotationSpec = &v.specs[i]
				break
			}
		}
	}

	if annotationSpec == nil {
		// Unknown annotation
		if v.config.ValidateAnnotations == ValidationModeStrict {
			v.addError(location, fmt.Sprintf("unknown annotation '@%s'", ann.Name), "error")
		}
		return
	}

	// Validate parameters
	v.validateAnnotationParams(ann, annotationSpec, location)
}

// validateAnnotationParams validates annotation parameters
func (v *Validator) validateAnnotationParams(ann annotations.Annotation, spec *annotations.AnnotationSpec, location string) {
	// Check for unknown parameters in strict mode
	if v.config.ValidateAnnotations == ValidationModeStrict {
		for paramName := range ann.Params {
			if paramName == "" {
				continue // Default parameter
			}
			// Skip argN parameters - these are positional array arguments
			if strings.HasPrefix(paramName, "arg") && len(paramName) > 3 {
				if _, err := strconv.Atoi(paramName[3:]); err == nil {
					continue // Valid argN parameter
				}
			}
			found := false
			for _, paramSpec := range spec.Params {
				if strings.EqualFold(paramSpec.Name, paramName) {
					found = true
					break
				}
				// Check aliases
				for _, alias := range paramSpec.Aliases {
					if strings.EqualFold(alias, paramName) {
						found = true
						break
					}
				}
			}
			if !found {
				v.addError(location, fmt.Sprintf("unknown parameter '%s' in @%s", paramName, ann.Name), "error")
			}
		}
	}

	// Validate required parameters
	for _, paramSpec := range spec.Params {
		if !paramSpec.IsRequired {
			continue
		}

		paramValue, hasParam := ann.Params[paramSpec.Name]
		if !hasParam && paramSpec.Name != "" {
			// Check default parameter
			if defaultVal, hasDefault := ann.Params[""]; hasDefault && paramSpec.Name == "" {
				paramValue = defaultVal
				hasParam = true
			}
		}

		if !hasParam || paramValue == "" {
			severity := "error"
			if v.config.ValidateAnnotations == ValidationModeLax {
				severity = "warning"
			}
			v.addError(location, fmt.Sprintf("required parameter '%s' missing in @%s", paramSpec.Name, ann.Name), severity)
		}
	}

	// Validate parameter types and enum values
	for paramName, paramValue := range ann.Params {
		var paramSpec *annotations.AnnotationParam
		for i := range spec.Params {
			if paramName == "" && spec.Params[i].Name == "" {
				paramSpec = &spec.Params[i]
				break
			}
			if strings.EqualFold(spec.Params[i].Name, paramName) {
				paramSpec = &spec.Params[i]
				break
			}
			// Check aliases
			for _, alias := range spec.Params[i].Aliases {
				if strings.EqualFold(alias, paramName) {
					paramSpec = &spec.Params[i]
					break
				}
			}
		}

		if paramSpec == nil {
			continue
		}

		// Validate enum values
		if len(paramSpec.EnumValues) > 0 {
			valid := true

			// Check if the value is an array format (starts with '[')
			trimmedValue := strings.TrimSpace(paramValue)
			if strings.HasPrefix(trimmedValue, "[") && strings.HasSuffix(trimmedValue, "]") {
				// Parse array values
				arrayContent := strings.TrimPrefix(trimmedValue, "[")
				arrayContent = strings.TrimSuffix(arrayContent, "]")
				parts := strings.Split(arrayContent, ",")

				// Validate each value in the array
				for _, part := range parts {
					part = strings.TrimSpace(part)
					part = strings.Trim(part, `"'`)

					if part == "" {
						continue
					}

					partValid := false
					for _, enumValue := range paramSpec.EnumValues {
						if strings.EqualFold(part, enumValue) {
							partValid = true
							break
						}
					}
					if !partValid {
						valid = false
						break
					}
				}
			} else {
				// Single value validation
				valid = false
				for _, enumValue := range paramSpec.EnumValues {
					if strings.EqualFold(paramValue, enumValue) {
						valid = true
						break
					}
				}
			}

			if !valid {
				severity := "error"
				if v.config.ValidateAnnotations == ValidationModeLax {
					severity = "warning"
				}
				v.addError(location, fmt.Sprintf("invalid value '%s' for parameter '%s' in @%s, expected one of: %v",
					paramValue, paramName, ann.Name, paramSpec.EnumValues), severity)
			}
		}

		// Validate type
		v.validateParameterType(paramValue, paramSpec, ann.Name, paramName, location)
	}
}

// validateParameterType validates parameter type
func (v *Validator) validateParameterType(value string, paramSpec *annotations.AnnotationParam, annName, paramName, location string) {
	if len(paramSpec.Types) == 0 {
		return
	}

	validType := false
	for _, expectedType := range paramSpec.Types {
		switch strings.ToLower(expectedType) {
		case "string":
			validType = true // Any value is valid as string
		case "bool":
			lower := strings.ToLower(strings.TrimSpace(value))
			if lower == "true" || lower == "false" || lower == "1" || lower == "0" || lower == "yes" || lower == "no" {
				validType = true
			}
		case "int", "integer":
			if _, err := strconv.Atoi(value); err == nil {
				validType = true
			}
		case "number", "float":
			if _, err := strconv.ParseFloat(value, 64); err == nil {
				validType = true
			}
		case "[]string", "array":
			validType = true // List validation is complex, skip for now
		case "enum":
			validType = true // Enum validation is done separately
		case "null":
			validType = true // Null is valid for optional params
		}
		if validType {
			break
		}
	}

	if !validType && v.config.ValidateAnnotations == ValidationModeStrict {
		v.addError(location, fmt.Sprintf("invalid type for parameter '%s' in @%s, expected: %v",
			paramName, annName, paramSpec.Types), "warning")
	}
}

// AddError adds a validation error (public method for external validators)
func (v *Validator) AddError(location, message, severity string) {
	v.addError(location, message, severity)
}

// addError adds a validation error (internal)
func (v *Validator) addError(location, message, severity string) {
	v.errors = append(v.errors, ValidationError{
		Location: location,
		Message:  message,
		Severity: severity,
	})
}

// GetErrors returns all validation errors
func (v *Validator) GetErrors() []ValidationError {
	return v.errors
}

// HasErrors returns true if there are any errors
func (v *Validator) HasErrors() bool {
	for _, err := range v.errors {
		if err.Severity == "error" {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any warnings
func (v *Validator) HasWarnings() bool {
	for _, err := range v.errors {
		if err.Severity == "warning" {
			return true
		}
	}
	return false
}

// ShouldFail returns true if validation should fail generation
func (v *Validator) ShouldFail() bool {
	if v.config.Action == ValidationActionFail {
		return v.HasErrors()
	}
	return false
}

// LogErrors logs all validation errors and warnings
func (v *Validator) LogErrors(logger Logger) {
	if logger == nil {
		return
	}

	for _, err := range v.errors {
		msg := fmt.Sprintf("[%s] %s: %s", err.Severity, err.Location, err.Message)
		if err.Severity == "error" {
			if v.config.Action == ValidationActionWarn {
				logger.Warn(msg)
			} else {
				logger.Error(msg)
			}
		} else {
			logger.Warn(msg)
		}
	}
}

package gonnotation

import (
	"strconv"
	"strings"
)

// GetParamValue returns the value of an annotation parameter by name.
// It first checks for the exact parameter name, then checks aliases.
// Returns the value and true if found, empty string and false otherwise.
func (a *Annotation) GetParamValue(name string, aliases ...string) (string, bool) {

	// Check exact name first
	if val, ok := a.Params[name]; ok {
		return val, true
	}

	// Check aliases
	for _, alias := range aliases {
		if val, ok := a.Params[alias]; ok {
			return val, true
		}
	}

	return "", false
}

// GetParamValueOrDefault returns the value of an annotation parameter by name,
// or returns the default value if not found.
func (a *Annotation) GetParamValueOrDefault(name string, defaultValue string, aliases ...string) string {
	if val, ok := a.GetParamValue(name, aliases...); ok {
		return val
	}
	return defaultValue
}

// HasParam checks if an annotation has a parameter with the given name or aliases.
func (a *Annotation) HasParam(name string, aliases ...string) bool {
	_, ok := a.GetParamValue(name, aliases...)
	return ok
}

// GetParamBool returns a boolean parameter value. Accepted true values (case-insensitive):
// "true", "1", "yes", "on". Returns (value, true) if the param exists and was parsed, (false, false) if absent or unparsable.
func (a *Annotation) GetParamBool(name string, aliases ...string) (bool, bool) {
	if raw, ok := a.GetParamValue(name, aliases...); ok {
		lower := strings.ToLower(strings.TrimSpace(raw))
		switch lower {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		default:
			return false, false
		}
	}
	return false, false
}

// GetParamBoolOrDefault returns the bool value of a parameter or a provided default.
// If the parameter exists but can't be parsed it returns the default.
func (a *Annotation) GetParamBoolOrDefault(name string, def bool, aliases ...string) bool {
	if v, ok := a.GetParamBool(name, aliases...); ok {
		return v
	}
	return def
}

// GetParamInt returns an int parameter value. Returns (value, true) if present and parsed, (0,false) otherwise.
func (a *Annotation) GetParamInt(name string, aliases ...string) (int, bool) {
	if raw, ok := a.GetParamValue(name, aliases...); ok {
		iv, err := strconv.Atoi(strings.TrimSpace(raw))
		if err == nil {
			return iv, true
		}
	}
	return 0, false
}

// GetParamIntOrDefault returns the int parameter value or the provided default.
func (a *Annotation) GetParamIntOrDefault(name string, def int, aliases ...string) int {
	if v, ok := a.GetParamInt(name, aliases...); ok {
		return v
	}
	return def
}

// GetParamFloat returns a float64 parameter value. Returns (value,true) if parsed, (0,false) otherwise.
func (a *Annotation) GetParamFloat(name string, aliases ...string) (float64, bool) {
	if raw, ok := a.GetParamValue(name, aliases...); ok {
		fv, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err == nil {
			return fv, true
		}
	}
	return 0, false
}

// GetParamFloatOrDefault returns the float parameter value or the provided default.
func (a *Annotation) GetParamFloatOrDefault(name string, def float64, aliases ...string) float64 {
	if v, ok := a.GetParamFloat(name, aliases...); ok {
		return v
	}
	return def
}

// GetParamStringList returns a list of strings from a parameter. It supports:
// 1. Comma or semicolon separated single param value (e.g., "a,b,c")
// 2. Multiple argN parameters (arg0, arg1, ...)
// Returns (list,true) if any source is present, (nil,false) otherwise.
func (a *Annotation) GetParamStringList(name string, aliases ...string) ([]string, bool) {
	if raw, ok := a.GetParamValue(name, aliases...); ok && strings.TrimSpace(raw) != "" {
		// Remove array brackets if present
		raw = strings.TrimSpace(raw)

		raw = strings.TrimPrefix(raw, "[")
		raw = strings.TrimSuffix(raw, "]")

		sepReplacer := strings.NewReplacer(";", ",")
		clean := sepReplacer.Replace(raw)
		parts := strings.Split(clean, ",")
		var list []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			// Remove quotes if present
			p = strings.Trim(p, `"'`)
			if p != "" {
				list = append(list, p)
			}
		}
		return list, true
	}
	// Fallback to argN style parameters
	var list []string
	for k, v := range a.Params {
		if strings.HasPrefix(k, "arg") {
			val := strings.TrimSpace(v)
			if val != "" {
				list = append(list, val)
			}
		}
	}
	if len(list) > 0 {
		return list, true
	}
	return nil, false
}

// GetParamStringListOrDefault returns the string list parameter or the provided default.
func (a *Annotation) GetParamStringListOrDefault(name string, def []string, aliases ...string) []string {
	if v, ok := a.GetParamStringList(name, aliases...); ok {
		return v
	}
	return def
}

// GetTagValue returns the value of a struct tag by name.
// It first checks for the exact tag name, then checks aliases.
// Returns the value and true if found, empty string and false otherwise.
func (tags StructTags) GetTagValue(name string, aliases ...string) (string, bool) {
	// Check exact name first
	if val, ok := tags[name]; ok {
		return val, true
	}

	// Check aliases
	for _, alias := range aliases {
		if val, ok := tags[alias]; ok {
			return val, true
		}
	}

	return "", false
}

// GetTagValueOrDefault returns the value of a struct tag by name,
// or returns the default value if not found.
func (tags StructTags) GetTagValueOrDefault(name string, defaultValue string, aliases ...string) string {
	if val, ok := tags.GetTagValue(name, aliases...); ok {
		return val
	}
	return defaultValue
}

// HasTag checks if a struct has a tag with the given name or aliases.
func (tags StructTags) HasTag(name string, aliases ...string) bool {
	_, ok := tags.GetTagValue(name, aliases...)
	return ok
}

// GetTagBool typed helpers mirror annotation helpers for consistency.
func (tags StructTags) GetTagBool(name string, aliases ...string) (bool, bool) {
	if raw, ok := tags.GetTagValue(name, aliases...); ok {
		lower := strings.ToLower(strings.TrimSpace(raw))
		switch lower {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		default:
			return false, false
		}
	}
	return false, false
}

func (tags StructTags) GetTagBoolOrDefault(name string, def bool, aliases ...string) bool {
	if v, ok := tags.GetTagBool(name, aliases...); ok {
		return v
	}
	return def
}

func (tags StructTags) GetTagInt(name string, aliases ...string) (int, bool) {
	if raw, ok := tags.GetTagValue(name, aliases...); ok {
		iv, err := strconv.Atoi(strings.TrimSpace(raw))
		if err == nil {
			return iv, true
		}
	}
	return 0, false
}

func (tags StructTags) GetTagIntOrDefault(name string, def int, aliases ...string) int {
	if v, ok := tags.GetTagInt(name, aliases...); ok {
		return v
	}
	return def
}

func (tags StructTags) GetTagFloat(name string, aliases ...string) (float64, bool) {
	if raw, ok := tags.GetTagValue(name, aliases...); ok {
		fv, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err == nil {
			return fv, true
		}
	}
	return 0, false
}

func (tags StructTags) GetTagFloatOrDefault(name string, def float64, aliases ...string) float64 {
	if v, ok := tags.GetTagFloat(name, aliases...); ok {
		return v
	}
	return def
}

func (tags StructTags) GetTagStringList(name string, aliases ...string) ([]string, bool) {
	if raw, ok := tags.GetTagValue(name, aliases...); ok && strings.TrimSpace(raw) != "" {
		sepReplacer := strings.NewReplacer(";", ",")
		clean := sepReplacer.Replace(raw)
		parts := strings.Split(clean, ",")
		var list []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				list = append(list, p)
			}
		}
		return list, true
	}
	return nil, false
}

func (tags StructTags) GetTagStringListOrDefault(name string, def []string, aliases ...string) []string {
	if v, ok := tags.GetTagStringList(name, aliases...); ok {
		return v
	}
	return def
}

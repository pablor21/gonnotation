package annotations

import (
	"go/ast"
	"strings"
)

// StructTags represents parsed struct tags
type StructTags map[string]string

// ParseAnnotations extracts annotations from comment groups
func ParseAnnotations(comments []*ast.CommentGroup) []Annotation {
	var annotations []Annotation

	for _, cg := range comments {
		for _, c := range cg.List {
			text := strings.TrimSpace(c.Text)
			text = strings.TrimPrefix(text, "//")
			text = strings.TrimPrefix(text, "/*")
			text = strings.TrimSuffix(text, "*/")
			text = strings.TrimSpace(text)

			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "*")
				line = strings.TrimSpace(line)

				if strings.HasPrefix(line, "@") {
					ann := parseAnnotation(line)
					if ann.Name != "" {
						annotations = append(annotations, ann)
					}
				}
			}
		}
	}

	return annotations
}

// parseAnnotation parses single annotation: @name or @name(key:value) or @name key="value"
func parseAnnotation(line string) Annotation {
	ann := Annotation{
		RawText: line,
		Params:  make(map[string]string),
	}

	line = strings.TrimPrefix(line, "@")
	parenIdx := strings.Index(line, "(")

	// Format 1: @name(key:value, key2:value2)
	if parenIdx != -1 {
		ann.Name = strings.TrimSpace(line[:parenIdx])
		paramsStr := line[parenIdx+1:]
		if endIdx := strings.LastIndex(paramsStr, ")"); endIdx != -1 {
			paramsStr = paramsStr[:endIdx]
		}
		ann.Params = parseParamsParentheses(paramsStr)
		return ann
	}

	// Format 2: @name key="value" key2="value2" (space-separated)
	// or Format 3: @name (no parameters)
	parts := splitAnnotationParts(line)
	if len(parts) == 0 {
		return ann
	}

	ann.Name = strings.TrimSpace(parts[0])

	// If there are more parts, parse them as space-separated key="value" pairs
	if len(parts) > 1 {
		ann.Params = parseParamsSpaceSeparated(parts[1:])
	}

	return ann
}

// parseParamsParentheses parses key:value pairs from parentheses format: (key:value, key2:value2)
func parseParamsParentheses(s string) map[string]string {
	params := make(map[string]string)
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)
	bracketDepth := 0

	for _, ch := range s {
		if ch == '"' || ch == '\'' {
			if !inQuotes {
				inQuotes = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuotes = false
			}
			current.WriteRune(ch)
		} else if ch == '[' && !inQuotes {
			bracketDepth++
			current.WriteRune(ch)
		} else if ch == ']' && !inQuotes {
			bracketDepth--
			current.WriteRune(ch)
		} else if ch == ',' && !inQuotes && bracketDepth == 0 {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	positionalIdx := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Support both : and = as separators
		sepIdx := -1
		for i, ch := range part {
			if (ch == ':' || ch == '=') && !isInQuotes(part, i) {
				sepIdx = i
				break
			}
		}

		if sepIdx == -1 {
			// No separator - could be a boolean flag or positional argument
			// Check if the original part (before removing quotes) was quoted
			wasQuoted := (strings.HasPrefix(part, `"`) && strings.HasSuffix(part, `"`)) ||
				(strings.HasPrefix(part, `'`) && strings.HasSuffix(part, `'`))
			value := strings.Trim(part, `"'`)

			// If it was quoted, treat it as a positional argument (not a boolean flag)
			// Otherwise, check if it looks like a boolean flag (simple identifier)
			if !wasQuoted && isBooleanFlag(value) {
				params[value] = "true"
			} else {
				// Positional argument
				if positionalIdx == 0 {
					// First positional argument uses empty key (default parameter)
					params[""] = value
				} else {
					// Subsequent positional arguments are stored as indexed values
					// This allows validation to accept them as valid array elements
					params[""] = params[""] + "," + value
				}
				positionalIdx++
			}
			continue
		}

		key := strings.TrimSpace(part[:sepIdx])
		value := strings.TrimSpace(part[sepIdx+1:])
		value = strings.Trim(value, `"'`)
		params[key] = value
	}

	return params
}

// splitAnnotationParts splits annotation line into parts, respecting quotes
func splitAnnotationParts(line string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)

	for _, ch := range line {
		if ch == '"' || ch == '\'' {
			if !inQuotes {
				inQuotes = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuotes = false
			}
			current.WriteRune(ch)
		} else if ch == ' ' && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseParamsSpaceSeparated parses space-separated key="value" pairs
func parseParamsSpaceSeparated(parts []string) map[string]string {
	params := make(map[string]string)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Support both = and : as separators
		sepIdx := -1
		for i, ch := range part {
			if (ch == '=' || ch == ':') && !isInQuotes(part, i) {
				sepIdx = i
				break
			}
		}

		if sepIdx == -1 {
			// No separator, treat as boolean flag
			params[part] = "true"
			continue
		}

		key := strings.TrimSpace(part[:sepIdx])
		value := strings.TrimSpace(part[sepIdx+1:])
		value = strings.Trim(value, `"'`)
		params[key] = value
	}

	return params
}

// isInQuotes checks if a character at given index is inside quotes
func isInQuotes(s string, idx int) bool {
	inQuotes := false
	quoteChar := rune(0)

	for i, ch := range s {
		if i >= idx {
			break
		}
		if ch == '"' || ch == '\'' {
			if !inQuotes {
				inQuotes = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuotes = false
			}
		}
	}

	return inQuotes
}

// isBooleanFlag checks if a string looks like a boolean flag (simple identifier)
func isBooleanFlag(s string) bool {
	// Boolean flags are simple identifiers without special characters
	// They shouldn't contain spaces, quotes, or other punctuation
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, ch := range s {
		// Valid characters: a-z, A-Z, 0-9, _, -
		isLetter := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
		isDigit := ch >= '0' && ch <= '9'
		isSpecial := ch == '_' || ch == '-'
		if !isLetter && !isDigit && !isSpecial {
			return false
		}
	}
	return true
}

// ExtractFileLevelNamespace extracts the @namespace annotation from file-level comments
// (comments that appear right after the package declaration)
func ExtractFileLevelNamespace(file *ast.File) string {
	if file == nil || file.Doc == nil {
		return ""
	}

	// Parse annotations from the package comment group
	annotations := ParseAnnotations([]*ast.CommentGroup{file.Doc})

	for _, ann := range annotations {
		if strings.ToLower(ann.Name) == "namespace" {
			// Check if it's @namespace("value") or @namespace(value="...")
			if len(ann.Params) == 0 && ann.RawText != "" {
				// Extract value from @namespace("value") format
				text := strings.TrimSpace(ann.RawText)
				text = strings.TrimPrefix(text, "@")
				if strings.HasPrefix(text, "namespace(") && strings.HasSuffix(text, ")") {
					val := strings.TrimPrefix(text, "namespace(")
					val = strings.TrimSuffix(val, ")")
					val = strings.Trim(val, "\"'")
					return strings.TrimSpace(val)
				}
			}
			// Check params (e.g., @namespace(value="auth"))
			if val, exists := ann.Params["value"]; exists {
				return val
			}
			// Check if the first param is the namespace (e.g., @namespace(auth))
			for _, val := range ann.Params {
				return val
			}
		}
	}

	return ""
}

// ExtractTypeNamespace extracts the @namespace annotation from type-level annotations
// Returns namespace value or empty string if not found
func ExtractTypeNamespace(annotations []Annotation) string {
	for _, ann := range annotations {
		if strings.ToLower(ann.Name) == "namespace" {
			// Check if it's @namespace("value") format
			if len(ann.Params) == 0 && ann.RawText != "" {
				text := strings.TrimSpace(ann.RawText)
				text = strings.TrimPrefix(text, "@")
				if strings.HasPrefix(text, "namespace(") && strings.HasSuffix(text, ")") {
					val := strings.TrimPrefix(text, "namespace(")
					val = strings.TrimSuffix(val, ")")
					val = strings.Trim(val, "\"'")
					return strings.TrimSpace(val)
				}
			}
			// Check params
			if val, exists := ann.Params["value"]; exists {
				return val
			}
			if val, exists := ann.Params["namespace"]; exists {
				return val
			}
			// Return first param value if exists
			for _, val := range ann.Params {
				return val
			}
		}

		// Also check for namespace parameter in other annotations
		// e.g., @gqlType(namespace="auth")
		if val, exists := ann.Params["namespace"]; exists {
			return val
		}
	}

	return ""
}

package annotations

import (
	"testing"
)

func TestAnnotationFormats(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantName   string
		wantParams map[string]string
	}{
		{
			name:     "space-separated format",
			input:    `@schema title="Product" description="A product"`,
			wantName: "schema",
			wantParams: map[string]string{
				"title":       "Product",
				"description": "A product",
			},
		},
		{
			name:     "parentheses format with colon",
			input:    `@schema(title:"Product", description:"A product")`,
			wantName: "schema",
			wantParams: map[string]string{
				"title":       "Product",
				"description": "A product",
			},
		},
		{
			name:     "parentheses format with equals",
			input:    `@schema(title="Product", description="A product")`,
			wantName: "schema",
			wantParams: map[string]string{
				"title":       "Product",
				"description": "A product",
			},
		},
		{
			name:     "boolean flags in parentheses",
			input:    `@schema(readonly, nullable)`,
			wantName: "schema",
			wantParams: map[string]string{
				"readonly": "true",
				"nullable": "true",
			},
		},
		{
			name:     "mixed flags and key-value pairs",
			input:    `@schema(title:"Product", readonly, description:"A product")`,
			wantName: "schema",
			wantParams: map[string]string{
				"title":       "Product",
				"readonly":    "true",
				"description": "A product",
			},
		},
		{
			name:     "positional parameter only",
			input:    `@schema("Product")`,
			wantName: "schema",
			wantParams: map[string]string{
				"": "Product",
			},
		},
		{
			name:     "multiple positional parameters",
			input:    `@schema("Product", "Description")`,
			wantName: "schema",
			wantParams: map[string]string{
				"": "Product,Description",
			},
		},
		{
			name:     "mixed positional and named parameters",
			input:    `@schema("Product", title:"Title", description:"Desc")`,
			wantName: "schema",
			wantParams: map[string]string{
				"":            "Product",
				"title":       "Title",
				"description": "Desc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ann := parseAnnotation(tt.input)
			if ann.Name != tt.wantName {
				t.Errorf("parseAnnotation() name = %v, want %v", ann.Name, tt.wantName)
			}
			for k, expectedV := range tt.wantParams {
				if actualV, ok := ann.Params[k]; !ok || actualV != expectedV {
					t.Errorf("parseAnnotation() params[%s] = %v, want %v", k, actualV, expectedV)
				}
			}
		})
	}
}

func TestAnnotationHelpers(t *testing.T) {
	ann := Annotation{
		Name: "test",
		Params: map[string]string{
			"name":     "TestName",
			"active":   "true",
			"count":    "42",
			"price":    "19.99",
			"tags":     "a,b,c",
			"optional": "false",
		},
	}

	// Test GetParamValue
	if val, ok := ann.GetParamValue("name"); !ok || val != "TestName" {
		t.Errorf("GetParamValue failed: got %s, %v", val, ok)
	}

	// Test GetParamValueOrDefault
	if val := ann.GetParamValueOrDefault("missing", "default"); val != "default" {
		t.Errorf("GetParamValueOrDefault failed: got %s", val)
	}

	// Test GetParamBool
	if val, ok := ann.GetParamBool("active"); !ok || !val {
		t.Errorf("GetParamBool failed: got %v, %v", val, ok)
	}

	// Test GetParamInt
	if val, ok := ann.GetParamInt("count"); !ok || val != 42 {
		t.Errorf("GetParamInt failed: got %d, %v", val, ok)
	}

	// Test GetParamFloat
	if val, ok := ann.GetParamFloat("price"); !ok || val != 19.99 {
		t.Errorf("GetParamFloat failed: got %f, %v", val, ok)
	}

	// Test GetParamStringList
	if list, ok := ann.GetParamStringList("tags"); !ok || len(list) != 3 || list[0] != "a" {
		t.Errorf("GetParamStringList failed: got %v, %v", list, ok)
	}
}

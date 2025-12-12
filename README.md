# Gonnotation

[![CI](https://github.com/pablor21/gonnotation/workflows/CI/badge.svg)](https://github.com/pablor21/gonnotation/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/pablor21/gonnotation.svg)](https://pkg.go.dev/github.com/pablor21/gonnotation)
[![Go Report Card](https://goreportcard.com/badge/github.com/pablor21/gonnotation)](https://goreportcard.com/report/github.com/pablor21/gonnotation)

A **truly generic** Go annotation parser library that enables `@annotation` parsing using specs.

This is a raw library, you usually will use this througth the specs defined in a library.

## What is an annotation?

An annotation is any comment on a type or var/const or file that is prefixed with `@`, for example:

```go
// @myannotation(param1:'test', param2:true)
// @maysecondannotation param1='test' param2=true
type MyType struct {
    Name string `myanntag:"param1:'test'"`
    // @myfieldannotation(params ...)
    Age int
}
```

## How to create and use annotation specs

Annotation specs are a way to define which annotations are valid where, for example, this is a spec definition:

```go
import "github.com/pablor21/gonnotation"


var annotationSpecs = []gonnotation.AnnotationSpec{
	{
		Name:    "excludeall",
		Aliases: []string{"ignore", "ignored"},
		Params: []gonnotation.AnnotationParam{
			{
				Name:         "",
				IsDefault:    true,
				DefaultValue: "true",
				Types:        []string{"string"},
			},
		},
		Description: "Exclude the annotated element from the types",
		Multiple:    false,
		ValidOn: []ValidOn{
			gonnotation.StructAnnotationPlacement,
		},
	}
}


var tagParams = []gonnotation.TagParam{
	{
		Name:         "ignore",
		Aliases:      []string{"exclude", "skip"},
		Types:        []string{"bool"},
		Description:  "Indicates if the field should be ignored",
		DefaultValue: "true",
	},
	{
		Name:         "include",
		Aliases:      []string{"add", "scan"},
		Types:        []string{"bool"},
		Description:  "Indicates if the field should be included",
		DefaultValue: "true",
	},
}


var Specs = gonnotation.AnnotationSpecs{
	Annotations: annotationSpecs,
	StructTags:  tagParams,
}

```

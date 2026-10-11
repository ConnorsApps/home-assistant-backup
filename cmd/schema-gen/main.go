// Command schema-gen writes ../../charts/hass-backup/values.schema.json, which
// Helm enforces on install and lint and editors use for completion. Run it
// from this directory. With -check it writes nothing and exits 1 when the
// file on disk is stale.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"

	jsonschema "github.com/swaggest/jsonschema-go"
)

const (
	draft07    = "http://json-schema.org/draft-07/schema#" // what Helm documents
	schemaFile = "../../charts/hass-backup/values.schema.json"
)

// ownPkg is "main" under `go run` but the import path under `go test`.
var ownPkg = reflect.TypeFor[values]().PkgPath()

func main() {
	check := flag.Bool("check", false, "fail if "+schemaFile+" is out of date instead of writing it")
	flag.Parse()

	schema, err := build()
	if err != nil {
		die(err)
	}
	if *check {
		if have, err := os.ReadFile(schemaFile); err != nil || !bytes.Equal(have, schema) {
			die(fmt.Errorf("%s is out of date: run `go run .` in cmd/schema-gen", schemaFile))
		}
		return
	}
	if err := os.WriteFile(schemaFile, schema, 0o644); err != nil {
		die(err)
	}
	fmt.Printf("Wrote %s\n", schemaFile)
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "schema-gen: %v\n", err)
	os.Exit(1)
}

func build() ([]byte, error) {
	schema, err := reflectSchema(values{}, newK8sDocs())
	if err != nil {
		return nil, fmt.Errorf("reflect: %w", err)
	}
	schema.WithSchema(draft07).
		WithTitle("hass-backup chart values").
		WithDescription("Helm values for the hass-backup chart")

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(schema); err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return buf.Bytes(), nil
}

func reflectSchema(v any, docs *k8sDocs) (*jsonschema.Schema, error) {
	r := jsonschema.Reflector{}
	schema, err := r.Reflect(v,
		jsonschema.InlineRefs,
		jsonschema.InterceptSchema(func(p jsonschema.InterceptSchemaParams) (bool, error) {
			if !p.Value.IsValid() {
				return false, nil
			}
			if !p.Processed {
				return k8sQuantity(p.Value, p.Schema)
			}
			t := p.Value.Type()
			for t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			switch {
			case isK8sType(t):
				return false, docs.enrich(t, p.Schema)
			case t.Kind() == reflect.Struct && t.PkgPath() == ownPkg:
				ownStruct(t, p.Schema)
			}
			return false, nil
		}),
	)
	return &schema, err
}

// ownStruct rejects unknown keys on our own structs, so a typo fails `helm
// lint`. Kubernetes types stay open: a newer cluster's valid field must not be
// rejected. env and secret list the variables worth completing but accept any
// other name, since each key becomes an environment variable.
func ownStruct(t reflect.Type, s *jsonschema.Schema) {
	switch t {
	case reflect.TypeFor[envValues]():
		// The template quotes every value, so any scalar renders.
		scalar := &jsonschema.Schema{Type: &jsonschema.Type{SliceOfSimpleTypeValues: []jsonschema.SimpleType{
			jsonschema.String, jsonschema.Number, jsonschema.Boolean,
		}}}
		s.AdditionalProperties = &jsonschema.SchemaOrBool{TypeObject: scalar}
		s.WithPropertyNames(envName())
	case reflect.TypeFor[secretValues]():
		// Rendered as stringData, which only takes strings: a number is a
		// type error at install time, so catch it at lint time.
		str := (&jsonschema.Schema{}).WithType(jsonschema.String.Type())
		s.AdditionalProperties = &jsonschema.SchemaOrBool{TypeObject: str}
		s.WithPropertyNames(envName())
	default:
		closed := false
		s.AdditionalProperties = &jsonschema.SchemaOrBool{TypeBoolean: &closed}
	}
}

// envName is what Kubernetes accepts as an environment variable name.
func envName() jsonschema.SchemaOrBool {
	return (&jsonschema.Schema{}).WithPattern(envNamePattern).ToSchemaOrBool()
}

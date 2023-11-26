// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command update helps identify and perform updates to Firefly's dependencies.
package update

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/bazelbuild/buildtools/build"

	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

var program = filepath.Base(os.Args[0])

func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("update", flag.ExitOnError)

	var help bool
	flags.BoolVar(&help, "h", false, "Show this message and exit.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s OPTIONS\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	args = flags.Args()
	if len(args) != 0 {
		log.Printf("Unexpected arguments: %s\n", strings.Join(args, " "))
		flags.Usage()
	}

	return UpdateDependencies(vendeps.DepsBzl)
}

// UnmarshalFields processes the AST node for a
// Starlark function call and stores its parameters
// into data.
//
// UnmarshalFields will return an error if any required
// fields were unset, or if any additional fields were
// found in the AST.
func UnmarshalFields(call *build.CallExpr, v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("invalid set of fields: got %v, expected struct", val.Kind())
	}

	// Use reflection to extract the data for
	// each field in a format we can process
	// more easily as we iterate through the
	// call.

	type FieldData struct {
		Name     string
		Optional bool
		Ignore   bool
		Value    *string
		Ptr      **string
	}

	valType := val.Type()
	fieldType := reflect.TypeOf(StringField{})
	numFields := val.NumField()
	fields := make([]*FieldData, numFields)
	fieldsMap := make(map[string]*FieldData)
	for i := 0; i < numFields; i++ {
		valField := val.Field(i)
		typeField := valType.Field(i)
		if valField.Type() != fieldType {
			return fmt.Errorf("invalid set of fields: field %s has unexpected type %s, want %s", typeField.Name, valField.Type(), fieldType)
		}

		name, ok := typeField.Tag.Lookup("bzl")
		optional := false
		ignore := false
		if strings.HasSuffix(name, ",optional") {
			optional = true
			name = strings.TrimSuffix(name, ",optional")
		} else if strings.HasSuffix(name, ",ignore") {
			ignore = true
			name = strings.TrimSuffix(name, ",ignore")
		}

		if !ok {
			name = typeField.Name
		}

		if name == "" {
			return fmt.Errorf("invalid set of fields: field %s has no field name", typeField.Name)
		}

		// We already know valField is a struct.
		valPtr := valField.Field(0).Addr().Interface().(*string)
		ptrPtr := valField.Field(1).Addr().Interface().(**string)

		field := &FieldData{
			Name:     name,
			Optional: optional,
			Ignore:   ignore,
			Value:    valPtr,
			Ptr:      ptrPtr,
		}

		if fieldsMap[name] != nil {
			return fmt.Errorf("invalid set of fields: multiple fields have the name %q", name)
		}

		fields[i] = field
		fieldsMap[name] = field
	}

	// Now we have the field data ready, we can
	// start parsing the call.

	for i, expr := range call.List {
		assign, ok := expr.(*build.AssignExpr)
		if !ok {
			return fmt.Errorf("field %d in the call is not an assignment", i)
		}

		lhs, ok := assign.LHS.(*build.Ident)
		if !ok {
			return fmt.Errorf("field %d in the call assigns to a non-identifier value %#v", i, assign.LHS)
		}

		field := fieldsMap[lhs.Name]
		if field == nil {
			return fmt.Errorf("field %d in the call has unexpected field %q", i, lhs.Name)
		}

		if field.Ignore {
			continue
		}

		if *field.Ptr != nil {
			return fmt.Errorf("field %d in the call assigns to %s for the second time", i, lhs.Name)
		}

		rhs, ok := assign.RHS.(*build.StringExpr)
		if !ok {
			return fmt.Errorf("field %d in the call (%s) has non-string value %#v", i, lhs.Name, assign.RHS)
		}

		*field.Value = rhs.Value
		*field.Ptr = &rhs.Value
	}

	// Check we've got values for all required
	// fields.
	for _, field := range fields {
		if field.Optional || field.Ignore {
			continue
		}

		if *field.Ptr != nil {
			continue
		}

		return fmt.Errorf("function call had no value for required field %s", field.Name)
	}

	return nil
}

// StringField represents a field in a Starlark
// function that receives a string literal.
type StringField struct {
	// The parsed value.
	Value string

	// A pointer to the original AST node, which
	// can be modified to update the AST.
	Ptr *string
}

func (f StringField) String() string {
	return f.Value
}

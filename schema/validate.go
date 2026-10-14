package schema

import (
	"errors"
	"fmt"
	"strconv"
)

// Validate checks that a parsed schema makes sense. It reports every
// problem it finds, not just the first one.
func Validate(s *Schema) error {
	errs := validateDatasource(s.Datasource)

	seen := map[string]bool{}
	for _, m := range s.Models {
		if seen[m.Name] {
			errs = append(errs, fmt.Errorf("line %d: duplicate model %q", m.Line, m.Name))
		}
		seen[m.Name] = true
		errs = append(errs, validateModel(m)...)
	}

	return errors.Join(errs...)
}

func validateDatasource(ds *Datasource) []error {
	if ds == nil {
		return []error{errors.New("schema has no datasource block")}
	}

	var errs []error
	if ds.Provider == "" {
		errs = append(errs, fmt.Errorf("line %d: datasource %s has no provider", ds.Line, ds.Name))
	}
	if ds.URL == "" && ds.URLEnv == "" {
		errs = append(errs, fmt.Errorf("line %d: datasource %s has no url", ds.Line, ds.Name))
	}
	return errs
}

func validateModel(m Model) []error {
	var errs []error

	ids := 0
	for _, f := range m.Fields {
		if f.ID {
			ids++
		}
	}
	switch {
	case ids == 0:
		errs = append(errs, fmt.Errorf("line %d: model %s has no @id field", m.Line, m.Name))
	case ids > 1:
		errs = append(errs, fmt.Errorf("line %d: model %s has more than one @id field", m.Line, m.Name))
	}

	seen := map[string]bool{}
	for _, f := range m.Fields {
		if seen[f.Name] {
			errs = append(errs, fmt.Errorf("line %d: duplicate field %q in model %s", f.Line, f.Name, m.Name))
		}
		seen[f.Name] = true

		if f.ID && f.Optional {
			errs = append(errs, fmt.Errorf("line %d: @id field %s cannot be optional", f.Line, f.Name))
		}
		if f.Default != nil {
			if err := validateDefault(f); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func validateDefault(f Field) error {
	d := f.Default

	switch d.Kind {
	case DefaultFunc:
		switch d.Value {
		case "autoincrement":
			// MySQL only allows AUTO_INCREMENT on a key column.
			if f.Type != Int || !f.ID {
				return fmt.Errorf("line %d: autoincrement() is only allowed on an Int @id field", f.Line)
			}
		case "now":
			if f.Type != DateTime {
				return fmt.Errorf("line %d: now() is only allowed on a DateTime field", f.Line)
			}
		default:
			return fmt.Errorf("line %d: unknown function %s() in @default", f.Line, d.Value)
		}
	case DefaultString:
		if f.Type != String {
			return defaultMismatch(f)
		}
	case DefaultBool:
		if f.Type != Boolean {
			return defaultMismatch(f)
		}
	case DefaultNumber:
		if f.Type != Int {
			return defaultMismatch(f)
		}
		// Int is a 32-bit column, so the default has to fit in one.
		if _, err := strconv.ParseInt(d.Value, 10, 32); err != nil {
			return fmt.Errorf("line %d: @default(%s) is not a valid Int", f.Line, d.Value)
		}
	}
	return nil
}

func defaultMismatch(f Field) error {
	value := f.Default.Value
	if f.Default.Kind == DefaultString {
		value = strconv.Quote(value)
	}
	return fmt.Errorf("line %d: @default(%s) does not match type %s", f.Line, value, f.Type)
}

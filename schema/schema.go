// Package schema holds the in-memory form of a schema.prismo file.
package schema

// Schema is a parsed schema file.
type Schema struct {
	// Datasource is nil if the file has no datasource block.
	Datasource *Datasource
	Models     []Model
}

// Datasource describes the database a schema targets.
type Datasource struct {
	Name     string
	Provider string
	// URL is set when the url is written as a literal string.
	URL string
	// URLEnv is set when the url is written as env("NAME").
	URLEnv string
	Line   int
}

// Model is a model block, which maps to a database table.
type Model struct {
	Name   string
	Fields []Field
	Line   int
}

// Field is a model field, which maps to a table column.
type Field struct {
	Name     string
	Type     FieldType
	Optional bool
	ID       bool
	Unique   bool
	// Default is nil if the field has no @default.
	Default *Default
	Line    int
}

// FieldType is a scalar type a field can have.
type FieldType string

// Supported field types.
const (
	Int      FieldType = "Int"
	String   FieldType = "String"
	Boolean  FieldType = "Boolean"
	DateTime FieldType = "DateTime"
)

// Default is the value of a @default attribute.
type Default struct {
	Kind DefaultKind
	// Value is the function name for DefaultFunc ("autoincrement", "now"),
	// the unquoted text for DefaultString, and the literal as written
	// otherwise ("42", "true").
	Value string
}

// DefaultKind says what kind of value a @default holds.
type DefaultKind int

// Kinds of default values.
const (
	DefaultFunc DefaultKind = iota
	DefaultString
	DefaultNumber
	DefaultBool
)

// ParseFieldType returns the FieldType named s, and false if there is none.
func ParseFieldType(s string) (FieldType, bool) {
	switch t := FieldType(s); t {
	case Int, String, Boolean, DateTime:
		return t, true
	default:
		return "", false
	}
}

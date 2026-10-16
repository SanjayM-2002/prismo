package mysql

import (
	"reflect"
	"testing"

	"github.com/SanjayM-2002/prismo/migrate"
	"github.com/SanjayM-2002/prismo/schema"
)

func TestColumnFromMySQL(t *testing.T) {
	str := func(s string) *string { return &s }

	tests := []struct {
		name       string
		columnType string
		nullable   string
		def        *string
		extra      string
		want       migrate.Column
	}{
		{
			name: "auto-increment int", columnType: "int", nullable: "NO", extra: "auto_increment",
			want: migrate.Column{Name: "c", Type: schema.Int, AutoIncrement: true},
		},
		{
			name: "int with display width", columnType: "int(11)", nullable: "NO",
			want: migrate.Column{Name: "c", Type: schema.Int},
		},
		{
			name: "nullable string", columnType: "varchar(191)", nullable: "YES",
			want: migrate.Column{Name: "c", Type: schema.String, Nullable: true},
		},
		{
			name: "string default", columnType: "varchar(191)", nullable: "NO", def: str(`it's a \ test`),
			want: migrate.Column{Name: "c", Type: schema.String, Default: &schema.Default{Kind: schema.DefaultString, Value: `it's a \ test`}},
		},
		{
			name: "negative int default", columnType: "int", nullable: "NO", def: str("-1"),
			want: migrate.Column{Name: "c", Type: schema.Int, Default: &schema.Default{Kind: schema.DefaultNumber, Value: "-1"}},
		},
		{
			name: "boolean default true", columnType: "tinyint(1)", nullable: "NO", def: str("1"),
			want: migrate.Column{Name: "c", Type: schema.Boolean, Default: &schema.Default{Kind: schema.DefaultBool, Value: "true"}},
		},
		{
			name: "boolean default false", columnType: "tinyint(1)", nullable: "NO", def: str("0"),
			want: migrate.Column{Name: "c", Type: schema.Boolean, Default: &schema.Default{Kind: schema.DefaultBool, Value: "false"}},
		},
		{
			name: "datetime default now", columnType: "datetime(3)", nullable: "NO", def: str("CURRENT_TIMESTAMP(3)"), extra: "DEFAULT_GENERATED",
			want: migrate.Column{Name: "c", Type: schema.DateTime, Default: &schema.Default{Kind: schema.DefaultFunc, Value: "now"}},
		},
		{
			name: "other expression default", columnType: "varchar(191)", nullable: "NO", def: str("uuid()"), extra: "DEFAULT_GENERATED",
			want: migrate.Column{Name: "c", Type: schema.String, Default: &schema.Default{Kind: schema.DefaultFunc, Value: "uuid()"}},
		},
		{
			name: "type prismo doesn't create", columnType: "bigint", nullable: "NO",
			want: migrate.Column{Name: "c", Type: schema.FieldType("bigint")},
		},
		{
			name: "string with another length", columnType: "varchar(255)", nullable: "NO",
			want: migrate.Column{Name: "c", Type: schema.FieldType("varchar(255)")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := columnFromMySQL("c", tt.columnType, tt.nullable, tt.def, tt.extra)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("columnFromMySQL\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

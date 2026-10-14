package mysql

import (
	"strings"
	"testing"

	"github.com/SanjayM-2002/prismo/migrate"
	"github.com/SanjayM-2002/prismo/schema"
)

func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "User", want: "`User`"},
		{in: "created_at", want: "`created_at`"},
		{in: "we`ird", want: "`we``ird`"},
	}

	for _, tt := range tests {
		if got := (Dialect{}).QuoteIdent(tt.in); got != tt.want {
			t.Errorf("QuoteIdent(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCreateTable(t *testing.T) {
	tests := []struct {
		name  string
		table migrate.CreateTable
		want  string
	}{
		{
			name: "all column options",
			table: migrate.CreateTable{
				Name: "User",
				Columns: []migrate.Column{
					{Name: "id", Type: schema.Int, AutoIncrement: true},
					{Name: "email", Type: schema.String},
					{Name: "name", Type: schema.String, Nullable: true, Default: &schema.Default{Kind: schema.DefaultString, Value: "anonymous"}},
					{Name: "age", Type: schema.Int, Default: &schema.Default{Kind: schema.DefaultNumber, Value: "-1"}},
					{Name: "active", Type: schema.Boolean, Default: &schema.Default{Kind: schema.DefaultBool, Value: "true"}},
					{Name: "createdAt", Type: schema.DateTime, Default: &schema.Default{Kind: schema.DefaultFunc, Value: "now"}},
				},
				PrimaryKey: "id",
				Unique:     []string{"email"},
			},
			want: "CREATE TABLE `User` (\n" +
				"  `id` INT NOT NULL AUTO_INCREMENT,\n" +
				"  `email` VARCHAR(191) NOT NULL,\n" +
				"  `name` VARCHAR(191) NULL DEFAULT 'anonymous',\n" +
				"  `age` INT NOT NULL DEFAULT -1,\n" +
				"  `active` BOOLEAN NOT NULL DEFAULT true,\n" +
				"  `createdAt` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),\n" +
				"  PRIMARY KEY (`id`),\n" +
				"  UNIQUE INDEX `User_email_key` (`email`)\n" +
				") DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		},
		{
			name: "no unique columns",
			table: migrate.CreateTable{
				Name:       "Post",
				Columns:    []migrate.Column{{Name: "id", Type: schema.String}},
				PrimaryKey: "id",
			},
			want: "CREATE TABLE `Post` (\n" +
				"  `id` VARCHAR(191) NOT NULL,\n" +
				"  PRIMARY KEY (`id`)\n" +
				") DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		},
		{
			name: "several unique columns",
			table: migrate.CreateTable{
				Name: "User",
				Columns: []migrate.Column{
					{Name: "id", Type: schema.Int},
					{Name: "email", Type: schema.String},
					{Name: "phone", Type: schema.String},
				},
				PrimaryKey: "id",
				Unique:     []string{"email", "phone"},
			},
			want: "CREATE TABLE `User` (\n" +
				"  `id` INT NOT NULL,\n" +
				"  `email` VARCHAR(191) NOT NULL,\n" +
				"  `phone` VARCHAR(191) NOT NULL,\n" +
				"  PRIMARY KEY (`id`),\n" +
				"  UNIQUE INDEX `User_email_key` (`email`),\n" +
				"  UNIQUE INDEX `User_phone_key` (`phone`)\n" +
				") DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		},
		{
			name: "string default is escaped",
			table: migrate.CreateTable{
				Name: "Note",
				Columns: []migrate.Column{
					{Name: "id", Type: schema.Int},
					{Name: "text", Type: schema.String, Default: &schema.Default{Kind: schema.DefaultString, Value: `it's a \ test`}},
				},
				PrimaryKey: "id",
			},
			want: "CREATE TABLE `Note` (\n" +
				"  `id` INT NOT NULL,\n" +
				"  `text` VARCHAR(191) NOT NULL DEFAULT 'it''s a \\\\ test',\n" +
				"  PRIMARY KEY (`id`)\n" +
				") DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Dialect{}.CreateTable(tt.table)
			if err != nil {
				t.Fatalf("CreateTable returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("CreateTable\n got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestCreateTableErrors(t *testing.T) {
	tests := []struct {
		name    string
		column  migrate.Column
		wantErr string
	}{
		{
			name:    "unsupported type",
			column:  migrate.Column{Name: "price", Type: schema.FieldType("Float")},
			wantErr: "table T: column price: unsupported type Float",
		},
		{
			name:    "unsupported default function",
			column:  migrate.Column{Name: "id", Type: schema.String, Default: &schema.Default{Kind: schema.DefaultFunc, Value: "uuid"}},
			wantErr: "table T: column id: unsupported default function uuid()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := migrate.CreateTable{Name: "T", Columns: []migrate.Column{tt.column}}
			_, err := Dialect{}.CreateTable(table)
			if err == nil {
				t.Fatalf("CreateTable returned no error, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("CreateTable error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

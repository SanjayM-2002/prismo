package migrate

import (
	"reflect"
	"testing"

	"github.com/SanjayM-2002/prismo/schema"
)

func TestCreateTables(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []CreateTable
	}{
		{
			name: "no models",
			src:  "",
			want: nil,
		},
		{
			name: "all column options",
			src: `model User {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  name      String?  @default("anonymous")
  active    Boolean  @default(true)
  createdAt DateTime @default(now())
}`,
			want: []CreateTable{{
				Name: "User",
				Columns: []Column{
					{Name: "id", Type: schema.Int, AutoIncrement: true},
					{Name: "email", Type: schema.String},
					{Name: "name", Type: schema.String, Nullable: true, Default: &schema.Default{Kind: schema.DefaultString, Value: "anonymous"}},
					{Name: "active", Type: schema.Boolean, Default: &schema.Default{Kind: schema.DefaultBool, Value: "true"}},
					{Name: "createdAt", Type: schema.DateTime, Default: &schema.Default{Kind: schema.DefaultFunc, Value: "now"}},
				},
				PrimaryKey: "id",
				Unique:     []string{"email"},
			}},
		},
		{
			name: "several models keep schema order",
			src: `model User {
  id    Int    @id
  email String @unique
  phone String @unique
}

model Post {
  id    String @id
  title String
}`,
			want: []CreateTable{
				{
					Name: "User",
					Columns: []Column{
						{Name: "id", Type: schema.Int},
						{Name: "email", Type: schema.String},
						{Name: "phone", Type: schema.String},
					},
					PrimaryKey: "id",
					Unique:     []string{"email", "phone"},
				},
				{
					Name: "Post",
					Columns: []Column{
						{Name: "id", Type: schema.String},
						{Name: "title", Type: schema.String},
					},
					PrimaryKey: "id",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := schema.Parse(tt.src)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}

			got := CreateTables(s)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateTables\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

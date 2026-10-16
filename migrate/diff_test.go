package migrate

import (
	"reflect"
	"testing"

	"github.com/SanjayM-2002/prismo/schema"
)

var (
	userTable = Table{
		Name: "User",
		Columns: []Column{
			{Name: "id", Type: schema.Int, AutoIncrement: true},
			{Name: "email", Type: schema.String},
		},
		PrimaryKey: "id",
		Unique:     []string{"email"},
	}
	postTable = Table{
		Name:       "Post",
		Columns:    []Column{{Name: "id", Type: schema.Int}},
		PrimaryKey: "id",
	}
	oldTable = Table{
		Name:       "Old",
		Columns:    []Column{{Name: "id", Type: schema.Int}},
		PrimaryKey: "id",
	}
)

func TestDiffTables(t *testing.T) {
	tests := []struct {
		name string
		want []Table
		have []Table
		diff []Step
	}{
		{
			name: "in sync",
			want: []Table{userTable, postTable},
			have: []Table{postTable, userTable},
			diff: nil,
		},
		{
			name: "empty database",
			want: []Table{userTable, postTable},
			have: nil,
			diff: []Step{CreateTable{Table: userTable}, CreateTable{Table: postTable}},
		},
		{
			name: "model removed",
			want: []Table{userTable},
			have: []Table{postTable, userTable},
			diff: []Step{DropTable{Name: "Post"}},
		},
		{
			name: "drops come before creates",
			want: []Table{userTable, postTable},
			have: []Table{oldTable, userTable},
			diff: []Step{DropTable{Name: "Old"}, CreateTable{Table: postTable}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Diff(tt.want, tt.have)
			if err != nil {
				t.Fatalf("Diff returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.diff) {
				t.Errorf("Diff\n got: %+v\nwant: %+v", got, tt.diff)
			}
		})
	}
}

// withColumns returns a copy of t with different columns.
func withColumns(t Table, cols ...Column) Table {
	t.Columns = cols
	return t
}

func TestDiffColumns(t *testing.T) {
	id := Column{Name: "id", Type: schema.Int, AutoIncrement: true}
	email := Column{Name: "email", Type: schema.String}
	age := Column{Name: "age", Type: schema.Int, Nullable: true}

	user := withColumns(userTable, id, email)
	userWithAge := withColumns(userTable, id, email, age)
	userWithRows := user
	userWithRows.HasRows = true

	tests := []struct {
		name string
		want []Table
		have []Table
		diff []Step
	}{
		{
			name: "column added",
			want: []Table{userWithAge},
			have: []Table{user},
			diff: []Step{AddColumn{Table: "User", Column: age}},
		},
		{
			name: "column removed",
			want: []Table{user},
			have: []Table{userWithAge},
			diff: []Step{DropColumn{Table: "User", Column: "age"}},
		},
		{
			name: "column order doesn't matter",
			want: []Table{withColumns(userTable, email, id)},
			have: []Table{user},
			diff: nil,
		},
		{
			name: "table with rows is passed on to the steps",
			want: []Table{userWithAge},
			have: []Table{userWithRows},
			diff: []Step{AddColumn{Table: "User", Column: age, HasRows: true}},
		},
		{
			name: "column changed",
			want: []Table{withColumns(userTable, id, email, Column{Name: "age", Type: schema.Int})},
			have: []Table{userWithAge},
			diff: []Step{AlterColumn{Table: "User", From: age, To: Column{Name: "age", Type: schema.Int}}},
		},
		{
			name: "number defaults written differently are the same",
			want: []Table{withColumns(userTable, id, Column{Name: "n", Type: schema.Int, Default: &schema.Default{Kind: schema.DefaultNumber, Value: "007"}})},
			have: []Table{withColumns(userTable, id, Column{Name: "n", Type: schema.Int, Default: &schema.Default{Kind: schema.DefaultNumber, Value: "7"}})},
			diff: nil,
		},
		{
			name: "drops come first",
			want: []Table{withColumns(userTable, id, age), postTable},
			have: []Table{user, oldTable},
			diff: []Step{
				DropTable{Name: "Old"},
				DropColumn{Table: "User", Column: "email"},
				CreateTable{Table: postTable},
				AddColumn{Table: "User", Column: age},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Diff(tt.want, tt.have)
			if err != nil {
				t.Fatalf("Diff returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.diff) {
				t.Errorf("Diff\n got: %+v\nwant: %+v", got, tt.diff)
			}
		})
	}
}

func TestDiffUnique(t *testing.T) {
	id := Column{Name: "id", Type: schema.Int, AutoIncrement: true}
	email := Column{Name: "email", Type: schema.String}
	phone := Column{Name: "phone", Type: schema.String}

	table := func(unique []string, cols ...Column) Table {
		return Table{Name: "User", Columns: cols, PrimaryKey: "id", Unique: unique}
	}

	tests := []struct {
		name string
		want []Table
		have []Table
		diff []Step
	}{
		{
			name: "unique added",
			want: []Table{table([]string{"email"}, id, email)},
			have: []Table{table(nil, id, email)},
			diff: []Step{AddUnique{Table: "User", Column: "email"}},
		},
		{
			name: "unique removed",
			want: []Table{table(nil, id, email)},
			have: []Table{table([]string{"email"}, id, email)},
			diff: []Step{DropUnique{Table: "User", Column: "email"}},
		},
		{
			name: "unique column added: column first, then index",
			want: []Table{table([]string{"email", "phone"}, id, email, phone)},
			have: []Table{table([]string{"email"}, id, email)},
			diff: []Step{AddColumn{Table: "User", Column: phone}, AddUnique{Table: "User", Column: "phone"}},
		},
		{
			name: "unique column removed: index first, then column",
			want: []Table{table([]string{"email"}, id, email)},
			have: []Table{table([]string{"email", "phone"}, id, email, phone)},
			diff: []Step{DropUnique{Table: "User", Column: "phone"}, DropColumn{Table: "User", Column: "phone"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Diff(tt.want, tt.have)
			if err != nil {
				t.Fatalf("Diff returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.diff) {
				t.Errorf("Diff\n got: %+v\nwant: %+v", got, tt.diff)
			}
		})
	}
}

func TestDiffPrimaryKeyChange(t *testing.T) {
	want := Table{Name: "User", Columns: []Column{{Name: "code", Type: schema.String}}, PrimaryKey: "code"}
	have := Table{Name: "User", Columns: []Column{{Name: "id", Type: schema.Int}}, PrimaryKey: "id"}

	_, err := Diff([]Table{want}, []Table{have})
	wantErr := `table User: changing the primary key from "id" to "code" is not supported yet`
	if err == nil || err.Error() != wantErr {
		t.Errorf("Diff error = %v, want %q", err, wantErr)
	}
}

func TestStepText(t *testing.T) {
	required := Column{Name: "age", Type: schema.Int}
	optional := Column{Name: "age", Type: schema.Int, Nullable: true}
	withDefault := Column{Name: "age", Type: schema.Int, Default: &schema.Default{Kind: schema.DefaultNumber, Value: "0"}}

	tests := []struct {
		step     Step
		describe string
		warning  string
	}{
		{step: CreateTable{Table: postTable}, describe: "Create table Post"},
		{step: DropTable{Name: "Old"}, describe: "Drop table Old"},
		{step: DropTable{Name: "Old", HasRows: true}, describe: "Drop table Old",
			warning: "Table Old and all its rows will be dropped"},
		{step: AddColumn{Table: "User", Column: required}, describe: "Add column User.age"},
		{step: AddColumn{Table: "User", Column: optional, HasRows: true}, describe: "Add column User.age"},
		{step: AddColumn{Table: "User", Column: withDefault, HasRows: true}, describe: "Add column User.age"},
		{step: AddColumn{Table: "User", Column: required, HasRows: true}, describe: "Add column User.age",
			warning: "Required column User.age has no default, so existing rows get an empty value (0, '' or a zero date)"},
		{step: AlterColumn{Table: "User", From: optional, To: required}, describe: "Change column User.age (optional -> required)"},
		{step: AlterColumn{Table: "User", From: optional, To: required, HasRows: true}, describe: "Change column User.age (optional -> required)",
			warning: "Column User.age becomes required; the change fails if any row has NULL in it"},
		{step: AlterColumn{Table: "User", From: required, To: withDefault, HasRows: true}, describe: "Change column User.age (default none -> 0)"},
		{
			step: AlterColumn{
				Table:   "User",
				From:    Column{Name: "age", Type: schema.FieldType("varchar(255)"), Nullable: true, Default: &schema.Default{Kind: schema.DefaultString, Value: "x"}},
				To:      Column{Name: "age", Type: schema.Int, AutoIncrement: true},
				HasRows: true,
			},
			describe: `Change column User.age (type varchar(255) -> Int, optional -> required, default "x" -> none, add autoincrement())`,
			warning: "Column User.age changes type from varchar(255) to Int; values that don't convert are lost or make the change fail; " +
				"Column User.age becomes required; the change fails if any row has NULL in it",
		},
		{step: AlterColumn{Table: "User", From: Column{Name: "id", Type: schema.Int, AutoIncrement: true}, To: Column{Name: "id", Type: schema.Int}},
			describe: "Change column User.id (remove autoincrement())"},
		{step: AddUnique{Table: "User", Column: "email"}, describe: "Add unique index on User.email"},
		{step: DropUnique{Table: "User", Column: "email"}, describe: "Drop unique index on User.email"},
		{step: DropColumn{Table: "User", Column: "age"}, describe: "Drop column User.age"},
		{step: DropColumn{Table: "User", Column: "age", HasRows: true}, describe: "Drop column User.age",
			warning: "Column User.age and all its values will be dropped"},
	}

	for _, tt := range tests {
		if got := tt.step.Describe(); got != tt.describe {
			t.Errorf("Describe() = %q, want %q", got, tt.describe)
		}
		if got := tt.step.Warning(); got != tt.warning {
			t.Errorf("%+v: Warning() = %q, want %q", tt.step, got, tt.warning)
		}
	}
}

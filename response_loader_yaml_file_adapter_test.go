package mocka

import (
	"encoding/xml"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castingcode/mocaprotocol"
	. "github.com/smartystreets/goconvey/convey"
)

// writeTestFile creates a file inside dir and fails the test if it cannot.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeTestFile %s: %v", name, err)
	}
}

// loaderFor creates a FileResponseLoader rooted at dir.
func loaderFor(dir string) *FileResponseLoader {
	return NewFileResponseLoader(dir)
}

func TestFileResponseLoader_PublishData(t *testing.T) {

	Convey("FileResponseLoader — publish_data entries", t, func() {

		Convey("Given a publish_data entry with context keys and a results file", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "result.xml", `<moca-results><metadata/><data/></moca-results>`)
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match:
      type: publish_data
      inner: "do thing"
      context:
        wh_id: MHE
        prt_client_id: ACME
    response:
      status: 0
      results: result.xml
`)
			entries, err := loaderFor(dir).Load()

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then one entry is returned", func() {
				So(entries, ShouldHaveLength, 1)
			})

			Convey("Then the entry has the correct match type", func() {
				So(entries[0].MatchType, ShouldEqual, MatchTypePublishData)
			})

			Convey("Then the inner command is normalized", func() {
				So(entries[0].Inner, ShouldEqual, "do thing")
			})

			Convey("Then context values are lowercased for case-insensitive matching", func() {
				So(entries[0].Context["wh_id"], ShouldEqual, "mhe")
				So(entries[0].Context["prt_client_id"], ShouldEqual, "acme")
			})

			Convey("Then the result XML is loaded from the referenced file", func() {
				So(entries[0].ResultSet, ShouldContainSubstring, "moca-results")
			})
		})

		Convey("Given a publish_data entry with no context (generic fallback)", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match:
      type: publish_data
      inner: "do thing"
    response:
      status: 510
      message: No Data Found
`)
			entries, err := loaderFor(dir).Load()

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the entry has an empty context map", func() {
				So(entries[0].Context, ShouldBeEmpty)
			})

			Convey("Then the inner command is set", func() {
				So(entries[0].Inner, ShouldEqual, "do thing")
			})

			Convey("Then the status and message are loaded", func() {
				So(entries[0].StatusCode, ShouldEqual, StatusSrvNoDataFound)
				So(entries[0].Message, ShouldEqual, "No Data Found")
			})
		})

		Convey("Given a publish_data entry whose inner command has inconsistent casing and extra whitespace", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match:
      type: publish_data
      inner: "DO  THING"
    response:
      status: 0
`)
			entries, err := loaderFor(dir).Load()

			Convey("Then the inner command is normalized at load time", func() {
				So(err, ShouldBeNil)
				So(entries[0].Inner, ShouldEqual, "do thing")
			})
		})

		Convey("Given a publish_data results file that does not exist", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match:
      type: publish_data
      inner: "do thing"
    response:
      status: 0
      results: missing.xml
`)
			_, err := loaderFor(dir).Load()

			Convey("Then an error is returned identifying the missing file", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "missing.xml")
			})
		})
	})
}

func TestFileResponseLoader_ExactWithQueryFile(t *testing.T) {

	Convey("FileResponseLoader — exact match with query_file", t, func() {

		Convey("Given an exact entry whose query is stored in a separate file", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "long-query.txt", "list warehouses where wh_id = 'MHE'")
			writeTestFile(t, dir, "result.xml", `<moca-results><metadata/><data/></moca-results>`)
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match:
      type: exact
      query_file: long-query.txt
    response:
      status: 0
      results: result.xml
`)
			entries, err := loaderFor(dir).Load()

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the query is read from the file and normalized", func() {
				So(entries[0].Query, ShouldEqual, normalizeQuery("list warehouses where wh_id = 'MHE'"))
			})

			Convey("Then the result XML is loaded", func() {
				So(entries[0].ResultSet, ShouldContainSubstring, "moca-results")
			})
		})

		Convey("Given a query file with multiple lines and extra indentation", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "long-query.txt", `
				list warehouses
				  where wh_id = 'MHE'
				  and wh_name = 'Main Warehouse'
			`)
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match:
      type: exact
      query_file: long-query.txt
    response:
      status: 0
`)
			entries, err := loaderFor(dir).Load()

			Convey("Then the multi-line query is collapsed to a single normalized string", func() {
				So(err, ShouldBeNil)
				So(entries[0].Query, ShouldEqual, "list warehouses where wh_id = 'mhe' and wh_name = 'main warehouse'")
			})
		})

		Convey("Given a query_file path that does not exist", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match:
      type: exact
      query_file: no-such-file.txt
    response:
      status: 0
`)
			_, err := loaderFor(dir).Load()

			Convey("Then an error is returned identifying the missing file", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "no-such-file.txt")
			})
		})
	})
}

func TestFileResponseLoader_Validation(t *testing.T) {

	Convey("FileResponseLoader — invalid entries are rejected", t, func() {

		cases := []struct {
			name    string
			entry   string
			wantErr string
		}{
			{"a misspelled match type", "match: {type: exat, query: ping}", `entry 1: unknown match type "exat"`},
			{"a missing match type", "match: {query: ping}", `entry 1: unknown match type ""`},
			{"a misspelled field name", "match: {type: exact, querry: ping}", `[2:26] unknown field "querry"`},
			{"both query and query_file", "match: {type: exact, query: ping, query_file: q.txt}", "entry 1: exact match sets both query and query_file"},
			{"an exact match without a query", "match: {type: exact}", "entry 1: exact match needs a non-empty query"},
			{"a prefix match without a prefix", "match: {type: prefix}", "entry 1: prefix match needs a non-empty prefix"},
			{"a publish_data match without inner", "match: {type: publish_data, context: {wh_id: MHE}}", "entry 1: publish_data match needs a non-empty inner"},
			{"a field that belongs to another type", "match: {type: prefix, prefix: list, query: list}", "entry 1: query is not used by prefix matches"},
		}

		for _, c := range cases {
			Convey("Given "+c.name, func() {
				dir := t.TempDir()
				writeTestFile(t, dir, "responses.yml", "responses:\n  - "+c.entry+"\n    response: {status: 0}\n")
				_, err := loaderFor(dir).Load()

				Convey("Then an error names the file, the location, and the problem", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "loading responses.yml: ")
					So(err.Error(), ShouldContainSubstring, c.wantErr)
				})
			})
		}

		Convey("Given two publish_data entries with the same inner and context in a different order and case", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match: {type: publish_data, inner: do thing, context: {wh_id: MHE, client: ACME}}
    response: {status: 0}
  - match: {type: publish_data, inner: DO THING, context: {client: acme, wh_id: mhe}}
    response: {status: 0}
`)
			_, err := loaderFor(dir).Load()

			Convey("Then a duplicate error points at the first definition", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "entry 2: duplicate publish_data match (first defined in responses.yml entry 1)")
			})
		})

		Convey("Given publish_data entries with the same inner but different context", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match: {type: publish_data, inner: do thing, context: {wh_id: MHE}}
    response: {status: 0}
  - match: {type: publish_data, inner: do thing, context: {wh_id: WMD1}}
    response: {status: 0}
  - match: {type: publish_data, inner: do thing}
    response: {status: 0}
`)
			entries, err := loaderFor(dir).Load()

			Convey("Then they are not duplicates", func() {
				So(err, ShouldBeNil)
				So(entries, ShouldHaveLength, 3)
			})
		})
	})
}

func TestFileResponseLoader_MultipleFiles(t *testing.T) {

	Convey("FileResponseLoader — multiple response files", t, func() {

		Convey("Given responses.yml and two files in responses.d", func() {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "responses.d"), 0755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, dir, "shared.xml", `<moca-results/>`)
			writeTestFile(t, dir, "responses.yml", "responses:\n  - match: {type: exact, query: a}\n    response: {status: 1}\n")
			writeTestFile(t, dir, "responses.d/20-orders.yaml", "responses:\n  - match: {type: exact, query: c}\n    response: {status: 3}\n")
			writeTestFile(t, dir, "responses.d/10-warehouses.yml", "responses:\n  - match: {type: exact, query: b}\n    response: {status: 2, results: shared.xml}\n")
			writeTestFile(t, dir, "responses.d/notes.txt", "not yaml")
			entries, err := loaderFor(dir).Load()

			Convey("Then responses.yml loads first, then responses.d files in name order", func() {
				So(err, ShouldBeNil)
				So(entries, ShouldHaveLength, 3)
				So(entries[0].Query, ShouldEqual, "a")
				So(entries[1].Query, ShouldEqual, "b")
				So(entries[2].Query, ShouldEqual, "c")
			})

			Convey("Then results paths in responses.d are relative to the responses folder", func() {
				So(entries[1].ResultSet, ShouldEqual, "<moca-results/>")
			})
		})

		Convey("Given the same exact query in responses.yml and in responses.d", func() {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "responses.d"), 0755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, dir, "responses.yml", "responses:\n  - match: {type: exact, query: List  Warehouses}\n    response: {status: 0}\n")
			writeTestFile(t, dir, "responses.d/more.yml", "responses:\n  - match: {type: exact, query: list warehouses}\n    response: {status: 0}\n")
			_, err := loaderFor(dir).Load()

			Convey("Then a duplicate error names both locations", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "loading responses.d/more.yml: entry 1: duplicate")
				So(err.Error(), ShouldContainSubstring, "first defined in responses.yml entry 1")
			})
		})

		Convey("Given only responses.yaml", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yaml", "responses:\n  - match: {type: prefix, prefix: list}\n    response: {status: 0}\n")
			entries, err := loaderFor(dir).Load()

			Convey("Then it is loaded", func() {
				So(err, ShouldBeNil)
				So(entries, ShouldHaveLength, 1)
			})
		})

		Convey("Given both responses.yml and responses.yaml", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", "responses: []\n")
			writeTestFile(t, dir, "responses.yaml", "responses: []\n")
			_, err := loaderFor(dir).Load()

			Convey("Then an error asks to keep only one", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "keep only one")
			})
		})

		Convey("Given no response files at all", func() {
			entries, err := loaderFor(t.TempDir()).Load()

			Convey("Then the registry is empty without an error", func() {
				So(err, ShouldBeNil)
				So(entries, ShouldBeEmpty)
			})
		})
	})
}

func TestFileResponseLoader_CSVResults(t *testing.T) {

	// loadCSV loads one exact entry whose results file holds csv, and returns
	// the parsed result set or the load error.
	loadCSV := func(t *testing.T, name, csv string) (mocaprotocol.MocaResults, error) {
		dir := t.TempDir()
		writeTestFile(t, dir, name, csv)
		writeTestFile(t, dir, "responses.yml", "responses:\n  - match: {type: exact, query: q}\n    response: {status: 0, results: "+name+"}\n")
		entries, err := loaderFor(dir).Load()
		if err != nil {
			return mocaprotocol.MocaResults{}, err
		}
		var results mocaprotocol.MocaResults
		if err := xml.Unmarshal([]byte(entries[0].ResultSet), &results); err != nil {
			t.Fatalf("generated XML does not parse: %v\n%s", err, entries[0].ResultSet)
		}
		if !strings.HasPrefix(entries[0].ResultSet, "<moca-results>") {
			t.Fatalf("generated XML is not a moca-results fragment: %s", entries[0].ResultSet)
		}
		return results, nil
	}

	Convey("FileResponseLoader — CSV result files", t, func() {

		Convey("Given a CSV file with typed columns, quoted values, and empty cells", func() {
			results, err := loadCSV(t, "wh.csv", "\uFEFFwh_id:S:32, wh_name ,qty:i,cost:F:8\n"+
				"WMD1,\"Main, \"\"North\"\"\",5,1.25\n"+
				"WMD2,,-3,\n")

			Convey("Then the header declares the columns with defaults for missing parts", func() {
				So(err, ShouldBeNil)
				So(results.Metadata.Columns, ShouldResemble, []mocaprotocol.Column{
					{Name: "wh_id", Type: "S", Length: "32", Nullable: "true"},
					{Name: "wh_name", Type: "S", Length: "0", Nullable: "true"},
					{Name: "qty", Type: "I", Length: "0", Nullable: "true"},
					{Name: "cost", Type: "F", Length: "8", Nullable: "true"},
				})
			})

			Convey("Then each row becomes a data row, quoted values keep commas and quotes, and empty cells are null", func() {
				So(results.Data.Rows, ShouldResemble, []mocaprotocol.Row{
					{Fields: []mocaprotocol.Field{{Value: "WMD1"}, {Value: `Main, "North"`}, {Value: "5"}, {Value: "1.25"}}},
					{Fields: []mocaprotocol.Field{{Value: "WMD2"}, {Null: "true"}, {Value: "-3"}, {Null: "true"}}},
				})
			})
		})

		Convey("Given a CSV file with only a header row and an upper-case extension", func() {
			results, err := loadCSV(t, "EMPTY.CSV", "wh_id\n")

			Convey("Then the result has its columns and no rows", func() {
				So(err, ShouldBeNil)
				So(results.Metadata.Columns, ShouldHaveLength, 1)
				So(results.Data.Rows, ShouldBeEmpty)
			})
		})

		cases := []struct{ name, csv, wantErr string }{
			{"an empty file", "", "missing header row"},
			{"an unknown column type", "a:X\n1\n", `header column 1: "a:X" has unknown type "X"`},
			{"a length that is not a number", "a:S:wide\n", "has a length that is not a number"},
			{"a header cell with no name", "a,:I\n", "header column 2"},
			{"a non-integer value in an I column", "id,qty:I\nA,1\nB,1.5\n", `line 3, column qty: "1.5" is not an integer`},
			{"a non-numeric value in an F column", "cost:F\nabc\n", `line 2, column cost: "abc" is not a number`},
			{"a row with the wrong number of fields", "a,b\n1,2\n3\n", "line 3"},
		}
		for _, c := range cases {
			Convey("Given "+c.name, func() {
				_, err := loadCSV(t, "r.csv", c.csv)

				Convey("Then loading fails naming the file and the problem", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "responses.yml: entry 1: results file r.csv: ")
					So(err.Error(), ShouldContainSubstring, c.wantErr)
				})
			})
		}
	})
}

func TestFileResponseLoader_Command(t *testing.T) {

	Convey("FileResponseLoader — command entries", t, func() {

		Convey("Given a command entry with mixed-case keys, quoted text, and a number", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match:
      type: command
      command: "LIST  Warehouses"
      args: {WH_ID: MHE, qty: 5, txt: "Salt  and pepper=1"}
    response: {status: 0}
`)
			entries, err := loaderFor(dir).Load()

			Convey("Then the command, keys, and values are normalized like an incoming query", func() {
				So(err, ShouldBeNil)
				So(entries[0].Command, ShouldEqual, "list warehouses")
				So(entries[0].Args, ShouldResemble, map[string]string{"wh_id": "mhe", "qty": "5", "txt": "salt and pepper = 1"})
			})

			Convey("Then a query with those arguments in another order resolves to it", func() {
				q := normalizeQuery("list warehouses where txt = 'salt and pepper=1' and qty = 5 and wh_id = 'MHE'")
				_, e := resolveQuery(q, entries, slog.Default())
				So(e, ShouldNotBeNil)
				So(e.Source, ShouldEqual, "responses.yml entry 1")
			})
		})

		cases := []struct{ name, match, wantErr string }{
			{"a command with a where clause", "{type: command, command: \"list warehouses where wh_id = 'MHE'\"}", `command "list warehouses where wh_id = 'MHE'" must be only a command name`},
			{"a command that is not a command name", "{type: command, command: \"[select 1]\"}", `command "[select 1]" must be only a command name`},
			{"a missing command", "{type: command, args: {a: b}}", "command match needs a non-empty command"},
			{"an args key that is not an argument name", "{type: command, command: list x, args: {\"wh id\": b}}", `args key "wh id" is not an argument name`},
			{"args on another match type", "{type: prefix, prefix: list, args: {a: b}}", "args is not used by prefix matches"},
		}
		for _, c := range cases {
			Convey("Given "+c.name, func() {
				dir := t.TempDir()
				writeTestFile(t, dir, "responses.yml", "responses:\n  - match: "+c.match+"\n    response: {status: 0}\n")
				_, err := loaderFor(dir).Load()

				Convey("Then loading fails with the problem", func() {
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "entry 1: "+c.wantErr)
				})
			})
		}

		Convey("Given two command entries with the same args in a different order and case", func() {
			dir := t.TempDir()
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match: {type: command, command: list x, args: {a: "1", b: "2"}}
    response: {status: 0}
  - match: {type: command, command: LIST X, args: {B: "2", A: "1"}}
    response: {status: 0}
`)
			_, err := loaderFor(dir).Load()

			Convey("Then they are duplicates", func() {
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "entry 2: duplicate command match (first defined in responses.yml entry 1)")
			})
		})
	})
}

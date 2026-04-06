package mocka

import (
	"os"
	"path/filepath"
	"testing"

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

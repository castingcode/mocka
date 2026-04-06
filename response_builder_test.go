package mocka

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestResponseBuilder(t *testing.T) {

	Convey("ResponseBuilder", t, func() {

		Convey("NewResponse sets the status code", func() {
			resp := NewResponse(StatusOK).Build()
			So(resp.StatusCode, ShouldEqual, StatusOK)
		})

		Convey("WithMessage sets the message", func() {
			resp := NewResponse(StatusSrvNoDataFound).WithMessage("No Data Found").Build()
			So(resp.StatusCode, ShouldEqual, StatusSrvNoDataFound)
			So(resp.Message, ShouldEqual, "No Data Found")
		})

		Convey("WithResultSet sets the result XML directly", func() {
			xml := `<moca-results><metadata/><data/></moca-results>`
			resp := NewResponse(StatusOK).WithResultSet(xml).Build()
			So(resp.ResultSet, ShouldEqual, xml)
		})

		Convey("WithResultSetFromFile reads XML from disk", func() {
			dir := t.TempDir()
			content := `<moca-results><metadata/><data/></moca-results>`
			if err := os.WriteFile(filepath.Join(dir, "result.xml"), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			rb, err := NewResponse(StatusOK).WithResultSetFromFile(filepath.Join(dir, "result.xml"))

			Convey("Then no error is returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the result set is the trimmed file contents", func() {
				So(rb.Build().ResultSet, ShouldEqual, content)
			})
		})

		Convey("WithResultSetFromFile with whitespace-padded file trims the content", func() {
			dir := t.TempDir()
			content := `<moca-results><metadata/><data/></moca-results>`
			if err := os.WriteFile(filepath.Join(dir, "result.xml"), []byte("\n  "+content+"  \n"), 0644); err != nil {
				t.Fatal(err)
			}

			rb, err := NewResponse(StatusOK).WithResultSetFromFile(filepath.Join(dir, "result.xml"))
			So(err, ShouldBeNil)
			So(rb.Build().ResultSet, ShouldEqual, content)
		})

		Convey("WithResultSetFromFile returns an error when the file does not exist", func() {
			_, err := NewResponse(StatusOK).WithResultSetFromFile("/no/such/file.xml")
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "no/such/file.xml")
		})

		Convey("Builder methods are chainable and independent", func() {
			xml := `<moca-results/>`
			resp := NewResponse(StatusDBError).
				WithMessage("DB Error").
				WithResultSet(xml).
				Build()
			So(resp.StatusCode, ShouldEqual, StatusDBError)
			So(resp.Message, ShouldEqual, "DB Error")
			So(resp.ResultSet, ShouldEqual, xml)
		})
	})
}

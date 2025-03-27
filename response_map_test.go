package mocka

import (
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetResponse(t *testing.T) {
	Convey("Given I have ResponseLookup", t, func() {
		responseDirectory := t.TempDir()
		Convey("When the files do not exist", func() {
			lookup, err := NewResponseLookup(WithDataFolder(responseDirectory))
			Convey("Then an error should be returned", func() {
				So(lookup, ShouldBeNil)
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When the files are empty", func() {
			f, err := os.Create(fmt.Sprintf("%s/responses.yml", responseDirectory))
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
			uf, err := os.Create(fmt.Sprintf("%s/user_responses.yml", responseDirectory))
			if err != nil {
				t.Fatal(err)
			}
			defer uf.Close()
			lookup, err := NewResponseLookup(WithDataFolder(responseDirectory))
			Convey("Then no error should be returned", func() {
				So(err, ShouldBeNil)
				So(lookup, ShouldNotBeNil)
			})
			Convey("And getting a response for a command should be a moca error", func() {
				response := lookup.GetResponse("super", "noop")
				So(response.StatusCode, ShouldEqual, StatusCommandNotFound)
				So(response.ResultSet, ShouldBeEmpty)
				So(response.Message, ShouldEqual, "Command (noop) not found")
			})

		})

		Convey("When the user file is invalid", func() {
			data, err := os.ReadFile("testdata/responses.yml")
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(fmt.Sprintf("%s/responses.yml", responseDirectory), []byte(data), 0644)
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(fmt.Sprintf("%s/user_responses.yml", responseDirectory), []byte("bad contents"), 0644)
			if err != nil {
				t.Fatal(err)
			}
			_, err = NewResponseLookup(WithDataFolder(responseDirectory))
			Convey("Then an error should be returned", func() {
				So(err, ShouldNotBeNil)
			})

		})

		Convey("When the command is mapped for all users", func() {
			data, err := os.ReadFile("testdata/responses.yml")
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(fmt.Sprintf("%s/responses.yml", responseDirectory), []byte(data), 0644)
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(fmt.Sprintf("%s/user_responses.yml", responseDirectory))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			lookup, err := NewResponseLookup(WithDataFolder(responseDirectory))
			Convey("Then no error should be returned when creating the lookup", func() {
				So(err, ShouldBeNil)
				So(lookup, ShouldNotBeNil)
			})
			Convey("And the command lookup should return a non-user specific response", func() {
				response := lookup.GetResponse("super", "publish data")
				want := `<moca-results>
        <metadata>
            <column name="line" type="I" length="0" nullable="true"/>
            <column name="text" type="S" length="0" nullable="true"/>
        </metadata>
        <data>
            <row>
                <field>0</field>
                <field>hello</field>
            </row>
        </data>
    </moca-results>`
				So(response.StatusCode, ShouldEqual, StatusOK)
				So(compact(response.ResultSet), ShouldEqual, compact(want))
				So(response.Message, ShouldBeEmpty)

				response = lookup.GetResponse("super", "list warehouses")
				So(response.StatusCode, ShouldEqual, StatusSrvNoDataFound)
				So(response.Message, ShouldEqual, "No Data Found")
				So(response.ResultSet, ShouldBeEmpty)

			})
		})

		Convey("When the command is mapped for a specific user", func() {
			data, err := os.ReadFile("testdata/responses.yml")
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(fmt.Sprintf("%s/responses.yml", responseDirectory), []byte(data), 0644)
			if err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile("testdata/user_responses.yml")
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(fmt.Sprintf("%s/user_responses.yml", responseDirectory), []byte(data), 0644)
			if err != nil {
				t.Fatal(err)
			}
			lookup, err := NewResponseLookup(WithDataFolder(responseDirectory))
			Convey("Then no error should be returned when creating the lookup", func() {
				So(err, ShouldBeNil)
				So(lookup, ShouldNotBeNil)
			})
			Convey("And the command lookup should return the generic response when the user is not mapped to a user specific response", func() {
				response := lookup.GetResponse("jjazwiec", "publish data")
				want := `<moca-results>
        <metadata>
            <column name="line" type="I" length="0" nullable="true"/>
            <column name="text" type="S" length="0" nullable="true"/>
        </metadata>
        <data>
            <row>
                <field>0</field>
                <field>hello</field>
            </row>
        </data>
    </moca-results>`
				So(response.StatusCode, ShouldEqual, StatusOK)
				So(compact(response.ResultSet), ShouldEqual, compact(want))
				So(response.Message, ShouldBeEmpty)

				response = lookup.GetResponse("jjazwiec", "list warehouses")
				So(response.StatusCode, ShouldEqual, StatusSrvNoDataFound)
				So(response.Message, ShouldEqual, "No Data Found")
				So(response.ResultSet, ShouldBeEmpty)
			})
			Convey("And the command lookup should return the specific response when the user is mapped to a user specific response", func() {
				response := lookup.GetResponse("super", "publish data")
				want := `<moca-results>
        <metadata>
            <column name="line" type="I" length="0" nullable="true"/>
            <column name="text" type="S" length="0" nullable="true"/>
        </metadata>
        <data>
            <row>
                <field>0</field>
                <field>world</field>
            </row>
        </data>
    </moca-results>`
				So(response.StatusCode, ShouldEqual, StatusOK)
				So(compact(response.ResultSet), ShouldEqual, compact(want))
				So(response.Message, ShouldBeEmpty)

				response = lookup.GetResponse("super", "list warehouses")
				So(response.StatusCode, ShouldEqual, StatusSrvNoDataFound)
				So(response.Message, ShouldEqual, "No Data Found")
				So(response.ResultSet, ShouldBeEmpty)

			})
		})

	})
}

// standardize white space for testing
func compact(s string) string {
	tokens := strings.Fields(s)
	return strings.Join(tokens, " ")
}

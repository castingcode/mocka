package mocka

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestResponseMapFromFile(t *testing.T) {
	Convey("Given I have a MockDataFile", t, func() {
		path := "testdata/responses.yml"

		Convey("When I read it into a ResponseMap", func() {
			responseMap, err := ResponseMapFromFile(path)

			Convey("Then there should be no error", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the ResponseMap should be correct", func() {
				So(responseMap["publish data"].StatusCode, ShouldEqual, 0)
				So(responseMap["publish data"].Message, ShouldBeEmpty)
				So(compact(responseMap["publish data"].ResultSet), ShouldEqual, compact(`<moca-results>
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
          </moca-results>`))

				So(responseMap["list warehouses"].StatusCode, ShouldEqual, 510)
				So(responseMap["list warehouses"].Message, ShouldEqual, "No Data Found")
				So(responseMap["list warehouses"].ResultSet, ShouldBeEmpty)
			})
		})
	})
}

func TestUserResponseMapFromFile(t *testing.T) {
	Convey("Given I have a MockUserDataFile", t, func() {
		path := "testdata/user_responses.yml"

		Convey("When I read it into a UserResponseMap", func() {
			responseMap, err := UserResponseMapFromFile(path)

			Convey("Then there should be no error", func() {
				So(err, ShouldBeNil)
			})

			Convey("Then the ResponseMap should be correct", func() {
				So(responseMap["super"]["publish data"].StatusCode, ShouldEqual, 0)
				So(responseMap["super"]["publish data"].Message, ShouldBeEmpty)
				So(compact(responseMap["super"]["publish data"].ResultSet), ShouldEqual, compact(`<moca-results>
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
          </moca-results>`))
			})
		})
	})
}

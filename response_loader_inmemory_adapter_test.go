package mocka

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestInMemoryResponseLoader(t *testing.T) {

	Convey("InMemoryResponseLoader", t, func() {

		Convey("With no options, Load returns an empty slice", func() {
			entries, err := NewInMemoryResponseLoader().Load()
			So(err, ShouldBeNil)
			So(entries, ShouldBeEmpty)
		})

		Convey("WithEntries appends a pre-built slice of entries", func() {
			prebuilt := []Entry{
				{MatchType: MatchTypeExact, Query: "list warehouses", StatusCode: StatusOK},
				{MatchType: MatchTypeExact, Query: "list users", StatusCode: StatusOK},
			}
			entries, err := NewInMemoryResponseLoader(WithEntries(prebuilt)).Load()
			So(err, ShouldBeNil)
			So(entries, ShouldHaveLength, 2)
		})

		Convey("WithExactMatch", func() {

			Convey("appends an exact-match entry with a normalized query", func() {
				resp := NewResponse(StatusOK).WithResultSet("<data/>").Build()
				entries, _ := NewInMemoryResponseLoader(
					WithExactMatch("List Warehouses Where wh_id = 'MHE'", resp),
				).Load()

				So(entries, ShouldHaveLength, 1)
				So(entries[0].MatchType, ShouldEqual, MatchTypeExact)
				So(entries[0].Query, ShouldEqual, normalizeQuery("list warehouses where wh_id = 'MHE'"))
				So(entries[0].StatusCode, ShouldEqual, StatusOK)
				So(entries[0].ResultSet, ShouldEqual, "<data/>")
			})

			Convey("appends an error-only entry with no result set", func() {
				resp := NewResponse(StatusSrvNoDataFound).WithMessage("No Data Found").Build()
				entries, _ := NewInMemoryResponseLoader(
					WithExactMatch("list things", resp),
				).Load()

				So(entries[0].StatusCode, ShouldEqual, StatusSrvNoDataFound)
				So(entries[0].Message, ShouldEqual, "No Data Found")
				So(entries[0].ResultSet, ShouldBeEmpty)
			})
		})

		Convey("WithPrefixMatch appends a prefix-match entry with a normalized prefix", func() {
			resp := NewResponse(StatusSrvNoDataFound).WithMessage("No Data Found").Build()
			entries, _ := NewInMemoryResponseLoader(
				WithPrefixMatch("List Warehouses", resp),
			).Load()

			So(entries, ShouldHaveLength, 1)
			So(entries[0].MatchType, ShouldEqual, MatchTypePrefix)
			So(entries[0].Prefix, ShouldEqual, "list warehouses")
			So(entries[0].StatusCode, ShouldEqual, StatusSrvNoDataFound)
		})

		Convey("WithPublishDataMatch", func() {

			Convey("appends a generic publish-data entry with a normalized inner command", func() {
				resp := NewResponse(StatusOK).WithResultSet("<data/>").Build()
				entries, _ := NewInMemoryResponseLoader(
					WithPublishDataMatch("DO  THING", resp),
				).Load()

				So(entries, ShouldHaveLength, 1)
				So(entries[0].MatchType, ShouldEqual, MatchTypePublishData)
				So(entries[0].Inner, ShouldEqual, "do thing")
				So(entries[0].Context, ShouldBeEmpty)
			})
		})

		Convey("WithContextualPublishDataMatch", func() {

			Convey("appends a contextual publish-data entry with lowercased context values", func() {
				resp := NewResponse(StatusOK).WithResultSet("<mhe-data/>").Build()
				entries, _ := NewInMemoryResponseLoader(
					WithContextualPublishDataMatch(
						"do thing",
						map[string]string{"wh_id": "MHE", "prt_client_id": "ACME"},
						resp,
					),
				).Load()

				So(entries, ShouldHaveLength, 1)
				So(entries[0].MatchType, ShouldEqual, MatchTypePublishData)
				So(entries[0].Inner, ShouldEqual, "do thing")
				So(entries[0].Context["wh_id"], ShouldEqual, "mhe")
				So(entries[0].Context["prt_client_id"], ShouldEqual, "acme")
			})
		})

		Convey("Multiple options accumulate entries in declaration order", func() {
			entries, _ := NewInMemoryResponseLoader(
				WithExactMatch("list warehouses", NewResponse(StatusOK).Build()),
				WithPrefixMatch("list", NewResponse(StatusSrvNoDataFound).Build()),
				WithPublishDataMatch("do thing", NewResponse(StatusOK).Build()),
			).Load()

			So(entries, ShouldHaveLength, 3)
			So(entries[0].MatchType, ShouldEqual, MatchTypeExact)
			So(entries[1].MatchType, ShouldEqual, MatchTypePrefix)
			So(entries[2].MatchType, ShouldEqual, MatchTypePublishData)
		})

		Convey("WithEntries and sugar options can be combined freely", func() {
			prebuilt := []Entry{
				{MatchType: MatchTypeExact, Query: "list users", StatusCode: StatusOK},
			}
			entries, _ := NewInMemoryResponseLoader(
				WithEntries(prebuilt),
				WithExactMatch("list warehouses", NewResponse(StatusOK).Build()),
			).Load()

			So(entries, ShouldHaveLength, 2)
			So(entries[0].Query, ShouldEqual, "list users")
			So(entries[1].Query, ShouldEqual, "list warehouses")
		})
	})
}

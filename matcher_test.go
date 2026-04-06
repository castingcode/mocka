package mocka

import (
	"log/slog"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// matchQuery receives an already-normalized query, matching how the HTTP
// handler calls it. Tests use normalizeQuery to mirror that pipeline so the
// assertions reflect real usage rather than internal representation details.

func TestMatchQuery_ExactMatch(t *testing.T) {

	logger := slog.Default()

	Convey("Given an exact-match entry is registered", t, func() {

		entries := []Entry{
			{
				MatchType:  MatchTypeExact,
				Query:      normalizeQuery("list warehouses where wh_id = 'MHE'"),
				StatusCode: StatusOK,
				ResultSet:  "<warehouses/>",
			},
		}

		Convey("When the query matches exactly", func() {
			r := matchQuery(normalizeQuery("list warehouses where wh_id = 'MHE'"), entries, logger)
			Convey("Then the registered response is returned", func() {
				So(r.StatusCode, ShouldEqual, StatusOK)
				So(r.ResultSet, ShouldEqual, "<warehouses/>")
			})
		})

		Convey("When the query differs from the registered entry", func() {
			r := matchQuery(normalizeQuery("list warehouses where wh_id = 'OTHER'"), entries, logger)
			Convey("Then command not found is returned", func() {
				So(r.StatusCode, ShouldEqual, StatusCommandNotFound)
			})
		})
	})
}

func TestMatchQuery_PublishData(t *testing.T) {

	logger := slog.Default()

	Convey("publish-data matching", t, func() {

		Convey("Given a contextual entry registered for a specific warehouse", func() {

			entries := []Entry{
				{
					MatchType:  MatchTypePublishData,
					Inner:      normalizeQuery("do thing"),
					Context:    map[string]string{"wh_id": "mhe"},
					StatusCode: StatusOK,
					ResultSet:  "<thing-mhe/>",
				},
			}

			Convey("When the publish-data query carries the matching context", func() {
				q := normalizeQuery("publish data where wh_id = 'MHE' | { do thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then the contextual response is returned", func() {
					So(r.StatusCode, ShouldEqual, StatusOK)
					So(r.ResultSet, ShouldEqual, "<thing-mhe/>")
				})
			})

			Convey("When the context value is provided with double quotes instead of single quotes", func() {
				q := normalizeQuery(`publish data where wh_id = "MHE" | { do thing }`)
				r := matchQuery(q, entries, logger)
				Convey("Then the response is still returned (quotes are interchangeable in local syntax)", func() {
					So(r.StatusCode, ShouldEqual, StatusOK)
					So(r.ResultSet, ShouldEqual, "<thing-mhe/>")
				})
			})

			Convey("When the publish-data query carries a different warehouse", func() {
				q := normalizeQuery("publish data where wh_id = 'OTHER' | { do thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then command not found is returned (no generic fallback registered)", func() {
					So(r.StatusCode, ShouldEqual, StatusCommandNotFound)
				})
			})

			Convey("When the inner command does not match the registered entry", func() {
				q := normalizeQuery("publish data where wh_id = 'MHE' | { do other thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then command not found is returned", func() {
					So(r.StatusCode, ShouldEqual, StatusCommandNotFound)
				})
			})
		})

		Convey("Given both a contextual entry and a generic fallback are registered", func() {

			entries := []Entry{
				{
					MatchType:  MatchTypePublishData,
					Inner:      normalizeQuery("do thing"),
					Context:    map[string]string{"wh_id": "mhe"},
					StatusCode: StatusOK,
					ResultSet:  "<thing-mhe/>",
				},
				{
					MatchType:  MatchTypePublishData,
					Inner:      normalizeQuery("do thing"),
					StatusCode: StatusOK,
					ResultSet:  "<thing-generic/>",
				},
			}

			Convey("When the context matches the registered entry", func() {
				q := normalizeQuery("publish data where wh_id = 'MHE' | { do thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then the contextual entry is preferred over the generic fallback", func() {
					So(r.ResultSet, ShouldEqual, "<thing-mhe/>")
				})
			})

			Convey("When the context does not match any contextual entry", func() {
				q := normalizeQuery("publish data where wh_id = 'OTHER' | { do thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then the generic fallback is used", func() {
					So(r.ResultSet, ShouldEqual, "<thing-generic/>")
				})
			})
		})

		Convey("Given an entry registered with multiple context keys", func() {

			entries := []Entry{
				{
					MatchType:  MatchTypePublishData,
					Inner:      normalizeQuery("do thing"),
					Context:    map[string]string{"wh_id": "mhe", "prt_client_id": "acme"},
					StatusCode: StatusOK,
					ResultSet:  "<thing-mhe-acme/>",
				},
			}

			Convey("When the query supplies both context keys in any order", func() {
				q := normalizeQuery("publish data where prt_client_id = 'ACME' and wh_id = 'MHE' | { do thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then the entry matches regardless of key order", func() {
					So(r.StatusCode, ShouldEqual, StatusOK)
					So(r.ResultSet, ShouldEqual, "<thing-mhe-acme/>")
				})
			})

			Convey("When the query supplies extra context keys beyond what the entry requires", func() {
				q := normalizeQuery("publish data where wh_id = 'MHE' and prt_client_id = 'ACME' and extra_key = 'X' | { do thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then the entry still matches (extra keys in query are ignored)", func() {
					So(r.StatusCode, ShouldEqual, StatusOK)
					So(r.ResultSet, ShouldEqual, "<thing-mhe-acme/>")
				})
			})

			Convey("When only one of the required context keys matches", func() {
				q := normalizeQuery("publish data where wh_id = 'MHE' and prt_client_id = 'WRONG' | { do thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then command not found is returned (partial context is not a match)", func() {
					So(r.StatusCode, ShouldEqual, StatusCommandNotFound)
				})
			})
		})

		Convey("Given a generic (no-context) entry only", func() {

			entries := []Entry{
				{
					MatchType:  MatchTypePublishData,
					Inner:      normalizeQuery("do thing"),
					StatusCode: StatusOK,
					ResultSet:  "<thing-generic/>",
				},
			}

			Convey("When any publish-data query arrives for that inner command", func() {
				q := normalizeQuery("publish data where wh_id = 'ANYTHING' | { do thing }")
				r := matchQuery(q, entries, logger)
				Convey("Then the generic entry matches regardless of context", func() {
					So(r.StatusCode, ShouldEqual, StatusOK)
					So(r.ResultSet, ShouldEqual, "<thing-generic/>")
				})
			})
		})
	})
}

func TestMatchQuery_PrefixMatch(t *testing.T) {

	logger := slog.Default()

	Convey("Given a prefix-match entry is registered", t, func() {

		entries := []Entry{
			{
				MatchType:  MatchTypePrefix,
				Prefix:     normalizeQuery("list warehouses"),
				StatusCode: StatusSrvNoDataFound,
				Message:    "No Data Found",
			},
		}

		Convey("When the query starts with the registered prefix", func() {
			r := matchQuery(normalizeQuery("list warehouses where wh_id = 'ABC'"), entries, logger)
			Convey("Then the prefix response is returned", func() {
				So(r.StatusCode, ShouldEqual, StatusSrvNoDataFound)
				So(r.Message, ShouldEqual, "No Data Found")
			})
		})

		Convey("When the query is exactly the prefix", func() {
			r := matchQuery(normalizeQuery("list warehouses"), entries, logger)
			Convey("Then the prefix response is still returned", func() {
				So(r.StatusCode, ShouldEqual, StatusSrvNoDataFound)
			})
		})

		Convey("When the query does not start with the prefix", func() {
			r := matchQuery(normalizeQuery("list users"), entries, logger)
			Convey("Then command not found is returned", func() {
				So(r.StatusCode, ShouldEqual, StatusCommandNotFound)
			})
		})
	})
}

func TestMatchQuery_MatchPriority(t *testing.T) {

	logger := slog.Default()

	Convey("match priority", t, func() {

		Convey("Exact match takes priority over prefix match for the same query text", func() {
			entries := []Entry{
				{MatchType: MatchTypePrefix, Prefix: normalizeQuery("list warehouses"), StatusCode: StatusSrvNoDataFound},
				{MatchType: MatchTypeExact, Query: normalizeQuery("list warehouses"), StatusCode: StatusOK, ResultSet: "<warehouses/>"},
			}
			r := matchQuery(normalizeQuery("list warehouses"), entries, logger)
			So(r.StatusCode, ShouldEqual, StatusOK)
			So(r.ResultSet, ShouldEqual, "<warehouses/>")
		})

		Convey("Publish-data match takes priority over prefix match", func() {
			entries := []Entry{
				{MatchType: MatchTypePrefix, Prefix: normalizeQuery("publish data"), StatusCode: StatusSrvNoDataFound},
				{
					MatchType:  MatchTypePublishData,
					Inner:      normalizeQuery("do thing"),
					StatusCode: StatusOK,
					ResultSet:  "<thing/>",
				},
			}
			q := normalizeQuery("publish data where wh_id = 'MHE' | { do thing }")
			r := matchQuery(q, entries, logger)
			So(r.StatusCode, ShouldEqual, StatusOK)
			So(r.ResultSet, ShouldEqual, "<thing/>")
		})
	})
}

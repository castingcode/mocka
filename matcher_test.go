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

		Convey("Given a less specific contextual entry registered before a more specific one", func() {

			entries := []Entry{
				{MatchType: MatchTypePublishData, Inner: "do thing", Context: map[string]string{"wh_id": "mhe"}, StatusCode: 1},
				{MatchType: MatchTypePublishData, Inner: "do thing", Context: map[string]string{"wh_id": "mhe", "client": "acme"}, StatusCode: 2},
				{MatchType: MatchTypePublishData, Inner: "do thing", Context: map[string]string{"client": "acme"}, StatusCode: 3},
			}

			Convey("When the query satisfies every entry", func() {
				q := normalizeQuery("publish data where client = 'ACME' and wh_id = 'MHE' | { do thing }")
				Convey("Then the entry with the most matching context keys wins", func() {
					So(matchQuery(q, entries, logger).StatusCode, ShouldEqual, 2)
				})
			})

			Convey("When the query satisfies two equally specific entries", func() {
				q := normalizeQuery("publish data where client = 'ACME' and wh_id = 'MHE' and x = '1' | { do thing }")
				one := []Entry{entries[2], entries[0]}
				Convey("Then the first registered wins", func() {
					So(matchQuery(q, one, logger).StatusCode, ShouldEqual, 3)
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

		Convey("When a longer prefix is registered after a shorter one", func() {
			entries := []Entry{
				{MatchType: MatchTypePrefix, Prefix: normalizeQuery("list"), StatusCode: 1},
				{MatchType: MatchTypePrefix, Prefix: normalizeQuery("list warehouses where"), StatusCode: 3},
				{MatchType: MatchTypePrefix, Prefix: normalizeQuery("list warehouses"), StatusCode: 2},
			}
			Convey("Then the longest matching prefix wins regardless of order", func() {
				So(matchQuery(normalizeQuery("list warehouses where wh_id = 'A'"), entries, logger).StatusCode, ShouldEqual, 3)
				So(matchQuery(normalizeQuery("list warehouses"), entries, logger).StatusCode, ShouldEqual, 2)
				So(matchQuery(normalizeQuery("list users"), entries, logger).StatusCode, ShouldEqual, 1)
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

func TestMatchQuery_CommandMatch(t *testing.T) {

	logger := slog.Default()
	status := func(query string, entries []Entry) int {
		return matchQuery(normalizeQuery(query), entries, logger).StatusCode
	}

	Convey("command matching", t, func() {

		entries := []Entry{
			{MatchType: MatchTypeCommand, Command: "list warehouses", Args: map[string]string{}, StatusCode: 1},
			{MatchType: MatchTypeCommand, Command: "list warehouses", Args: map[string]string{"wh_id": "mhe", "client": "acme"}, StatusCode: 3},
			{MatchType: MatchTypeCommand, Command: "list warehouses", Args: map[string]string{"wh_id": "mhe"}, StatusCode: 2},
		}

		Convey("Arguments match in any order, extra arguments are ignored, and the entry with the most matching args wins", func() {
			So(status("list warehouses where client = 'ACME' and x = 1 and wh_id = 'MHE'", entries), ShouldEqual, 3)
			So(status("list warehouses where wh_id = \"MHE\" and x = 1", entries), ShouldEqual, 2)
		})

		Convey("An entry without args is the fallback for other arguments and for no arguments", func() {
			So(status("list warehouses where wh_id = 'OTHER'", entries), ShouldEqual, 1)
			So(status("list warehouses", entries), ShouldEqual, 1)
		})

		Convey("The command name must match exactly, not as a prefix", func() {
			short := []Entry{{MatchType: MatchTypeCommand, Command: "list", Args: map[string]string{}, StatusCode: 1}}
			So(status("list warehouses", short), ShouldEqual, StatusCommandNotFound)
			So(status("list warehouse where wh_id = 'MHE'", entries), ShouldEqual, StatusCommandNotFound)
		})

		Convey("Quoted values keep ' and ', '=', and doubled quotes, and unquoted values match too", func() {
			quoted := []Entry{
				{MatchType: MatchTypeCommand, Command: "create note", Args: map[string]string{"txt": "salt and pepper = 1", "who": "o'brien", "qty": "5"}, StatusCode: 7},
			}
			So(status("create note where qty = 5 and txt = 'salt and pepper=1' and who = 'O''Brien'", quoted), ShouldEqual, 7)
			So(status("create note where qty = 5 and txt = 'salt' and who = 'O''Brien'", quoted), ShouldEqual, StatusCommandNotFound)
		})

		Convey("An arg registered with an empty value requires the argument to be present", func() {
			empty := []Entry{{MatchType: MatchTypeCommand, Command: "list x", Args: map[string]string{"a": ""}, StatusCode: 1}}
			So(status("list x where a = ''", empty), ShouldEqual, 1)
			So(status("list x where b = ''", empty), ShouldEqual, StatusCommandNotFound)
		})

		Convey("Pipelines, command blocks, and SQL are not command matched and fall through to prefix", func() {
			withPrefix := append([]Entry{{MatchType: MatchTypePrefix, Prefix: "list warehouses", StatusCode: 9}}, entries...)
			So(status("list warehouses where wh_id = 'MHE' | list orders", withPrefix), ShouldEqual, 9)
			So(status("list warehouses where wh_id = 'MHE'; list orders", withPrefix), ShouldEqual, 9)
			sql := []Entry{{MatchType: MatchTypeCommand, Command: "select", Args: map[string]string{}, StatusCode: 1}}
			So(status("[select * from wh_mst]", sql), ShouldEqual, StatusCommandNotFound)
		})

		Convey("Exact beats command, and command beats prefix", func() {
			mixed := []Entry{
				{MatchType: MatchTypePrefix, Prefix: "list warehouses", StatusCode: 9},
				{MatchType: MatchTypeCommand, Command: "list warehouses", Args: map[string]string{"wh_id": "mhe"}, StatusCode: 2},
				{MatchType: MatchTypeExact, Query: normalizeQuery("list warehouses where wh_id = 'MHE'"), StatusCode: 0},
			}
			So(status("list warehouses where wh_id = 'MHE'", mixed), ShouldEqual, 0)
			So(status("list warehouses where x = 1 and wh_id = 'MHE'", mixed), ShouldEqual, 2)
			So(status("list warehouses where wh_id = 'B'", mixed), ShouldEqual, 9)
		})
	})
}

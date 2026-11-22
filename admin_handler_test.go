package mocka

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/castingcode/mocaprotocol"
	. "github.com/smartystreets/goconvey/convey"
)

// newAdminTestServer returns a mux with the MOCA and admin routes, backed by a
// responses.yml with an exact entry and a prefix entry.
func newAdminTestServer(t *testing.T) (*http.ServeMux, *MocaRequestHandler) {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "wh.xml", `<moca-results><metadata/><data/></moca-results>`)
	writeTestFile(t, dir, "responses.yml", `
responses:
  - match: {type: exact, query: "list warehouses where wh_id = 'MHE'"}
    response: {status: 0, results: wh.xml}
  - match: {type: prefix, prefix: "list warehouses"}
    response: {status: 510, message: No Data Found}
`)
	lookup, err := NewResponseLookup(NewFileResponseLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewMocaRequestHandler(lookup)
	mux := http.NewServeMux()
	RegisterRoutes(mux, handler)
	RegisterAdminRoutes(mux, handler)
	return mux, handler
}

func postMatch(mux *http.ServeMux, contentType, body string) (*httptest.ResponseRecorder, matchResult) {
	req := httptest.NewRequest("POST", "/_mocka/match", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	var result matchResult
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return w, result
}

func TestAdmin_Match(t *testing.T) {

	Convey("Given a server with an exact and a prefix entry", t, func() {
		mux, handler := newAdminTestServer(t)

		Convey("When I match a query that differs only in case and whitespace from the exact entry", func() {
			w, result := postMatch(mux, "text/plain", "LIST warehouses\n  where wh_id = \"MHE\"")

			Convey("Then it reports the exact entry, where it is defined, and its response", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				So(result.NormalizedQuery, ShouldEqual, "list warehouses where wh_id = 'mhe'")
				So(result.Match, ShouldNotBeNil)
				So(result.Match.Type, ShouldEqual, "exact")
				So(result.Match.Source, ShouldEqual, "responses.yml entry 1")
				So(result.Response.Status, ShouldEqual, StatusOK)
				So(result.Response.Results, ShouldContainSubstring, "<moca-results>")
			})
		})

		Convey("When I match a query as JSON that only the prefix entry matches", func() {
			_, result := postMatch(mux, "application/json; charset=utf-8", `{"query": "list warehouses where wh_id = 'X'"}`)

			Convey("Then it reports the prefix entry", func() {
				So(result.Match.Type, ShouldEqual, "prefix")
				So(result.Match.Prefix, ShouldEqual, "list warehouses")
				So(result.Match.Source, ShouldEqual, "responses.yml entry 2")
				So(result.Response.Status, ShouldEqual, StatusSrvNoDataFound)
				So(result.Response.Message, ShouldEqual, "No Data Found")
			})
		})

		Convey("When I match a query that nothing matches", func() {
			w, result := postMatch(mux, "", "list users")

			Convey("Then match is null and the response is command not found", func() {
				So(w.Body.String(), ShouldContainSubstring, `"match":null`)
				So(result.Response.Status, ShouldEqual, StatusCommandNotFound)
			})
		})

		Convey("When I match a built-in command", func() {
			_, result := postMatch(mux, "", "publish data | { login user where usr_id = 'u' and usr_pswd = 'p' }")

			Convey("Then it reports the built-in command and no canned response", func() {
				So(result.Match.Type, ShouldEqual, "builtin")
				So(result.Match.Command, ShouldEqual, "login")
				So(result.Response, ShouldBeNil)
			})
		})

		Convey("When I send an empty query or malformed JSON", func() {
			empty, _ := postMatch(mux, "", "  \n")
			badJSON, _ := postMatch(mux, "application/json", `{"query":`)

			Convey("Then the request is rejected", func() {
				So(empty.Code, ShouldEqual, http.StatusBadRequest)
				So(badJSON.Code, ShouldEqual, http.StatusBadRequest)
			})
		})

		Convey("When I match queries", func() {
			postMatch(mux, "", "list users")

			Convey("Then they are not added to the request log", func() {
				So(handler.Requests(), ShouldBeEmpty)
			})
		})
	})
}

func TestAdmin_Requests(t *testing.T) {

	Convey("Given a server that has received several MOCA requests", t, func() {
		mux, handler := newAdminTestServer(t)

		send := func(query string, options ...TestRequestOption) mocaprotocol.MocaResponse {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, buildRequest(t, query, options...))
			var resp mocaprotocol.MocaResponse
			if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			return resp
		}
		login := send("login user where usr_id = 'u' and usr_pswd = 'p'")
		key := login.MocaResults.Data.Rows[0].Fields[4].Value
		send("list warehouses where wh_id = 'MHE'", WithSessionKey(key))
		send("list users", WithSessionKey(key))
		send("list users")

		list := func(path string) []RequestRecord {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			var body struct {
				Requests []RequestRecord `json:"requests"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			return body.Requests
		}

		Convey("When I list the requests", func() {
			records := list("/_mocka/requests")

			Convey("Then each request is logged in order with its status and what handled it", func() {
				So(records, ShouldHaveLength, 4)

				So(records[0].Match.Type, ShouldEqual, "builtin")
				So(records[0].Match.Command, ShouldEqual, "login")
				So(records[0].Status, ShouldEqual, StatusOK)

				So(records[1].Query, ShouldEqual, "list warehouses where wh_id = 'MHE'")
				So(records[1].Match.Source, ShouldEqual, "responses.yml entry 1")
				So(records[1].Unmatched, ShouldBeFalse)

				So(records[2].Status, ShouldEqual, StatusCommandNotFound)
				So(records[2].Match, ShouldBeNil)
				So(records[2].Unmatched, ShouldBeTrue)

				So(records[3].Status, ShouldEqual, StatusInvalidSessionKey)
				So(records[3].Unmatched, ShouldBeFalse)
				So(records[3].Time.IsZero(), ShouldBeFalse)
			})
		})

		Convey("When I list only unmatched requests", func() {
			records := list("/_mocka/requests?unmatched=true")

			Convey("Then only the query with no registered response is returned", func() {
				So(records, ShouldHaveLength, 1)
				So(records[0].NormalizedQuery, ShouldEqual, "list users")
				So(records[0].Status, ShouldEqual, StatusCommandNotFound)
			})
		})

		Convey("When I clear the requests", func() {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/_mocka/requests", nil))

			Convey("Then the log is empty over HTTP and from Go", func() {
				So(w.Code, ShouldEqual, http.StatusNoContent)
				So(list("/_mocka/requests"), ShouldBeEmpty)
				So(handler.Requests(), ShouldBeEmpty)
			})
		})
	})

	Convey("Given more requests than the log keeps", t, func() {
		var j requestJournal
		for i := range maxRequestRecords + 5 {
			j.add(RequestRecord{Status: i})
		}

		Convey("Then only the most recent are kept, oldest first", func() {
			records := j.list()
			So(records, ShouldHaveLength, maxRequestRecords)
			So(records[0].Status, ShouldEqual, 5)
			So(records[len(records)-1].Status, ShouldEqual, maxRequestRecords+4)
		})
	})
}

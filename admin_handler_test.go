package mocka

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/castingcode/mocaprotocol"
	. "github.com/smartystreets/goconvey/convey"
)

// newAdminTestServer returns a mux with the MOCA and admin routes, backed by a
// responses.yml with an exact entry and a prefix entry.
func newAdminTestServer(t *testing.T) (*http.ServeMux, *MocaRequestHandler, string) {
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
	return mux, handler, dir
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
		mux, handler, _ := newAdminTestServer(t)

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
		mux, handler, _ := newAdminTestServer(t)

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

// adminDo sends an admin request and returns the recorder.
func adminDo(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func listResponses(t *testing.T, mux *http.ServeMux) []responseView {
	t.Helper()
	var body struct {
		Responses []responseView `json:"responses"`
	}
	if err := json.Unmarshal(adminDo(mux, "GET", "/_mocka/responses", "").Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Responses
}

func TestAdmin_Responses(t *testing.T) {

	Convey("Given a server loaded from responses.yml", t, func() {
		mux, _, dir := newAdminTestServer(t)
		resolve := func(query string) matchResult {
			_, result := postMatch(mux, "", query)
			return result
		}

		Convey("When I list the responses", func() {
			views := listResponses(t, mux)

			Convey("Then each file entry is shown in match order with an id and its source", func() {
				So(views, ShouldHaveLength, 2)
				So(views[0].Match.Type, ShouldEqual, "exact")
				So(views[0].Match.Source, ShouldEqual, "responses.yml entry 1")
				So(views[0].Response.Results, ShouldContainSubstring, "<moca-results>")
				So(views[1].Match.Prefix, ShouldEqual, "list warehouses")
				So(views[0].Match.ID, ShouldBeGreaterThan, 0)
				So(views[1].Match.ID, ShouldNotEqual, views[0].Match.ID)
			})
		})

		Convey("When I register a new response", func() {
			w := adminDo(mux, "POST", "/_mocka/responses", `{
				"match": {"type": "exact", "query": "LIST  Users"},
				"response": {"status": 0, "results": "<moca-results><metadata/><data/></moca-results>"}
			}`)
			var view responseView
			_ = json.Unmarshal(w.Body.Bytes(), &view)

			Convey("Then it is created, normalized, and queries resolve to it", func() {
				So(w.Code, ShouldEqual, http.StatusCreated)
				So(view.Match.Query, ShouldEqual, "list users")
				So(view.Match.Source, ShouldEqual, "admin API")
				So(listResponses(t, mux), ShouldHaveLength, 3)
				result := resolve("list users")
				So(result.Match.ID, ShouldEqual, view.Match.ID)
				So(result.Response.Status, ShouldEqual, StatusOK)
			})

			Convey("And I delete it by id", func() {
				del := adminDo(mux, "DELETE", fmt.Sprintf("/_mocka/responses/%d", view.Match.ID), "")

				Convey("Then it is gone and the query no longer matches", func() {
					So(del.Code, ShouldEqual, http.StatusNoContent)
					So(listResponses(t, mux), ShouldHaveLength, 2)
					So(resolve("list users").Match, ShouldBeNil)
				})

				Convey("Then deleting it again is not found", func() {
					again := adminDo(mux, "DELETE", fmt.Sprintf("/_mocka/responses/%d", view.Match.ID), "")
					So(again.Code, ShouldEqual, http.StatusNotFound)
				})
			})
		})

		Convey("When I register a response that matches the same query as a file entry", func() {
			before := listResponses(t, mux)
			w := adminDo(mux, "POST", "/_mocka/responses", `{
				"match": {"type": "exact", "query": "list warehouses where wh_id = \"mhe\""},
				"response": {"status": 511, "message": "Database Error"}
			}`)
			after := listResponses(t, mux)

			Convey("Then it replaces the file entry in place with a new id", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				So(after, ShouldHaveLength, 2)
				So(after[0].Match.Source, ShouldEqual, "admin API")
				So(after[0].Match.ID, ShouldNotEqual, before[0].Match.ID)
				So(after[1].Match.ID, ShouldEqual, before[1].Match.ID)
				result := resolve("list warehouses where wh_id = 'MHE'")
				So(result.Response.Status, ShouldEqual, StatusDBError)
				So(result.Response.Message, ShouldEqual, "Database Error")
			})
		})

		Convey("When I register an invalid response", func() {
			cases := []struct{ name, body, wantErr string }{
				{"an unknown match type", `{"match": {"type": "exat", "query": "a"}}`, `unknown match type "exat"`},
				{"a misspelled field", `{"match": {"type": "exact", "querry": "a"}}`, `unknown field "querry"`},
				{"a query_file", `{"match": {"type": "exact", "query_file": "q.txt"}}`, "query_file is not supported"},
				{"results that are not XML", `{"match": {"type": "exact", "query": "a"}, "response": {"results": "<moca-results>"}}`, "invalid response.results XML"},
			}
			for _, c := range cases {
				w := adminDo(mux, "POST", "/_mocka/responses", c.body)
				Convey("Then "+c.name+" is rejected without changing the responses", func() {
					So(w.Code, ShouldEqual, http.StatusBadRequest)
					So(w.Body.String(), ShouldContainSubstring, c.wantErr)
					So(listResponses(t, mux), ShouldHaveLength, 2)
				})
			}
		})

		Convey("When I delete with an id that is not a number", func() {
			w := adminDo(mux, "DELETE", "/_mocka/responses/abc", "")
			So(w.Code, ShouldEqual, http.StatusBadRequest)
		})

		Convey("When I add a runtime response, change responses.yml, and reload", func() {
			adminDo(mux, "POST", "/_mocka/responses", `{"match": {"type": "exact", "query": "list users"}}`)
			oldIDs := map[int]bool{}
			for _, v := range listResponses(t, mux) {
				oldIDs[v.Match.ID] = true
			}
			writeTestFile(t, dir, "responses.yml", `
responses:
  - match: {type: prefix, prefix: "list"}
    response: {status: 510}
`)
			w := adminDo(mux, "POST", "/_mocka/reload", "")
			views := listResponses(t, mux)

			Convey("Then only the entries now in the file remain, with ids not used before", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Body.String(), ShouldContainSubstring, `"count":1`)
				So(views, ShouldHaveLength, 1)
				So(views[0].Match.Prefix, ShouldEqual, "list")
				So(oldIDs[views[0].Match.ID], ShouldBeFalse)
				So(resolve("list users").Match.Type, ShouldEqual, "prefix")
			})
		})

		Convey("When responses.yml becomes invalid and I reload", func() {
			before := listResponses(t, mux)
			writeTestFile(t, dir, "responses.yml", "responses:\n  - match: {type: exat}\n")
			w := adminDo(mux, "POST", "/_mocka/reload", "")

			Convey("Then the error is returned and the current responses are kept", func() {
				So(w.Code, ShouldEqual, http.StatusInternalServerError)
				So(w.Body.String(), ShouldContainSubstring, `unknown match type "exat"`)
				So(listResponses(t, mux), ShouldResemble, before)
			})
		})
	})

	Convey("Given a server backed by an in-memory loader", t, func() {
		loader := NewInMemoryResponseLoader(
			WithExactMatch("a", NewResponse(1).Build()),
			WithExactMatch("b", NewResponse(2).Build()),
		)
		lookup, err := NewResponseLookup(loader)
		if err != nil {
			t.Fatal(err)
		}
		handler := NewMocaRequestHandler(lookup)
		mux := http.NewServeMux()
		RegisterAdminRoutes(mux, handler)

		Convey("When I replace and delete entries and then reload", func() {
			views := listResponses(t, mux)
			adminDo(mux, "DELETE", fmt.Sprintf("/_mocka/responses/%d", views[0].Match.ID), "")
			adminDo(mux, "POST", "/_mocka/responses", `{"match": {"type": "exact", "query": "b"}, "response": {"status": 99}}`)
			adminDo(mux, "POST", "/_mocka/reload", "")
			after := listResponses(t, mux)

			Convey("Then the loader's original entries come back unchanged", func() {
				So(after, ShouldHaveLength, 2)
				So(after[0].Match.Query, ShouldEqual, "a")
				So(after[0].Response.Status, ShouldEqual, 1)
				So(after[1].Match.Query, ShouldEqual, "b")
				So(after[1].Response.Status, ShouldEqual, 2)
			})
		})
	})
}

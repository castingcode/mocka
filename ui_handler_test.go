package mocka

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

const uiTestResults = `<moca-results><metadata><column name="wh_id" type="S" length="10" nullable="true"/><column name="descr" type="S" length="40" nullable="true"/></metadata><data><row><field>MHE</field><field null="true"></field></row></data></moca-results>`

// newUITestServer serves the MOCA, admin, and UI routes for a responses
// folder holding one exact entry. With saved false, the UI does not save.
func newUITestServer(t *testing.T, saved bool) (*http.ServeMux, *MocaRequestHandler, string) {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "wh.xml", uiTestResults)
	writeTestFile(t, dir, "responses.yml", `
responses:
  - match: {type: exact, query: "list warehouses where wh_id = 'MHE'"}
    response: {status: 0, results: wh.xml}
`)
	lookup, err := NewResponseLookup(NewFileResponseLoader(dir))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewMocaRequestHandler(lookup)
	folder := ""
	if saved {
		folder = dir
	}
	ui, err := NewUIHandler(handler, folder)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, handler)
	RegisterAdminRoutes(mux, handler)
	RegisterUIRoutes(mux, ui)
	return mux, handler, dir
}

func uiGet(mux http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

func uiPost(mux http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

// redirectMessage returns the notice or error a UI redirect carries.
func redirectMessage(w *httptest.ResponseRecorder) (notice, errMsg string) {
	u, _ := url.Parse(w.Header().Get("Location"))
	return u.Query().Get("notice"), u.Query().Get("error")
}

func entryBySource(h *MocaRequestHandler, prefix string) []Entry {
	var found []Entry
	for _, e := range h.lookup.list() {
		if strings.HasPrefix(e.Source, prefix) {
			found = append(found, e)
		}
	}
	return found
}

func TestUI_Pages(t *testing.T) {

	Convey("Given the UI", t, func() {
		mux, _, _ := newUITestServer(t, true)

		Convey("The root redirects to the responses page", func() {
			w := uiGet(mux, "/_mocka/ui/")
			So(w.Code, ShouldEqual, http.StatusSeeOther)
			So(w.Header().Get("Location"), ShouldEqual, "/_mocka/ui/responses")
		})

		Convey("The responses page lists the entries and the add form, and says where edits are saved", func() {
			w := uiGet(mux, "/_mocka/ui/responses")
			So(w.Code, ShouldEqual, http.StatusOK)
			So(w.Header().Get("Content-Type"), ShouldEqual, "text/html; charset=utf-8")
			body := w.Body.String()
			So(body, ShouldContainSubstring, "list warehouses where wh_id = &#39;mhe&#39;")
			So(body, ShouldContainSubstring, "responses.yml entry 1")
			So(body, ShouldContainSubstring, "1 row")
			So(body, ShouldContainSubstring, `Saved to <code>responses.d/ui.yml</code>`)
			So(body, ShouldContainSubstring, `<script src="/_mocka/ui/static/htmx.min.js" defer></script>`)
		})

		Convey("Query parameters fill in the add form, as the Create response links do", func() {
			body := uiGet(mux, "/_mocka/ui/responses?type=exact&query="+url.QueryEscape("list x where a = '<b>'")).Body.String()
			So(body, ShouldContainSubstring, `<option value="exact" selected>`)
			So(body, ShouldContainSubstring, "list x where a = &#39;&lt;b&gt;&#39;</textarea>")
		})

		Convey("The embedded static files are served", func() {
			js := uiGet(mux, "/_mocka/ui/static/htmx.min.js")
			So(js.Code, ShouldEqual, http.StatusOK)
			So(js.Body.String(), ShouldContainSubstring, `version:"2.0.11"`)
			So(uiGet(mux, "/_mocka/ui/static/style.css").Code, ShouldEqual, http.StatusOK)
		})
	})
}

func TestUI_Test(t *testing.T) {

	Convey("Given the test page", t, func() {
		mux, _, _ := newUITestServer(t, true)
		test := func(query string) string {
			return uiGet(mux, "/_mocka/ui/test?query="+url.QueryEscape(query)).Body.String()
		}

		Convey("A matching query shows the entry, its source, and the result set as a table with nulls", func() {
			body := test(`LIST warehouses where wh_id = "MHE"`)
			So(body, ShouldContainSubstring, "exact: list warehouses where wh_id = &#39;mhe&#39;")
			So(body, ShouldContainSubstring, "responses.yml entry 1")
			So(body, ShouldContainSubstring, "<th>wh_id (S)</th>")
			So(body, ShouldContainSubstring, "<td>MHE</td>")
			So(body, ShouldContainSubstring, `<span class="null">null</span>`)
		})

		Convey("An unmatched query shows the 501 and a link to create a response for it", func() {
			body := test("list carriers")
			So(body, ShouldContainSubstring, "No response matches.")
			So(body, ShouldContainSubstring, "501")
			So(body, ShouldContainSubstring, `href="/_mocka/ui/responses?type=exact&amp;query=list&#43;carriers#add"`)
		})

		Convey("A built-in command is reported as built in", func() {
			So(test("ping"), ShouldContainSubstring, "built-in ping command")
		})
	})
}

func TestUI_Requests(t *testing.T) {

	Convey("Given requests including an unmatched one with markup in it", t, func() {
		mux, _, _ := newUITestServer(t, true)
		login := parseMoca(t, sendMoca(t, mux, "login user where usr_id = 'u' and usr_pswd = 'p'").Body.Bytes())
		key := login.MocaResults.Data.Rows[0].Fields[4].Value
		sendMoca(t, mux, "list warehouses where wh_id = 'MHE'", WithSessionKey(key))
		sendMoca(t, mux, "list x where a = '<script>alert(1)</script>'", WithSessionKey(key))

		Convey("The page lists them newest first, escaped, with how each was handled", func() {
			body := uiGet(mux, "/_mocka/ui/requests").Body.String()
			So(body, ShouldNotContainSubstring, "<script>alert(1)</script>")
			So(body, ShouldContainSubstring, "&lt;script&gt;alert(1)&lt;/script&gt;")
			So(body, ShouldContainSubstring, "built-in login")
			So(body, ShouldContainSubstring, "exact · responses.yml entry 1")
			So(strings.Index(body, "list x where"), ShouldBeLessThan, strings.Index(body, "list warehouses where"))
		})

		Convey("The rows fragment the page polls holds only the table body, filtered to unmatched requests", func() {
			body := uiGet(mux, "/_mocka/ui/requests/rows?unmatched=1").Body.String()
			So(strings.TrimSpace(body), ShouldStartWith, `<tbody id="request-rows" hx-get="/_mocka/ui/requests/rows?unmatched=1" hx-trigger="every 2s"`)
			So(body, ShouldNotContainSubstring, "<html")
			So(body, ShouldContainSubstring, "Create response")
			So(body, ShouldNotContainSubstring, "list warehouses")
		})

		Convey("Clear empties the log", func() {
			w := uiPost(mux, "/_mocka/ui/requests/clear", nil)
			So(w.Code, ShouldEqual, http.StatusSeeOther)
			So(uiGet(mux, "/_mocka/ui/requests").Body.String(), ShouldContainSubstring, "No requests yet.")
		})
	})
}

func TestUI_AddAndDelete(t *testing.T) {

	Convey("Given the UI with a responses folder", t, func() {
		mux, handler, dir := newUITestServer(t, true)
		uiYML := func() string {
			b, _ := os.ReadFile(filepath.Join(dir, "responses.d", "ui.yml"))
			return string(b)
		}
		status := func(query string) int {
			resp, _ := handler.lookup.resolve(normalizeQuery(query))
			return resp.StatusCode
		}

		Convey("When I add an exact response with results", func() {
			w := uiPost(mux, "/_mocka/ui/responses", url.Values{
				"type": {"exact"}, "query": {"list carriers where carcod = 'UPS'"}, "status": {"0"}, "results": {uiTestResults},
				"prefix": {"ignored, since the type is exact"},
			})
			notice, _ := redirectMessage(w)

			Convey("Then it is saved to ui.yml with its results in ui/, and queries match it", func() {
				So(w.Code, ShouldEqual, http.StatusSeeOther)
				So(notice, ShouldEqual, "Saved to responses.d/ui.yml.")
				So(uiYML(), ShouldContainSubstring, "query: list carriers where carcod = 'UPS'")
				So(uiYML(), ShouldNotContainSubstring, "prefix")
				matches, _ := filepath.Glob(filepath.Join(dir, "ui", "list-carriers-where-carcod-ups-*.xml"))
				So(matches, ShouldHaveLength, 1)
				saved := entryBySource(handler, "responses.d/ui.yml")
				So(saved, ShouldHaveLength, 1)
				So(saved[0].Source, ShouldEqual, "responses.d/ui.yml entry 1")
				So(status("LIST CARRIERS where carcod = 'ups'"), ShouldEqual, StatusOK)
			})

			Convey("Then a fresh server on the same folder serves it", func() {
				lookup, err := NewResponseLookup(NewFileResponseLoader(dir))
				So(err, ShouldBeNil)
				So(lookup.GetResponse(normalizeQuery("list carriers where carcod = 'UPS'")).ResultSet, ShouldContainSubstring, "<field>MHE</field>")
			})

			Convey("And I add it again with another status", func() {
				uiPost(mux, "/_mocka/ui/responses", url.Values{"type": {"exact"}, "query": {"list carriers where carcod = 'ups'"}, "status": {"510"}})

				Convey("Then the saved entry is replaced and its old result file removed", func() {
					So(strings.Count(uiYML(), "match:"), ShouldEqual, 1)
					So(status("list carriers where carcod = 'UPS'"), ShouldEqual, StatusSrvNoDataFound)
					matches, _ := filepath.Glob(filepath.Join(dir, "ui", "*.xml"))
					So(matches, ShouldBeEmpty)
				})
			})

			Convey("And I delete it", func() {
				saved := entryBySource(handler, "responses.d/ui.yml")
				w := uiPost(mux, "/_mocka/ui/responses/"+strconv.Itoa(saved[0].id)+"/delete", nil)
				notice, _ := redirectMessage(w)

				Convey("Then it is removed from ui.yml and its files, and no longer matches", func() {
					So(notice, ShouldContainSubstring, "from responses.d/ui.yml")
					So(uiYML(), ShouldNotContainSubstring, "list carriers")
					matches, _ := filepath.Glob(filepath.Join(dir, "ui", "*.xml"))
					So(matches, ShouldBeEmpty)
					So(status("list carriers where carcod = 'UPS'"), ShouldEqual, StatusCommandNotFound)
				})
			})
		})

		Convey("When I add command and publish_data responses from key = value lines", func() {
			uiPost(mux, "/_mocka/ui/responses", url.Values{"type": {"command"}, "command": {"list orders"}, "args": {"ordnum = ORD*\n\n wh_id = MHE "}, "status": {"1"}})
			uiPost(mux, "/_mocka/ui/responses", url.Values{"type": {"publish_data"}, "inner": {"do thing"}, "context": {"wh_id = MHE"}, "status": {"2"}})

			Convey("Then both are saved and match", func() {
				So(strings.Count(uiYML(), "match:"), ShouldEqual, 2)
				So(status("list orders where wh_id = 'MHE' and ordnum = 'ORD7'"), ShouldEqual, 1)
				So(status("publish data where wh_id = 'MHE' | { do thing }"), ShouldEqual, 2)
			})
		})

		Convey("When I submit invalid input", func() {
			cases := []struct {
				name    string
				form    url.Values
				wantErr string
			}{
				{"results that are not MOCA XML", url.Values{"type": {"exact"}, "query": {"q <1>"}, "results": {"<warehouses/>"}}, "invalid results XML"},
				{"a status that is not a number", url.Values{"type": {"exact"}, "query": {"q <1>"}, "status": {"ok"}}, `status &#34;ok&#34; is not a number`},
				{"an args line without =", url.Values{"type": {"command"}, "command": {"list x"}, "args": {"ordnum"}}, `args: line 1 &#34;ordnum&#34; is not key = value`},
				{"a command with a where clause", url.Values{"type": {"command"}, "command": {"list x where a = 1"}}, "must be only a command name"},
			}
			for _, c := range cases {
				w := uiPost(mux, "/_mocka/ui/responses", c.form)
				Convey("Then "+c.name+" is shown as an error with the form kept, and nothing is saved", func() {
					So(w.Code, ShouldEqual, http.StatusUnprocessableEntity)
					So(w.Body.String(), ShouldContainSubstring, `<p class="error" role="alert">`)
					So(w.Body.String(), ShouldContainSubstring, c.wantErr)
					if q := c.form.Get("query"); q != "" {
						So(w.Body.String(), ShouldContainSubstring, "q &lt;1&gt;</textarea>")
					}
					So(uiYML(), ShouldBeEmpty)
				})
			}
		})

		Convey("When I add a response whose match is already defined in responses.yml", func() {
			w := uiPost(mux, "/_mocka/ui/responses", url.Values{"type": {"exact"}, "query": {"LIST warehouses where wh_id = 'mhe'"}, "status": {"510"}})

			Convey("Then it is refused, so ui.yml never duplicates another file", func() {
				So(w.Code, ShouldEqual, http.StatusUnprocessableEntity)
				So(w.Body.String(), ShouldContainSubstring, "already defined in responses.yml entry 1; change it there")
				So(uiYML(), ShouldBeEmpty)
			})
		})

		Convey("When a response was added through the admin API", func() {
			runtime, _ := handler.lookup.put(Entry{MatchType: MatchTypePrefix, Prefix: "list stuff", StatusCode: 7, Source: "admin API"})

			Convey("And I add a different response in the UI", func() {
				uiPost(mux, "/_mocka/ui/responses", url.Values{"type": {"exact"}, "query": {"list carriers"}})

				Convey("Then the admin API response is kept, with its id", func() {
					kept := entryBySource(handler, "admin API")
					So(kept, ShouldHaveLength, 1)
					So(kept[0].id, ShouldEqual, runtime.id)
					So(status("list stuff here"), ShouldEqual, 7)
				})
			})

			Convey("And I add a response with the same match in the UI", func() {
				uiPost(mux, "/_mocka/ui/responses", url.Values{"type": {"prefix"}, "prefix": {"LIST STUFF"}, "status": {"8"}})

				Convey("Then the saved response replaces it", func() {
					So(entryBySource(handler, "admin API"), ShouldBeEmpty)
					So(status("list stuff here"), ShouldEqual, 8)
				})
			})
		})

		Convey("When I delete a response defined in responses.yml", func() {
			w := uiPost(mux, "/_mocka/ui/responses/"+strconv.Itoa(handler.lookup.list()[0].id)+"/delete", nil)
			notice, _ := redirectMessage(w)

			Convey("Then it is removed only until the next reload, and the file is untouched", func() {
				So(notice, ShouldContainSubstring, "until the next reload. It is defined in responses.yml entry 1.")
				So(status("list warehouses where wh_id = 'MHE'"), ShouldEqual, StatusCommandNotFound)
				uiPost(mux, "/_mocka/ui/reload", nil)
				So(status("list warehouses where wh_id = 'MHE'"), ShouldEqual, StatusOK)
			})
		})

		Convey("When I delete a response that no longer exists", func() {
			_, errMsg := redirectMessage(uiPost(mux, "/_mocka/ui/responses/9999/delete", nil))
			So(errMsg, ShouldContainSubstring, "there is no response 9999")
		})
	})

	Convey("Given the UI without a responses folder", t, func() {
		mux, handler, dir := newUITestServer(t, false)

		Convey("When I add a response", func() {
			w := uiPost(mux, "/_mocka/ui/responses", url.Values{"type": {"exact"}, "query": {"list carriers"}, "status": {"0"}})
			notice, _ := redirectMessage(w)

			Convey("Then it is used until the next reload but not written anywhere", func() {
				So(notice, ShouldContainSubstring, "It lasts until the next reload.")
				So(entryBySource(handler, "UI (not saved)"), ShouldHaveLength, 1)
				_, err := os.Stat(filepath.Join(dir, "responses.d"))
				So(os.IsNotExist(err), ShouldBeTrue)
				uiPost(mux, "/_mocka/ui/reload", nil)
				So(entryBySource(handler, "UI (not saved)"), ShouldBeEmpty)
			})
		})

		Convey("Then the page says edits are not saved", func() {
			So(uiGet(mux, "/_mocka/ui/responses").Body.String(), ShouldContainSubstring, "Not saved to a file")
		})
	})
}

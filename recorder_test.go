package mocka

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/castingcode/mocaprotocol"
	. "github.com/smartystreets/goconvey/convey"
)

// fakeUpstream is a MOCA server that answers each normalized query with a
// canned response body and counts the requests it receives.
type fakeUpstream struct {
	mu        sync.Mutex
	responses map[string]string
	requests  int
}

func (u *fakeUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req mocaprotocol.MocaRequest
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.requests++
	body, ok := u.responses[normalizeQuery(req.Query.Text)]
	if !ok {
		body = `<?xml version="1.0" encoding="UTF-8"?><moca-response><status>501</status><message>not found upstream</message></moca-response>`
	}
	w.Header().Set("Content-Type", "application/moca-xml")
	w.Write([]byte(body))
}

func (u *fakeUpstream) set(query, body string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.responses[normalizeQuery(query)] = body
}

func readSample(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "recorder", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mocaResponse(status int, message string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><moca-response><status>%d</status><message>%s</message></moca-response>`, status, message)
}

// sendMoca posts query to mux at /service and returns the raw recorder.
func sendMoca(t *testing.T, mux http.Handler, query string, options ...TestRequestOption) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, buildRequest(t, query, options...))
	return w
}

func parseMoca(t *testing.T, body []byte) mocaprotocol.MocaResponse {
	t.Helper()
	var resp mocaprotocol.MocaResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		t.Fatalf("parsing MOCA response: %v\n%s", err, body)
	}
	return resp
}

// serveFolder returns a mux serving the responses folder the normal way, and
// a valid session key for it.
func serveFolder(t *testing.T, folder string) (*http.ServeMux, string) {
	t.Helper()
	lookup, err := NewResponseLookup(NewFileResponseLoader(folder))
	if err != nil {
		t.Fatalf("recorded folder does not load: %v", err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewMocaRequestHandler(lookup))
	login := parseMoca(t, sendMoca(t, mux, "login user where usr_id = 'u' and usr_pswd = 'p'").Body.Bytes())
	return mux, login.MocaResults.Data.Rows[0].Fields[4].Value
}

func TestRecordingHandler(t *testing.T) {

	Convey("Given a recording handler in front of a MOCA server", t, func() {
		upstream := &fakeUpstream{responses: map[string]string{}}
		warehouses := readSample(t, "list_warehouses_response.xml")
		upstream.set("list warehouses", warehouses)
		upstream.set("login user where usr_id = 'SUPER' and usr_pswd = 'pw'", readSample(t, "login_success_response.xml"))
		upstream.set("list orders where ordnum = 'X'", mocaResponse(510, "No Data Found"))
		upstream.set("list stuff", mocaResponse(523, "Invalid session key"))
		server := httptest.NewServer(upstream)
		defer server.Close()

		folder := t.TempDir()
		newRecorder := func() *http.ServeMux {
			recorder, err := NewRecordingHandler(server.URL+"/service", folder)
			So(err, ShouldBeNil)
			mux := http.NewServeMux()
			RegisterRoutes(mux, recorder)
			return mux
		}
		mux := newRecorder()
		recorded := func() string {
			b, _ := os.ReadFile(filepath.Join(folder, "responses.d", "recorded.yml"))
			return string(b)
		}

		Convey("When the client logs in", func() {
			w := sendMoca(t, mux, "login user where usr_id = 'SUPER' and usr_pswd = 'pw'")

			Convey("Then the real server's login response is returned and nothing is recorded", func() {
				So(w.Body.String(), ShouldEqual, readSample(t, "login_success_response.xml"))
				So(recorded(), ShouldBeEmpty)
			})
		})

		Convey("When the client runs a command that returns results", func() {
			w := sendMoca(t, mux, "list warehouses", WithSessionKey("real-session"))

			Convey("Then the client gets the real response unchanged", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Header().Get("Content-Type"), ShouldEqual, "application/moca-xml")
				So(w.Body.String(), ShouldEqual, warehouses)
			})

			Convey("Then serving the folder returns the same status and results as the real server", func() {
				served, key := serveFolder(t, folder)
				got := parseMoca(t, sendMoca(t, served, "LIST  warehouses", WithSessionKey(key)).Body.Bytes())
				want := parseMoca(t, []byte(warehouses))
				So(got.Status, ShouldEqual, want.Status)
				So(got.MocaResults, ShouldResemble, want.MocaResults)
				nulls := 0
				for _, row := range got.MocaResults.Data.Rows {
					for _, f := range row.Fields {
						if f.Null == "true" {
							nulls++
						}
					}
				}
				So(nulls, ShouldBeGreaterThan, 0) // the sample has null fields; they must survive the round trip
			})

			Convey("Then the result XML is saved next to the entry under recorded/", func() {
				So(recorded(), ShouldContainSubstring, "query: list warehouses")
				So(recorded(), ShouldContainSubstring, "results: recorded/list-warehouses-")
				matches, _ := filepath.Glob(filepath.Join(folder, "recorded", "list-warehouses-*.xml"))
				So(matches, ShouldHaveLength, 1)
			})

			Convey("And the real server later returns no data for the same query, sent differently", func() {
				upstream.set("list warehouses", mocaResponse(510, "No Data Found"))
				sendMoca(t, mux, "  LIST WAREHOUSES ", WithSessionKey("real-session"))

				Convey("Then the one entry is replaced and its old result file is removed", func() {
					So(strings.Count(recorded(), "match:"), ShouldEqual, 1)
					So(recorded(), ShouldContainSubstring, "status: 510")
					matches, _ := filepath.Glob(filepath.Join(folder, "recorded", "*.xml"))
					So(matches, ShouldBeEmpty)
					served, key := serveFolder(t, folder)
					got := parseMoca(t, sendMoca(t, served, "list warehouses", WithSessionKey(key)).Body.Bytes())
					So(got.Status, ShouldEqual, StatusSrvNoDataFound)
					So(got.Message, ShouldEqual, "No Data Found")
				})
			})

			Convey("And a new recording handler records another command into the same folder", func() {
				mux2 := newRecorder()
				sendMoca(t, mux2, "list orders where ordnum = 'X'", WithSessionKey("real-session"))

				Convey("Then the earlier recording is kept", func() {
					So(strings.Count(recorded(), "match:"), ShouldEqual, 2)
					served, key := serveFolder(t, folder)
					So(parseMoca(t, sendMoca(t, served, "list warehouses", WithSessionKey(key)).Body.Bytes()).Status, ShouldEqual, StatusOK)
					So(parseMoca(t, sendMoca(t, served, "list orders where ordnum = 'X'", WithSessionKey(key)).Body.Bytes()).Status, ShouldEqual, StatusSrvNoDataFound)
				})
			})
		})

		Convey("When the client runs a multi-line command", func() {
			query := "[select wh_id\n   from wh_mst]"
			upstream.set(query, warehouses)
			sendMoca(t, mux, query, WithSessionKey("real-session"))

			Convey("Then the query is saved to a query file and still matches when served", func() {
				So(recorded(), ShouldContainSubstring, "query_file: recorded/queries/select-wh-id-from-wh-mst-")
				served, key := serveFolder(t, folder)
				got := parseMoca(t, sendMoca(t, served, "[select wh_id from wh_mst]", WithSessionKey(key)).Body.Bytes())
				So(got.Status, ShouldEqual, StatusOK)
				So(got.MocaResults.Data.Rows, ShouldHaveLength, 2)
			})
		})

		Convey("When the real server returns an error without results", func() {
			sendMoca(t, mux, "list orders where ordnum = 'X'", WithSessionKey("real-session"))

			Convey("Then the status and message are recorded without a result file", func() {
				So(recorded(), ShouldContainSubstring, "status: 510")
				So(recorded(), ShouldContainSubstring, "message: No Data Found")
				So(recorded(), ShouldNotContainSubstring, "results:")
			})
		})

		Convey("When the real server rejects the session or does not know the command", func() {
			rejected := sendMoca(t, mux, "list stuff", WithSessionKey("expired"))
			unknown := sendMoca(t, mux, "list nothing", WithSessionKey("real-session"))

			Convey("Then the responses are passed through but not recorded", func() {
				So(parseMoca(t, rejected.Body.Bytes()).Status, ShouldEqual, StatusInvalidSessionKey)
				So(parseMoca(t, unknown.Body.Bytes()).Status, ShouldEqual, StatusCommandNotFound)
				So(recorded(), ShouldBeEmpty)
			})
		})

		Convey("When the folder's own responses.yml already has an exact entry for the query", func() {
			writeTestFile(t, folder, "responses.yml", "responses:\n  - match: {type: exact, query: LIST WAREHOUSES}\n    response: {status: 0}\n")
			mux := newRecorder()
			w := sendMoca(t, mux, "list warehouses", WithSessionKey("real-session"))

			Convey("Then the request is still forwarded but not recorded, so the folder keeps loading", func() {
				So(w.Body.String(), ShouldEqual, warehouses)
				So(recorded(), ShouldBeEmpty)
				serveFolder(t, folder)
			})
		})
	})

	Convey("Given a recording handler whose MOCA server is down", t, func() {
		server := httptest.NewServer(http.NotFoundHandler())
		url := server.URL + "/service"
		server.Close()
		recorder, err := NewRecordingHandler(url, t.TempDir())
		So(err, ShouldBeNil)
		mux := http.NewServeMux()
		RegisterRoutes(mux, recorder)

		Convey("Then requests fail with 502 Bad Gateway", func() {
			w := sendMoca(t, mux, "list warehouses")
			So(w.Code, ShouldEqual, http.StatusBadGateway)
			So(w.Body.String(), ShouldContainSubstring, url)
		})
	})

	Convey("Given a responses folder with invalid response files", t, func() {
		folder := t.TempDir()
		writeTestFile(t, folder, "responses.yml", "responses:\n  - match: {type: exat}\n")
		_, err := NewRecordingHandler("http://localhost/service", folder)

		Convey("Then creating the recording handler fails", func() {
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, `unknown match type "exat"`)
		})
	})
}

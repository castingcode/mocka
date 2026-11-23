package mocka

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
)

//go:embed ui
var uiFS embed.FS

const (
	// uiFile is where responses created in the UI are saved, relative to the
	// responses folder.
	uiFile = "responses.d/ui.yml"
	// uiDir holds the result and query files of responses created in the UI.
	uiDir = "ui"
	// uiBase is the path the UI is served under.
	uiBase = "/_mocka/ui/"
)

// UIHandler serves a web UI under /_mocka/ui/ to test queries, watch the
// requests mocka receives, and add, delete, and reload responses.
//
// With a responses folder, responses added in the UI are saved to
// responses.d/ui.yml in it, with result XML in its ui/ folder, so they are
// kept across reloads and restarts. Deleting such a response removes it from
// that file. Responses defined in other files can be deleted only until the
// next reload; the UI never edits those files. Without a responses folder,
// responses added in the UI last until the next reload, like those added
// through the admin API.
type UIHandler struct {
	moca   *MocaRequestHandler
	file   *managedFile // nil when edits are not saved
	pages  map[string]*template.Template
	mux    *http.ServeMux
	logger *slog.Logger

	mu sync.Mutex // serializes edits, which write the file and then reload
}

var _ http.Handler = (*UIHandler)(nil)

// NewUIHandler creates the UI for handler's responses and request log.
// folder is the responses folder handler's lookup loads from, where responses
// added in the UI are saved; pass "" to keep them only until the next reload.
func NewUIHandler(handler *MocaRequestHandler, folder string) (*UIHandler, error) {
	u := &UIHandler{
		moca:   handler,
		pages:  make(map[string]*template.Template),
		mux:    http.NewServeMux(),
		logger: slog.Default(),
	}
	if folder != "" {
		header := "# Responses created in the mocka UI. The UI rewrites this file when you add or delete responses in it.\n"
		file, err := loadManagedFile(folder, uiFile, uiDir, header)
		if err != nil {
			return nil, fmt.Errorf("loading UI responses: %w", err)
		}
		u.file = file
	}
	for _, page := range []string{"responses", "test", "requests"} {
		t, err := template.ParseFS(uiFS, "ui/templates/layout.html", "ui/templates/"+page+".html")
		if err != nil {
			return nil, fmt.Errorf("parsing %s page template: %w", page, err)
		}
		u.pages[page] = t
	}
	static, err := fs.Sub(uiFS, "ui/static")
	if err != nil {
		return nil, fmt.Errorf("opening static UI files: %w", err)
	}

	u.mux.Handle("GET "+uiBase+"static/", http.StripPrefix(uiBase+"static/", http.FileServerFS(static)))
	u.mux.HandleFunc("GET "+uiBase+"{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, uiBase+"responses", http.StatusSeeOther)
	})
	u.mux.HandleFunc("GET "+uiBase+"responses", u.handleResponses)
	u.mux.HandleFunc("POST "+uiBase+"responses", u.handleAdd)
	u.mux.HandleFunc("POST "+uiBase+"responses/{id}/delete", u.handleDelete)
	u.mux.HandleFunc("POST "+uiBase+"reload", u.handleReload)
	u.mux.HandleFunc("GET "+uiBase+"test", u.handleTest)
	u.mux.HandleFunc("GET "+uiBase+"requests", u.handleRequests)
	u.mux.HandleFunc("GET "+uiBase+"requests/rows", u.handleRequestRows)
	u.mux.HandleFunc("POST "+uiBase+"requests/clear", u.handleClearRequests)
	return u, nil
}

// RegisterUIRoutes serves the UI under /_mocka/ui/.
func RegisterUIRoutes(router Router, ui http.Handler) {
	router.Handle(uiBase, ui)
}

// ServeHTTP implements http.Handler.
func (u *UIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.mux.ServeHTTP(w, r)
}

// --- Responses page ---

// responseForm holds the add-response form fields as typed.
type responseForm struct {
	Type, Query, Inner, Context, Command, Args, Prefix, Status, Message, Results string
}

type entryRow struct {
	ID       int
	Type     string
	Match    string
	Detail   string
	Status   int
	Message  string
	Rows     int // result rows; -1 without a result set
	Source   string
	Editable bool // saved in the UI file, so deleting it is permanent
}

type responsesPage struct {
	Nav, Notice, Error string
	SavedTo            string // the UI file, or "" when edits are not saved
	Entries            []entryRow
	Form               responseForm
}

func (u *UIHandler) handleResponses(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	form := responseForm{Type: q.Get("type"), Query: q.Get("query"), Status: "0"}
	if form.Type == "" {
		form.Type = string(MatchTypeExact)
	}
	u.renderResponses(w, http.StatusOK, q.Get("notice"), q.Get("error"), form)
}

func (u *UIHandler) renderResponses(w http.ResponseWriter, status int, notice, errMsg string, form responseForm) {
	page := responsesPage{Nav: "responses", Notice: notice, Error: errMsg, Form: form}
	if u.file != nil {
		page.SavedTo = u.file.file
	}
	for _, e := range u.moca.lookup.list() {
		match, detail := describeMatch(e)
		row := entryRow{
			ID: e.id, Type: string(e.MatchType), Match: match, Detail: detail,
			Status: e.StatusCode, Message: e.Message, Rows: -1, Source: e.Source,
			Editable: u.file != nil && u.file.owns(e.Source),
		}
		if row.Source == "" {
			row.Source = "response loader"
		}
		if e.ResultSet != "" {
			if results, err := parseResultSet(e.ResultSet); err == nil {
				row.Rows = len(results.Data.Rows)
			}
		}
		page.Entries = append(page.Entries, row)
	}
	u.render(w, status, "responses", "layout", page)
}

// describeMatch summarizes what an entry matches: the query, inner command,
// command, or prefix, and its context or args.
func describeMatch(e Entry) (string, string) {
	switch e.MatchType {
	case MatchTypeExact:
		return e.Query, ""
	case MatchTypePrefix:
		return e.Prefix + " …", ""
	default:
		name, pairs := e.contextual()
		keys := make([]string, 0, len(pairs))
		for k := range pairs {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+" = "+pairs[k])
		}
		label := "args"
		if e.MatchType == MatchTypePublishData {
			label = "context"
		}
		if len(parts) == 0 {
			return name, "any " + label
		}
		return name, label + ": " + strings.Join(parts, ", ")
	}
}

func (u *UIHandler) handleAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("parsing form: %v", err), http.StatusBadRequest)
		return
	}
	form := responseForm{
		Type: r.PostForm.Get("type"), Query: r.PostForm.Get("query"), Inner: r.PostForm.Get("inner"),
		Context: r.PostForm.Get("context"), Command: r.PostForm.Get("command"), Args: r.PostForm.Get("args"),
		Prefix: r.PostForm.Get("prefix"), Status: r.PostForm.Get("status"), Message: r.PostForm.Get("message"),
		Results: r.PostForm.Get("results"),
	}
	notice, err := u.add(form)
	if err != nil {
		u.renderResponses(w, http.StatusUnprocessableEntity, "", err.Error(), form)
		return
	}
	redirectWith(w, r, "responses", "notice", notice)
}

// add validates form and adds its response, saving it to the UI file when
// there is one. It returns a notice describing what happened.
func (u *UIHandler) add(form responseForm) (string, error) {
	spec, resp, err := form.parse()
	if err != nil {
		return "", err
	}
	e, err := newEntry(spec, resp)
	if err != nil {
		return "", err
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	key := duplicateKey(e)
	var existing *Entry
	for _, other := range u.moca.lookup.list() {
		if duplicateKey(other) == key {
			existing = &other
			break
		}
	}
	if existing != nil && !existing.runtime && (u.file == nil || !u.file.owns(existing.Source)) {
		source := existing.Source
		if source == "" {
			source = "the response loader"
		}
		return "", fmt.Errorf("a response with this match is already defined in %s; change it there", source)
	}

	if u.file == nil {
		e.Source = "UI (not saved)"
		stored, _ := u.moca.lookup.put(e)
		u.logger.Info("response added in UI", "id", stored.id, "type", stored.MatchType)
		return fmt.Sprintf("Added response %d. It lasts until the next reload.", stored.id), nil
	}

	// A runtime entry with the same match would be applied again over the
	// saved one on reload, hiding it, so the saved response replaces it.
	if existing != nil && existing.runtime {
		if err := u.moca.lookup.remove(existing.id); err != nil {
			return "", fmt.Errorf("replacing runtime response %d: %w", existing.id, err)
		}
	}
	raw := rawEntry{Match: spec, RespSpec: responseSpec{Status: resp.Status, Message: resp.Message}}
	if err := u.file.put(raw, resp.Results); err != nil {
		return "", fmt.Errorf("saving to %s: %w", u.file.file, err)
	}
	if err := u.moca.lookup.reload(true); err != nil {
		return "", fmt.Errorf("saved to %s, but reloading the responses failed: %w", u.file.file, err)
	}
	u.logger.Info("response saved from UI", "type", e.MatchType, "file", u.file.file)
	return "Saved to " + u.file.file + ".", nil
}

// parse turns the form into a match and response, using only the match fields
// of the selected type.
func (f responseForm) parse() (matchSpec, matchResponse, error) {
	spec := matchSpec{Type: f.Type}
	var err error
	switch MatchType(f.Type) {
	case MatchTypeExact:
		spec.Query = strings.TrimSpace(f.Query)
	case MatchTypePublishData:
		spec.Inner = f.Inner
		if spec.Context, err = parsePairs(f.Context); err != nil {
			return matchSpec{}, matchResponse{}, fmt.Errorf("context: %w", err)
		}
	case MatchTypeCommand:
		spec.Command = f.Command
		if spec.Args, err = parsePairs(f.Args); err != nil {
			return matchSpec{}, matchResponse{}, fmt.Errorf("args: %w", err)
		}
	case MatchTypePrefix:
		spec.Prefix = f.Prefix
	}
	resp := matchResponse{Message: strings.TrimSpace(f.Message), Results: strings.TrimSpace(f.Results)}
	if s := strings.TrimSpace(f.Status); s != "" {
		if resp.Status, err = strconv.Atoi(s); err != nil {
			return matchSpec{}, matchResponse{}, fmt.Errorf("status %q is not a number", s)
		}
	}
	return spec, resp, nil
}

// parsePairs parses "key = value" lines; blank lines are skipped.
func parsePairs(text string) (map[string]string, error) {
	pairs := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d %q is not key = value", n, line)
		}
		pairs[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return pairs, nil
}

func (u *UIHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		redirectWith(w, r, "responses", "error", fmt.Sprintf("Invalid response id %q.", r.PathValue("id")))
		return
	}
	notice, err := u.delete(id)
	if err != nil {
		redirectWith(w, r, "responses", "error", err.Error())
		return
	}
	redirectWith(w, r, "responses", "notice", notice)
}

// delete removes the response with id: from the UI file when it is saved
// there, otherwise until the next reload. It returns a notice.
func (u *UIHandler) delete(id int) (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	entries := u.moca.lookup.list()
	i := slices.IndexFunc(entries, func(e Entry) bool { return e.id == id })
	if i < 0 {
		return "", fmt.Errorf("there is no response %d; it may have been reloaded", id)
	}
	e := entries[i]
	if u.file != nil && u.file.owns(e.Source) {
		if _, err := u.file.remove(duplicateKey(e)); err != nil {
			return "", fmt.Errorf("deleting from %s: %w", u.file.file, err)
		}
		if err := u.moca.lookup.reload(true); err != nil {
			return "", fmt.Errorf("deleted from %s, but reloading the responses failed: %w", u.file.file, err)
		}
		u.logger.Info("response deleted in UI", "file", u.file.file)
		return fmt.Sprintf("Deleted response %d from %s.", id, u.file.file), nil
	}
	if err := u.moca.lookup.remove(id); err != nil {
		return "", err
	}
	if e.runtime {
		return fmt.Sprintf("Deleted response %d.", id), nil
	}
	source := e.Source
	if source == "" {
		source = "the response loader"
	}
	return fmt.Sprintf("Removed response %d until the next reload. It is defined in %s.", id, source), nil
}

func (u *UIHandler) handleReload(w http.ResponseWriter, r *http.Request) {
	u.mu.Lock()
	err := u.moca.lookup.reload(false)
	u.mu.Unlock()
	if err != nil {
		redirectWith(w, r, "responses", "error", fmt.Sprintf("Reloading failed, so the current responses were kept: %v", err))
		return
	}
	redirectWith(w, r, "responses", "notice", fmt.Sprintf("Reloaded %d responses.", len(u.moca.lookup.list())))
}

// --- Test page ---

type resultCell struct {
	Value string
	Null  bool
}

type testPage struct {
	Nav, Query, Normalized string
	Tested                 bool
	Match                  string // what handled the query; "" when nothing matched
	Detail, Source         string
	ID                     int
	Builtin                bool
	Status                 int
	Message                string
	Columns                []string
	Rows                   [][]resultCell
	CreateURL              string
}

func (u *UIHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	page := testPage{Nav: "test", Query: r.URL.Query().Get("query")}
	if strings.TrimSpace(page.Query) != "" {
		page.Tested = true
		page.Normalized = normalizeQuery(page.Query)
		if cmd := builtinCommand(page.Normalized); cmd != "" {
			page.Builtin, page.Match = true, "built-in "+cmd+" command"
		} else {
			resp, e := u.moca.lookup.resolve(page.Normalized)
			page.Status, page.Message = resp.StatusCode, resp.Message
			if e != nil {
				page.ID, page.Source = e.id, e.Source
				match, detail := describeMatch(*e)
				page.Match, page.Detail = string(e.MatchType)+": "+match, detail
			} else {
				page.CreateURL = createURL(page.Query)
			}
			if resp.ResultSet != "" {
				if results, err := parseResultSet(resp.ResultSet); err == nil {
					for _, c := range results.Metadata.Columns {
						page.Columns = append(page.Columns, c.Name+" ("+string(c.Type)+")")
					}
					for _, row := range results.Data.Rows {
						cells := make([]resultCell, 0, len(row.Fields))
						for _, f := range row.Fields {
							cells = append(cells, resultCell{Value: f.Value, Null: f.Null == "true"})
						}
						page.Rows = append(page.Rows, cells)
					}
				}
			}
		}
	}
	u.render(w, http.StatusOK, "test", "layout", page)
}

// --- Requests page ---

type requestRow struct {
	Time, Query string
	Status      int
	Handled     string
	Unmatched   bool
	TestURL     string
	CreateURL   string
}

type requestsPage struct {
	Nav           string
	UnmatchedOnly bool
	RowsURL       string
	Rows          []requestRow
}

func (u *UIHandler) requestsPage(r *http.Request) requestsPage {
	page := requestsPage{Nav: "requests", UnmatchedOnly: r.URL.Query().Get("unmatched") == "1", RowsURL: uiBase + "requests/rows"}
	if page.UnmatchedOnly {
		page.RowsURL += "?unmatched=1"
	}
	records := u.moca.Requests()
	for i := len(records) - 1; i >= 0; i-- { // newest first
		rec := records[i]
		if page.UnmatchedOnly && !rec.Unmatched {
			continue
		}
		row := requestRow{
			Time: rec.Time.Format("15:04:05"), Query: rec.Query, Status: rec.Status,
			Unmatched: rec.Unmatched, TestURL: uiBase + "test?query=" + url.QueryEscape(rec.Query),
		}
		switch {
		case rec.Unmatched:
			row.Handled = "no response"
			row.CreateURL = createURL(rec.Query)
		case rec.Match == nil:
			row.Handled = "rejected (invalid session)"
		case rec.Match.Type == "builtin":
			row.Handled = "built-in " + rec.Match.Command
		default:
			row.Handled = rec.Match.Type
			if rec.Match.Source != "" {
				row.Handled += " · " + rec.Match.Source
			}
		}
		page.Rows = append(page.Rows, row)
	}
	return page
}

func (u *UIHandler) handleRequests(w http.ResponseWriter, r *http.Request) {
	u.render(w, http.StatusOK, "requests", "layout", u.requestsPage(r))
}

// handleRequestRows renders only the table body, which the requests page
// polls.
func (u *UIHandler) handleRequestRows(w http.ResponseWriter, r *http.Request) {
	u.render(w, http.StatusOK, "requests", "request_rows", u.requestsPage(r))
}

func (u *UIHandler) handleClearRequests(w http.ResponseWriter, r *http.Request) {
	u.moca.ClearRequests()
	http.Redirect(w, r, uiBase+"requests", http.StatusSeeOther)
}

// --- Helpers ---

// createURL links to the add form, filled in with an exact match for query.
func createURL(query string) string {
	return uiBase + "responses?type=exact&query=" + url.QueryEscape(query) + "#add"
}

// redirectWith redirects to a UI page with a notice or error message.
func redirectWith(w http.ResponseWriter, r *http.Request, page, param, msg string) {
	http.Redirect(w, r, uiBase+page+"?"+param+"="+url.QueryEscape(msg), http.StatusSeeOther)
}

func (u *UIHandler) render(w http.ResponseWriter, status int, page, name string, data any) {
	var buf bytes.Buffer
	if err := u.pages[page].ExecuteTemplate(&buf, name, data); err != nil {
		u.logger.Error("rendering UI page", "page", page, "error", err)
		http.Error(w, "rendering page failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

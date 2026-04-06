package mocka

const (
	StatusOK                = 0
	StatusSrvNoDataFound    = 510
	StatusDBNoDataFound     = -1403
	StatusDBError           = 511
	StatusGroovyException   = 531
	StatusCommandNotFound   = 501
	StatusInvalidSessionKey = 523
)

// MatchType identifies how an Entry matches incoming queries (YAML match.type).
type MatchType string

const (
	MatchTypeExact       MatchType = "exact"
	MatchTypePublishData MatchType = "publish_data"
	MatchTypePrefix      MatchType = "prefix"
)

// Response is the runtime result returned by the matcher, containing the mocked result data.
type Response struct {
	StatusCode int
	Message    string
	ResultSet  string
}

// Entry is a fully resolved, normalized response entry ready for matching.
type Entry struct {
	MatchType  MatchType
	Query      string            // normalized; used for exact match
	Inner      string            // normalized; used for publish_data match
	Context    map[string]string // normalized values; used for publish_data contextual match
	Prefix     string            // normalized; used for prefix match
	StatusCode int
	Message    string
	ResultSet  string // pre-loaded XML content
}

func (e *Entry) response() Response {
	return Response{
		StatusCode: e.StatusCode,
		Message:    e.Message,
		ResultSet:  e.ResultSet,
	}
}

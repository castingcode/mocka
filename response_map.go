package mocka

import (
	"errors"
	"fmt"
	"strings"
)

const (
	StatusOK                = 0
	StatusSrvNoDataFound    = 510
	StatusDBNoDataFound     = -1403
	StatusDBError           = 511
	StatusGroovyException   = 531
	StatusCommandNotFound   = 501
	StatusInvalidSessionKey = 523
)

// Response contains the mocked status code, message, and result set
type Response struct {
	StatusCode int    `yaml:"status"`
	Message    string `yaml:"message,omitempty"`
	ResultSet  string `yaml:"results,omitempty"`
}

// contains repsones for all users
type ResponseMap map[string]Response

// contains user-specific responses
type UserResponseMap map[string]ResponseMap

// ResponseLookup stores the responses for all users and user-specific responses
type ResponseLookup struct {
	userResponses    UserResponseMap
	allUserResponses ResponseMap
	dataFolder       string
}

var ErrrNoMatchFound = errors.New("no match found")

type ResponseLookupOption func(*ResponseLookup)

func WithDataFolder(folder string) ResponseLookupOption {
	return func(r *ResponseLookup) {
		r.dataFolder = folder
	}
}

func NewResponseLookup(opts ...ResponseLookupOption) (*ResponseLookup, error) {
	responseLookup := &ResponseLookup{
		dataFolder: "responses",
	}
	for _, opt := range opts {
		opt(responseLookup)
	}

	responseMap, err := ResponseMapFromFile(fmt.Sprintf("%s/responses.yml", responseLookup.dataFolder))
	if err != nil {
		return nil, fmt.Errorf("could parse responses.yml %w", err)
	}

	userResponseMap, err := UserResponseMapFromFile(fmt.Sprintf("%s/user_responses.yml", responseLookup.dataFolder))
	if err != nil {
		return nil, fmt.Errorf("could parse user responses.yml %w", err)
	}
	responseLookup.allUserResponses = responseMap
	responseLookup.userResponses = userResponseMap

	return responseLookup, nil
}

// GetResponse handles getting a lookup first by exact match first, then
// gets rid of the where clause and searches just on the command name.
// It does not handle groovy or SQL, just local syntax.
func (r ResponseLookup) GetResponse(user, query string) Response {
	response, err := r.getResponse(user, query)
	if err == nil {
		return response
	}

	// if starts with Groovy, return groovy-ish error; we're only exact matching on groovy
	if strings.HasPrefix(query, "[[") {
		return Response{
			StatusCode: StatusGroovyException,
			Message:    "Groovy Script Exception: java.lang.NullPointerException", // this would have a more detailed exception message, but we're not doing that here
		}
	}

	// if starts SQL, return SQL-ish error; we're only exact matching on SQL
	if strings.HasPrefix(query, "[") {
		return Response{
			StatusCode: StatusDBError,
			Message:    "Database Error: 511 - Invalid object name", // this would have the name of the object that wasn't found, but we're not doing that here
		}
	}

	// else, if it starts with local syntax,
	// we'll try matching on just the command regardless of the where clause or anything after that
	parts := strings.Split(query, " where ") // TODO: Make this regex to not get tripped up by whitespace
	commandName := strings.TrimSpace(parts[0])
	response, err = r.getResponse(user, commandName)
	if err == nil {
		return response
	}
	return Response{
		StatusCode: StatusCommandNotFound,
		Message:    fmt.Sprintf("Command (%s) not found", parts[0]),
	}
}

// getResponse handles the lookup for a user specific version first and a global
// version next.  It only handles exact matches on syntax of the request.
func (r ResponseLookup) getResponse(user, query string) (Response, error) {
	if m, ok := r.userResponses[user]; ok {
		if response, ok := m[query]; ok {
			return response, nil
		}
	}
	if response, ok := r.allUserResponses[query]; ok {
		return response, nil
	}
	return Response{}, ErrrNoMatchFound
}

package mocka

import (
	"fmt"
	"os"
	"strings"
)

// ResponseBuilder constructs a Response for use with InMemoryResponseLoader
// options. Use NewResponse to obtain a builder.
type ResponseBuilder struct {
	statusCode int
	message    string
	resultSet  string
}

// NewResponse returns a ResponseBuilder for the given HTTP/MOCA status code.
func NewResponse(statusCode int) *ResponseBuilder {
	return &ResponseBuilder{statusCode: statusCode}
}

// WithMessage sets the message on the response.
func (b *ResponseBuilder) WithMessage(message string) *ResponseBuilder {
	b.message = message
	return b
}

// WithResultSet sets the result XML directly from a string.
func (b *ResponseBuilder) WithResultSet(xml string) *ResponseBuilder {
	b.resultSet = xml
	return b
}

// WithResultSetFromFile reads the result XML from the given file path.
// Returns an error if the file cannot be read.
func (b *ResponseBuilder) WithResultSetFromFile(path string) (*ResponseBuilder, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading result set from %s: %w", path, err)
	}
	b.resultSet = strings.TrimSpace(string(data))
	return b, nil
}

// Build returns the constructed Response.
func (b *ResponseBuilder) Build() Response {
	return Response{
		StatusCode: b.statusCode,
		Message:    b.message,
		ResultSet:  b.resultSet,
	}
}

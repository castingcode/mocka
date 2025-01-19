package mocka

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"math/rand"

	"github.com/castingcode/mocaprotocol"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

func TestHandleMocaRequest_Ping(t *testing.T) {

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := t.TempDir()
		lookup, _ := NewResponseLookup(WithDataFolder(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to login with a valid user name and password", func() {

			req := buildRequest(t, "ping")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK with a session key and lifetime?", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, 0)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_Login(t *testing.T) {

	Convey("Given I have a MocaRequestHandler", t, func() {

		uuid.SetRand(rand.New(rand.NewSource(1))) // make UUID deterministic
		responseDirectory := t.TempDir()
		lookup, _ := NewResponseLookup(WithDataFolder(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to login with a valid user name and password", func() {

			req := buildRequest(t, "login user where usr_id = 'anyuser' and usr_pswd = 'anypass'")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK with a session key", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, 0)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 14)
				So(response.MocaResults.Metadata.Columns[0].Name, ShouldEqual, "usr_id")
				So(response.MocaResults.Metadata.Columns[1].Name, ShouldEqual, "locale_id")
				So(response.MocaResults.Metadata.Columns[4].Name, ShouldEqual, "session_key")
				So(response.MocaResults.Metadata.Columns[7].Name, ShouldEqual, "pswd_disable")
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields, ShouldHaveLength, 14)
				So(response.MocaResults.Data.Rows[0].Fields[0].Value, ShouldEqual, "anyuser")
				So(response.MocaResults.Data.Rows[0].Fields[1].Value, ShouldEqual, "US_ENGLISH")
				So(response.MocaResults.Data.Rows[0].Fields[4].Value, ShouldEqual, "52fdfc07-2182-454f-963f-5f0f9a621d72")
				So(response.MocaResults.Data.Rows[0].Fields[7].Value, ShouldEqual, "6008")

			})
		})

		Convey("When I attempt to login with a valid user name and no password", func() {

			req := buildRequest(t, "login user where usr_id = 'anyuser'")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK with a session key", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, 802)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
				So(response.Message, ShouldEqual, "Missing argument: Password (usr_pswd)")
			})
		})
	})
}

func TestHandleMocaRequest_Logout(t *testing.T) {

	Convey("Given I have a MocaRequestHandler", t, func() {
		sessionKey := uuid.NewString()
		responseDirectory := t.TempDir()
		lookup, _ := NewResponseLookup(WithDataFolder(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions[sessionKey] = "super"
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to logout with a valid session key", func() {

			req := buildRequest(t, "logout user", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK with a session key", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, 0)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
				So(handler.sessions, ShouldBeEmpty)
			})
		})

		Convey("When I attempt to logout with an invalid session key", func() {

			req := buildRequest(t, "logout user", WithSessionKey(uuid.NewString()))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK with a session key", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, 523)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
				So(response.Message, ShouldEqual, "Invalid session key")
			})
		})
	})
}

func TestHandleMocaRequest_NoContentType(t *testing.T) {

	Convey("Given I have a MocaRequestHandler", t, func() {
		lookup, _ := NewResponseLookup()
		handler := NewMocaRequestHandler(lookup)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to send a command with no content type header", func() {

			request := &mocaprotocol.MocaRequest{
				Autocommit: "true",
				Query:      mocaprotocol.Query{Text: "ping"},
			}
			requestBody, err := xml.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			req, err := http.NewRequest("POST", "/service", bytes.NewReader(requestBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be an html error page", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				So(w.Body.String(), ShouldStartWith, "<html>")
				So(w.Header().Get("Content-Type"), ShouldEqual, "text/html; charset=utf-8")
			})
		})
	})
}

func TestHandleMocaRequest_NotAuthenticated(t *testing.T) {

	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := creteEmptyResponseFiles(t)
		lookup, _ := NewResponseLookup(WithDataFolder(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions[sessionKey] = "super"
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to run a moca command without a session key", func() {

			req := buildRequest(t, "list shipments")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be invalid sessin key", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusInvalidSessionKey)
				So(response.Message, ShouldEqual, "Invalid session key")
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_InvalidSQL(t *testing.T) {

	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := creteEmptyResponseFiles(t)
		lookup, _ := NewResponseLookup(WithDataFolder(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions[sessionKey] = "super"
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to an unexpected SQL Statement", func() {

			req := buildRequest(t, "[select * from notable]", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusDBError", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusDBError)
				So(response.Message, ShouldEqual, "Database Error: 511 - Invalid object name")
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_ValidSQL(t *testing.T) {

	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := createResponseFiles(t, "testdata/sql_responses.yml", "")
		lookup, err := NewResponseLookup(WithDataFolder(responseDirectory))
		if err != nil {
			t.Fatal(err)
		}
		handler := NewMocaRequestHandler(lookup)
		handler.sessions[sessionKey] = "super"
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to a SQL Statement that returns results", func() {
			sql := `[select 'x' as myval
			           from dual where 1=1]`
			req := buildRequest(t, sql, WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusOK)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 1)
				So(response.MocaResults.Metadata.Columns[0].Name, ShouldEqual, "myval")
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields[0].Value, ShouldEqual, "x")
			})
		})

		Convey("When I attempt to a SQL Statement that returns an error", func() {
			sql := `[select 'x' as myval
			           from dual where 1=2]`
			req := buildRequest(t, sql, WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusDBError", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusDBNoDataFound)
				So(response.Message, ShouldEqual, "No Data Found")
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_InvalidGroovy(t *testing.T) {

	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := creteEmptyResponseFiles(t)
		lookup, _ := NewResponseLookup(WithDataFolder(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions[sessionKey] = "super"
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to an unexpected Groovy Statement", func() {

			req := buildRequest(t, "[[com.example.NoObject.doNothing()]]", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusGroovyException", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusGroovyException)
				So(response.Message, ShouldEqual, "Groovy Script Exception: java.lang.NullPointerException")
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_ValidGroovy(t *testing.T) {
	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := createResponseFiles(t, "testdata/groovy_responses.yml", "")
		lookup, err := NewResponseLookup(WithDataFolder(responseDirectory))
		if err != nil {
			t.Fatal(err)
		}
		handler := NewMocaRequestHandler(lookup)
		handler.sessions[sessionKey] = "super"
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to run a Groovy Statement that returns results", func() {
			sql := `[[
			def numbers = [1, 2, 3] 
			x = numbers[0]
			]]`
			req := buildRequest(t, sql, WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusOK)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 1)
				So(response.MocaResults.Metadata.Columns[0].Name, ShouldEqual, "x")
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields[0].Value, ShouldEqual, "1")
			})
		})

		Convey("When I attempt to run a Groovy Statement that returns an error", func() {
			sql := `[[ throw new Exception('error') ]]`
			req := buildRequest(t, sql, WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusGroovyException", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusGroovyException)
				So(response.Message, ShouldEqual, "Groovy Script Exception: java.lang.Exception: error")
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_InvalidLocalSyntax(t *testing.T) {

	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := creteEmptyResponseFiles(t)
		lookup, _ := NewResponseLookup(WithDataFolder(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions[sessionKey] = "super"
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to an unexpected Local Syntax", func() {

			req := buildRequest(t, "list players", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusCommandNotFound", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusCommandNotFound)
				So(response.Message, ShouldEqual, "Command (list players) not found")
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_ValidLocalSyntax(t *testing.T) {
	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := createResponseFiles(t, "testdata/local_syntax.yml", "")
		lookup, err := NewResponseLookup(WithDataFolder(responseDirectory))
		if err != nil {
			t.Fatal(err)
		}
		handler := NewMocaRequestHandler(lookup)
		handler.sessions[sessionKey] = "super"
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to run local syntax by exact match that returns results", func() {
			sql := `publish usr data
			where a = 'foo'`
			req := buildRequest(t, sql, WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusOK)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 1)
				So(response.MocaResults.Metadata.Columns[0].Name, ShouldEqual, "a")
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields[0].Value, ShouldEqual, "foo")
			})
		})

		Convey("When I attempt to run local syntax by command name match that returns results", func() {
			sql := `publish usr data
			where name = 'bub'`
			req := buildRequest(t, sql, WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusOK)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 1)
				So(response.MocaResults.Metadata.Columns[0].Name, ShouldEqual, "a")
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields, ShouldHaveLength, 1)
				So(response.MocaResults.Data.Rows[0].Fields[0].Value, ShouldEqual, "bar")
			})
		})

		Convey("When I attempt to run local syntaxt that returns an error", func() {
			sql := `publish usr data
			where a = 'bar'`
			req := buildRequest(t, sql, WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusGroovyException", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, 90001)
				So(response.Message, ShouldEqual, "this is really unexpected")
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

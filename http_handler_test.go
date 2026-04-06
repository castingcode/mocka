package mocka

import (
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
		lookup, _ := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I send a ping command", func() {

			req := buildRequest(t, "ping")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK with no results", func() {
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
		lookup, _ := NewResponseLookup(NewFileResponseLoader(responseDirectory))
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

			Convey("Then the response should be an error about missing password", func() {
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
		lookup, _ := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions.Add(sessionKey, "super")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to logout with a valid session key", func() {

			req := buildRequest(t, "logout user", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK and the session should be removed", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, 0)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
				So(handler.sessions.Len(), ShouldEqual, 0)
			})
		})

		Convey("When I attempt to logout with an invalid session key", func() {

			req := buildRequest(t, "logout user", WithSessionKey(uuid.NewString()))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be invalid session key", func() {
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
		responseDirectory := t.TempDir()
		lookup, _ := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to send a command with no content type header", func() {

			req, err := http.NewRequest("POST", "/service", nil)
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
		lookup, _ := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions.Add(sessionKey, "super")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to run a moca command without a session key", func() {

			req := buildRequest(t, "list shipments")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be invalid session key", func() {
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
		lookup, _ := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions.Add(sessionKey, "super")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt an unregistered SQL statement", func() {

			req := buildRequest(t, "[select * from notable]", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusCommandNotFound", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusCommandNotFound)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_ValidSQL(t *testing.T) {

	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := createResponseDir(t, "testdata/sql")
		lookup, err := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		if err != nil {
			t.Fatal(err)
		}
		handler := NewMocaRequestHandler(lookup)
		handler.sessions.Add(sessionKey, "super")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt a SQL statement that returns results", func() {
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

		Convey("When I attempt a SQL statement that returns an error", func() {
			sql := `[select 'x' as myval
			           from dual where 1=2]`
			req := buildRequest(t, sql, WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusDBNoDataFound", func() {
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
		lookup, _ := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions.Add(sessionKey, "super")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt an unregistered Groovy statement", func() {

			req := buildRequest(t, "[[com.example.NoObject.doNothing()]]", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be StatusCommandNotFound", func() {
				So(w.Code, ShouldEqual, http.StatusOK)
				var response mocaprotocol.MocaResponse
				err := xml.Unmarshal(w.Body.Bytes(), &response)
				So(err, ShouldBeNil)
				So(response.Status, ShouldEqual, StatusCommandNotFound)
				So(response.MocaResults.Metadata.Columns, ShouldHaveLength, 0)
				So(response.MocaResults.Data.Rows, ShouldHaveLength, 0)
			})
		})
	})
}

func TestHandleMocaRequest_ValidGroovy(t *testing.T) {
	sessionKey := uuid.NewString()

	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := createResponseDir(t, "testdata/groovy")
		lookup, err := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		if err != nil {
			t.Fatal(err)
		}
		handler := NewMocaRequestHandler(lookup)
		handler.sessions.Add(sessionKey, "super")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt to run a Groovy statement that returns results", func() {
			groovy := `[[
			def numbers = [1, 2, 3]
			x = numbers[0]
			]]`
			req := buildRequest(t, groovy, WithSessionKey(sessionKey))
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

		Convey("When I attempt to run a Groovy statement that returns an error", func() {
			groovy := `[[ throw new Exception('error') ]]`
			req := buildRequest(t, groovy, WithSessionKey(sessionKey))
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
		lookup, _ := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		handler := NewMocaRequestHandler(lookup)
		handler.sessions.Add(sessionKey, "super")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I attempt an unregistered local syntax command", func() {

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

		responseDirectory := createResponseDir(t, "testdata/local")
		lookup, err := NewResponseLookup(NewFileResponseLoader(responseDirectory))
		if err != nil {
			t.Fatal(err)
		}
		handler := NewMocaRequestHandler(lookup)
		handler.sessions.Add(sessionKey, "super")
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router, handler)

		Convey("When I run local syntax that matches exactly", func() {
			req := buildRequest(t, "publish usr data where a = 'foo'", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK with the exact-match result", func() {
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

		Convey("When I run local syntax that matches by prefix", func() {
			req := buildRequest(t, "publish usr data where name = 'bub'", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should be OK with the prefix-match result", func() {
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

		Convey("When I run local syntax that returns an error", func() {
			req := buildRequest(t, "publish usr data where a = 'bar'", WithSessionKey(sessionKey))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Convey("Then the response should contain the registered error", func() {
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

package mocka

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"math/rand"

	"github.com/castingcode/mocaprotocol"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	. "github.com/smartystreets/goconvey/convey"
)

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
	})
}

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

func TestHandleMocaRequest(t *testing.T) {
	// Given I have a MocaRequestHandler
	// When I call HandleMocaRequest
	// Then the response should be OK
	Convey("Given I have a MocaRequestHandler", t, func() {

		responseDirectory := t.TempDir()

		Convey("When the command is mapped for all users", func() {
			data, err := os.ReadFile("testdata/responses.yml")
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(fmt.Sprintf("%s/responses.yml", responseDirectory), []byte(data), 0644)
			if err != nil {
				t.Fatal(err)
			}
			_, err = os.Create(fmt.Sprintf("%s/user_responses.yml", responseDirectory))
			if err != nil {
				t.Fatal(err)
			}
			lookup, _ := NewResponseLookup(WithDataFolder(responseDirectory))
			handler := NewMocaRequestHandler(lookup)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			RegisterRoutes(router, handler)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/service", nil)
			router.ServeHTTP(w, req)

			fmt.Println(w.Body.String())
			fmt.Println(w.Code)

		})

	})

}

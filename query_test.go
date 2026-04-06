package mocka

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNormalizeQuery(t *testing.T) {

	Convey("normalizeQuery", t, func() {

		Convey("collapses whitespace", func() {

			Convey("newline after a keyword is treated as a space", func() {
				So(normalizeQuery("publish data\nwhere a = 1"), ShouldEqual, normalizeQuery("publish data where a = 1"))
			})

			Convey("multiple spaces are collapsed to a single space", func() {
				So(normalizeQuery("publish  data   where  a  =  1"), ShouldEqual, normalizeQuery("publish data where a = 1"))
			})

			Convey("leading and trailing whitespace is trimmed", func() {
				So(normalizeQuery("  publish data where a = 1  "), ShouldEqual, "publish data where a = 1")
			})
		})

		Convey("is case-insensitive", func() {

			Convey("uppercased keyword matches lowercased equivalent", func() {
				So(normalizeQuery("PUBLISH DATA"), ShouldEqual, normalizeQuery("publish data"))
			})

			Convey("mixed-case query matches lowercased equivalent", func() {
				So(normalizeQuery("Publish Data Where A = 1"), ShouldEqual, normalizeQuery("publish data where a = 1"))
			})
		})

		Convey("local syntax — quotes", func() {

			Convey("single-quoted value matches double-quoted equivalent", func() {
				So(normalizeQuery("publish data where a = 'foo'"), ShouldEqual, normalizeQuery(`publish data where a = "foo"`))
			})

			Convey("multiple quoted values — all pairs are interchangeable", func() {
				So(
					normalizeQuery("list things where a = 'foo' and b = 'bar'"),
					ShouldEqual,
					normalizeQuery(`list things where a = "foo" and b = "bar"`),
				)
			})
		})

		Convey("embedded SQL — bracket syntax [...]", func() {

			Convey("whitespace inside brackets is normalized", func() {
				So(normalizeQuery("[ select * from dual ]"), ShouldEqual, normalizeQuery("[select * from dual]"))
			})

			Convey("newlines inside brackets are normalized", func() {
				So(normalizeQuery("[\nselect *\nfrom dual\n]"), ShouldEqual, normalizeQuery("[select * from dual]"))
			})

			Convey("single and double quotes inside SQL brackets are NOT interchangeable", func() {
				So(
					normalizeQuery("[select * from dual where x = 'foo']"),
					ShouldNotEqual,
					normalizeQuery(`[select * from dual where x = "foo"]`),
				)
			})
		})

		Convey("embedded Groovy — double-bracket syntax [[...]]", func() {

			Convey("whitespace inside double brackets is normalized", func() {
				So(normalizeQuery("[[ println 'hello' ]]"), ShouldEqual, normalizeQuery("[[println 'hello']]"))
			})

			Convey("newlines inside double brackets are normalized", func() {
				So(normalizeQuery("[[\nprintln 'hello'\n]]"), ShouldEqual, normalizeQuery("[[println 'hello']]"))
			})

			Convey("single and double quotes inside Groovy brackets are NOT interchangeable", func() {
				So(
					normalizeQuery("[[println 'hello']]"),
					ShouldNotEqual,
					normalizeQuery(`[[println "hello"]]`),
				)
			})
		})
	})
}

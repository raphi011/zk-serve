package zk_test

import (
	"testing"

	"github.com/raphaelgruber/zk-serve/internal/zk"
)

func TestConvertQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// bare terms → quoted
		{input: "foo", want: `"foo"`},
		{input: "foo bar", want: `"foo" "bar"`},

		// already quoted → preserved
		{input: `"foo bar"`, want: `"foo bar"`},

		// mixed quoted/unquoted
		{input: `foo "bar baz" qux`, want: `"foo" "bar baz" "qux"`},

		// prefix wildcard
		{input: "foo*", want: `"foo"*`},

		// negation
		{input: "-foo", want: `NOT "foo"`},

		// pipe = OR
		{input: "foo|bar", want: `"foo" OR "bar"`},

		// operators pass through
		{input: "foo AND bar", want: `"foo" AND "bar"`},
		{input: "foo OR bar", want: `"foo" OR "bar"`},
		{input: "NOT foo", want: `NOT "foo"`},

		// column prefix
		{input: "title:foo", want: `title:"foo"`},

		// bare quote → empty
		{input: `"`, want: ""},

		// empty input
		{input: "", want: ""},

		// negation + prefix
		{input: "-foo*", want: `NOT "foo"*`},

		// column + prefix
		{input: "title:foo*", want: `title:"foo"*`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := zk.ConvertQuery(tt.input)
			if got != tt.want {
				t.Errorf("ConvertQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

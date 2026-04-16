package postgres

import "testing"

func TestWrapQueryUsesBoundedDefaultLimit(t *testing.T) {
	got := wrapQuery("SELECT * FROM accounts", 0)
	want := "SELECT * FROM (SELECT * FROM accounts) AS _limited LIMIT 100"
	if got != want {
		t.Fatalf("unexpected wrapped query:\n got: %q\nwant: %q", got, want)
	}
}

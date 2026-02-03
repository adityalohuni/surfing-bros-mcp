package page

import "testing"

func TestReducerExtractsTextAndElements(t *testing.T) {
	reducer := NewReducer(ReduceOptions{MaxText: 50, MaxElements: 2})
	input := RawPage{
		URL:  "https://example.com",
		HTML: `<html><body><a href="/start" id="start">Start here</a><button>Go</button><p>Some text</p></body></html>`,
	}
	snap := reducer.Reduce(input)
	if snap.Text == "" {
		t.Fatalf("expected text to be extracted")
	}
	if len(snap.Elements) == 0 {
		t.Fatalf("expected elements to be extracted")
	}
}

package agents

import (
	"sync"
	"testing"
)

func TestIssuesAddAndList(t *testing.T) {
	var is Issues
	if got := is.List(); len(got) != 0 {
		t.Fatalf("fresh collector should be empty, got %d", len(got))
	}

	is.Add("document_operator", IssueFetchPartial, "2 pdfs failed", "run-1")
	list := is.List()
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	got := list[0]
	if got.SourceAgent != "document_operator" || got.Code != IssueFetchPartial ||
		got.Message != "2 pdfs failed" || got.Context != "run-1" {
		t.Errorf("unexpected issue mapping: %+v", got)
	}
}

func TestIssuesListReturnsCopy(t *testing.T) {
	var is Issues
	is.Add("a", "code", "msg", "")

	list := is.List()
	list[0].Message = "mutated"

	if is.List()[0].Message != "msg" {
		t.Error("mutating the returned slice must not affect the collector")
	}
}

func TestIssuesConcurrentAdd(t *testing.T) {
	var is Issues
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			is.Add("agent", "code", "msg", "")
		}()
	}
	wg.Wait()
	if got := len(is.List()); got != 50 {
		t.Errorf("len = %d, want 50", got)
	}
}

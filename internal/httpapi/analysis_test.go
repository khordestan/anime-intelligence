package httpapi

import (
	"testing"
	"time"
)

func TestAnalysisBudgetLimitsOwnerAndInFlightWork(t *testing.T) {
	b := &analysisBudget{}
	now := time.Now()
	if !b.acquire("a", now) || !b.acquire("b", now) || b.acquire("c", now) {
		t.Fatal("concurrency limit did not apply")
	}
	b.release()
	b.release()
	for range 2 {
		if !b.acquire("a", now) {
			t.Fatal("owner limit applied too early")
		}
		b.release()
	}
	if b.acquire("a", now) || !b.acquire("a", now.Add(time.Minute)) {
		t.Fatal("owner window did not reset")
	}
	b.release()
}

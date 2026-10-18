package recurringjob

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

func TestResolveDate_NormalDay(t *testing.T) {
	d := resolveDate(2026, time.March, 15)
	assert.Equal(t, time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), d)
}

func TestResolveDate_DueDayExceedsFebruary(t *testing.T) {
	d := resolveDate(2026, time.February, 31)
	assert.Equal(t, time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC), d)
}

func TestResolveDate_DueDayExceedsFebruaryLeap(t *testing.T) {
	d := resolveDate(2024, time.February, 31)
	assert.Equal(t, time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC), d)
}

func TestResolveDate_DueDayExceedsApril(t *testing.T) {
	d := resolveDate(2026, time.April, 31)
	assert.Equal(t, time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC), d)
}

func TestResolveDate_Day1(t *testing.T) {
	d := resolveDate(2026, time.January, 1)
	assert.Equal(t, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), d)
}

func TestDeterministicClientID_SameInput(t *testing.T) {
	id1 := deterministicClientID("rec-uuid", 2026, 10)
	id2 := deterministicClientID("rec-uuid", 2026, 10)
	assert.Equal(t, id1, id2)
}

func TestDeterministicClientID_DifferentMonth(t *testing.T) {
	id1 := deterministicClientID("rec-uuid", 2026, 10)
	id2 := deterministicClientID("rec-uuid", 2026, 11)
	assert.NotEqual(t, id1, id2)
}

func TestDeterministicClientID_DifferentYear(t *testing.T) {
	id1 := deterministicClientID("rec-uuid", 2026, 10)
	id2 := deterministicClientID("rec-uuid", 2027, 10)
	assert.NotEqual(t, id1, id2)
}

func TestDeterministicClientID_DifferentRecurring(t *testing.T) {
	id1 := deterministicClientID("rec-A", 2026, 10)
	id2 := deterministicClientID("rec-B", 2026, 10)
	assert.NotEqual(t, id1, id2)
}

func TestRunForCurrentMonth_DoesNotPanic(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	j := New(db)
	// RunForCurrentMonth is the cron entry point; just verify it runs without panic.
	assert.NotPanics(t, func() { j.RunForCurrentMonth() })
}

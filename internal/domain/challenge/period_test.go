package challenge

import (
	"testing"
	"time"

	"github.com/rowlys/panjang-umur-backend/internal/models"
)

func TestDailyPeriodStartsAtLocalMidnight(t *testing.T) {
	// 02:00 WIB is still the previous day in UTC.
	at := time.Date(2026, 11, 16, 2, 0, 0, 0, appZone)
	got := currentPeriodStart(models.Daily, at, 0)

	want := time.Date(2026, 11, 16, 0, 0, 0, 0, appZone)
	if !got.Equal(want) {
		t.Fatalf("period start = %s, want %s", got, want)
	}

	// Midnight WIB is 17:00 UTC on the day before.
	if got.UTC() != time.Date(2026, 11, 15, 17, 0, 0, 0, time.UTC) {
		t.Fatalf("period start in UTC = %s, want 2026-11-15T17:00:00Z", got.UTC())
	}
}

func TestDailyPeriodIsStableAcrossLocalDay(t *testing.T) {
	morning := time.Date(2026, 11, 16, 8, 0, 0, 0, appZone)
	lateNight := time.Date(2026, 11, 16, 23, 30, 0, 0, appZone)

	if !currentPeriodStart(models.Daily, morning, 0).Equal(currentPeriodStart(models.Daily, lateNight, 0)) {
		t.Fatal("times on the same local day should share a period")
	}

	nextDay := time.Date(2026, 11, 17, 0, 5, 0, 0, appZone)
	if currentPeriodStart(models.Daily, nextDay, 0).Equal(currentPeriodStart(models.Daily, lateNight, 0)) {
		t.Fatal("a new local day should start a new period")
	}
}

func TestWeeklyPeriodStartsOnResetDay(t *testing.T) {
	// Tuesday 02:00 WIB, reset day Monday (time.Weekday == 1).
	at := time.Date(2026, 11, 17, 2, 0, 0, 0, appZone)
	got := currentPeriodStart(models.Weekly, at, int(time.Monday))

	want := time.Date(2026, 11, 16, 0, 0, 0, 0, appZone)
	if !got.Equal(want) {
		t.Fatalf("period start = %s, want %s", got, want)
	}
}

func TestBountyHasNoPeriod(t *testing.T) {
	if got := currentPeriodStart(models.Bounty, time.Now(), 0); !got.IsZero() {
		t.Fatalf("bounty period = %s, want zero time", got)
	}
}

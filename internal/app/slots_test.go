package app

import (
	"testing"
	"time"
)

func TestValidateSchedule(t *testing.T) {
	ok := Schedule{RoomID: "r1", DaysOfWeek: []int{1, 3, 5}, StartTime: "09:00", EndTime: "12:00"}
	if err := ValidateSchedule(ok); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	bad := []Schedule{
		{RoomID: "r1", DaysOfWeek: []int{0}, StartTime: "09:00", EndTime: "12:00"},
		{RoomID: "r1", DaysOfWeek: []int{1, 1}, StartTime: "09:00", EndTime: "12:00"},
		{RoomID: "r1", DaysOfWeek: []int{1}, StartTime: "xx", EndTime: "12:00"},
		{RoomID: "r1", DaysOfWeek: []int{1}, StartTime: "09:00", EndTime: "09:15"},
	}
	for _, sc := range bad {
		if err := ValidateSchedule(sc); err == nil {
			t.Fatalf("expected invalid for %+v", sc)
		}
	}
}

func TestGenerateSlotsForDate(t *testing.T) {
	sc := Schedule{RoomID: "r", DaysOfWeek: []int{1}, StartTime: "09:00", EndTime: "10:00"}
	day := time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC)
	slots, err := GenerateSlotsForDate("r", sc, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(slots))
	}
	if slots[0].Start.Format(time.RFC3339) != "2026-03-23T09:00:00Z" {
		t.Fatalf("unexpected start %s", slots[0].Start.Format(time.RFC3339))
	}
}

package app

import (
	"fmt"
	"time"
)

func ParseHM(v string) (int, int, error) {
	tm, err := time.Parse("15:04", v)
	if err != nil {
		return 0, 0, err
	}
	return tm.Hour(), tm.Minute(), nil
}

func ValidateSchedule(s Schedule) error {
	if s.RoomID == "" || len(s.DaysOfWeek) == 0 || s.StartTime == "" || s.EndTime == "" {
		return ErrInvalidRequest
	}
	seen := map[int]bool{}
	for _, d := range s.DaysOfWeek {
		if d < 1 || d > 7 || seen[d] {
			return ErrInvalidRequest
		}
		seen[d] = true
	}
	sh, sm, err := ParseHM(s.StartTime)
	if err != nil {
		return ErrInvalidRequest
	}
	eh, em, err := ParseHM(s.EndTime)
	if err != nil {
		return ErrInvalidRequest
	}
	start := sh*60 + sm
	end := eh*60 + em
	if end <= start || (end-start)%30 != 0 {
		return ErrInvalidRequest
	}
	return nil
}

func GenerateSlotsForDate(roomID string, schedule Schedule, day time.Time) ([]Slot, error) {
	if err := ValidateSchedule(schedule); err != nil {
		return nil, err
	}
	day = day.UTC()
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	okDay := false
	for _, d := range schedule.DaysOfWeek {
		if d == weekday {
			okDay = true
			break
		}
	}
	if !okDay {
		return []Slot{}, nil
	}

	sh, sm, _ := ParseHM(schedule.StartTime)
	eh, em, _ := ParseHM(schedule.EndTime)
	start := time.Date(day.Year(), day.Month(), day.Day(), sh, sm, 0, 0, time.UTC)
	end := time.Date(day.Year(), day.Month(), day.Day(), eh, em, 0, 0, time.UTC)

	slots := make([]Slot, 0, int(end.Sub(start)/slotDuration))
	for cur := start; cur.Before(end); cur = cur.Add(slotDuration) {
		next := cur.Add(slotDuration)
		if next.After(end) {
			break
		}
		slots = append(slots, Slot{
			ID:     fmt.Sprintf("%d", cur.UnixNano()), // ignored by postgres, used only in tests
			RoomID: roomID,
			Start:  cur,
			End:    next,
		})
	}
	return slots, nil
}

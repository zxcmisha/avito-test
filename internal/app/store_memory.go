package app

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

type MemoryStore struct {
	rooms     []Room
	schedules map[string]Schedule
	slots     map[string]Slot
	bookings  map[string]Booking
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		schedules: map[string]Schedule{},
		slots:     map[string]Slot{},
		bookings:  map[string]Booking{},
	}
}

func (m *MemoryStore) CreateRoom(name string, description *string, capacity *int) (Room, error) {
	now := time.Now().UTC()
	r := Room{ID: uuid.NewString(), Name: name, Description: description, Capacity: capacity, CreatedAt: &now}
	m.rooms = append(m.rooms, r)
	return r, nil
}

func (m *MemoryStore) ListRooms() ([]Room, error) { return slices.Clone(m.rooms), nil }
func (m *MemoryStore) RoomExists(roomID string) (bool, error) {
	for _, r := range m.rooms {
		if r.ID == roomID {
			return true, nil
		}
	}
	return false, nil
}
func (m *MemoryStore) CreateSchedule(s Schedule) (Schedule, error) {
	ok, _ := m.RoomExists(s.RoomID)
	if !ok {
		return Schedule{}, ErrRoomNotFound
	}
	if _, exists := m.schedules[s.RoomID]; exists {
		return Schedule{}, ErrScheduleExists
	}
	s.ID = uuid.NewString()
	m.schedules[s.RoomID] = s
	return s, nil
}
func (m *MemoryStore) GetScheduleByRoom(roomID string) (*Schedule, error) {
	s, ok := m.schedules[roomID]
	if !ok {
		return nil, nil
	}
	return &s, nil
}
func (m *MemoryStore) UpsertSlots(roomID string, slots []Slot) error {
	for _, s := range slots {
		existingID := ""
		for id, cur := range m.slots {
			if cur.RoomID == roomID && cur.Start.Equal(s.Start) && cur.End.Equal(s.End) {
				existingID = id
				break
			}
		}
		if existingID != "" {
			continue
		}
		s.ID = uuid.NewString()
		s.RoomID = roomID
		m.slots[s.ID] = s
	}
	return nil
}
func (m *MemoryStore) ListAvailableSlots(roomID string, date time.Time) ([]Slot, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	out := []Slot{}
	for _, s := range m.slots {
		if s.RoomID != roomID || s.Start.Before(start) || !s.Start.Before(end) {
			continue
		}
		busy := false
		for _, b := range m.bookings {
			if b.SlotID == s.ID && b.Status == BookingActive {
				busy = true
				break
			}
		}
		if !busy {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b Slot) int { return a.Start.Compare(b.Start) })
	return out, nil
}
func (m *MemoryStore) GetSlot(slotID string) (*Slot, error) {
	s, ok := m.slots[slotID]
	if !ok {
		return nil, ErrSlotNotFound
	}
	return &s, nil
}

func (m *MemoryStore) GetBookingByID(bookingID string) (*Booking, error) {
	b, ok := m.bookings[bookingID]
	if !ok {
		return nil, ErrBookingNotFound
	}
	return &b, nil
}
func (m *MemoryStore) CreateBooking(slotID, userID string, withConferenceLink bool) (Booking, error) {
	s, err := m.GetSlot(slotID)
	if err != nil {
		return Booking{}, err
	}
	if s.Start.Before(time.Now().UTC()) {
		return Booking{}, ErrInvalidRequest
	}
	for _, b := range m.bookings {
		if b.SlotID == slotID && b.Status == BookingActive {
			return Booking{}, ErrSlotAlreadyBooked
		}
	}
	now := time.Now().UTC()
	b := Booking{
		ID:        uuid.NewString(),
		SlotID:    slotID,
		UserID:    userID,
		Status:    BookingActive,
		CreatedAt: &now,
	}
	if withConferenceLink {
		v := "https://meet.example.com/" + uuid.NewString()
		b.ConferenceLink = &v
	}
	m.bookings[b.ID] = b
	return b, nil
}
func (m *MemoryStore) ListAllBookings(page, pageSize int) ([]Booking, int, error) {
	all := []Booking{}
	for _, b := range m.bookings {
		all = append(all, b)
	}
	slices.SortFunc(all, func(a, b Booking) int { return b.CreatedAt.Compare(*a.CreatedAt) })
	total := len(all)
	offset := (page - 1) * pageSize
	if offset >= total {
		return []Booking{}, total, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}
func (m *MemoryStore) ListMyFutureBookings(userID string, now time.Time) ([]Booking, error) {
	out := []Booking{}
	for _, b := range m.bookings {
		if b.UserID != userID {
			continue
		}
		s, _ := m.GetSlot(b.SlotID)
		if s.Start.Before(now) {
			continue
		}
		out = append(out, b)
	}
	return out, nil
}
func (m *MemoryStore) CancelBooking(bookingID, userID string) (Booking, error) {
	b, ok := m.bookings[bookingID]
	if !ok {
		return Booking{}, ErrBookingNotFound
	}
	if b.UserID != userID {
		return Booking{}, ErrForbidden
	}
	b.Status = BookingCancelled
	m.bookings[bookingID] = b
	return b, nil
}

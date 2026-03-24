package app

import "time"

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type BookingStatus string

const (
	BookingActive    BookingStatus = "active"
	BookingCancelled BookingStatus = "cancelled"
)

const slotDuration = 30 * time.Minute

type Claims struct {
	UserID string
	Role   Role
}

type User struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	CreatedAt *time.Time `json:"createdAt"`
}

type Room struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	Capacity    *int       `json:"capacity"`
	CreatedAt   *time.Time `json:"createdAt"`
}

type Schedule struct {
	ID         string `json:"id,omitempty"`
	RoomID     string `json:"roomId"`
	DaysOfWeek []int  `json:"daysOfWeek"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
}

type Slot struct {
	ID     string    `json:"id"`
	RoomID string    `json:"roomId"`
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
}

type Booking struct {
	ID             string        `json:"id"`
	SlotID         string        `json:"slotId"`
	UserID         string        `json:"userId"`
	Status         BookingStatus `json:"status"`
	ConferenceLink *string       `json:"conferenceLink"`
	CreatedAt      *time.Time    `json:"createdAt"`
}

type Store interface {
	CreateRoom(name string, description *string, capacity *int) (Room, error)
	ListRooms() ([]Room, error)
	RoomExists(roomID string) (bool, error)

	CreateSchedule(s Schedule) (Schedule, error)
	GetScheduleByRoom(roomID string) (*Schedule, error)

	UpsertSlots(roomID string, slots []Slot) error
	ListAvailableSlots(roomID string, date time.Time) ([]Slot, error)
	GetSlot(slotID string) (*Slot, error)
	GetBookingByID(bookingID string) (*Booking, error)

	CreateBooking(slotID, userID string, withConferenceLink bool) (Booking, error)
	ListAllBookings(page, pageSize int) ([]Booking, int, error)
	ListMyFutureBookings(userID string, now time.Time) ([]Booking, error)
	CancelBooking(bookingID, userID string) (Booking, error)
}

package app

type ErrorBody struct {
	Error struct {
		Code    string `json:"code" example:"INVALID_REQUEST"`
		Message string `json:"message" example:"invalid request"`
	} `json:"error"`
}

type DummyLoginRequest struct {
	Role string `json:"role" enums:"admin,user" example:"user"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type RegisterRequest struct {
	Email    string `json:"email" format:"email"`
	Password string `json:"password"`
	Role     string `json:"role" enums:"admin,user"`
}

type LoginRequest struct {
	Email    string `json:"email" format:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	User User `json:"user"`
}

type RoomsListResponse struct {
	Rooms []Room `json:"rooms"`
}

type CreateRoomRequest struct {
	Name        string  `json:"name" example:"Room A"`
	Description *string `json:"description" example:"Main conference room"`
	Capacity    *int    `json:"capacity" example:"8"`
}

type RoomResponse struct {
	Room Room `json:"room"`
}

type ScheduleResponse struct {
	Schedule Schedule `json:"schedule"`
}

type SlotsListResponse struct {
	Slots []Slot `json:"slots"`
}

type CreateBookingRequest struct {
	SlotID               string `json:"slotId" format:"uuid"`
	CreateConferenceLink bool   `json:"createConferenceLink"`
}

type BookingResponse struct {
	Booking Booking `json:"booking"`
}

type BookingsListResponse struct {
	Bookings   []Booking `json:"bookings"`
	Pagination struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
		Total    int `json:"total"`
	} `json:"pagination"`
}

type MyBookingsResponse struct {
	Bookings []Booking `json:"bookings"`
}

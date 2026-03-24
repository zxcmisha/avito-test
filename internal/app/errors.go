package app

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrRoomNotFound      = errors.New("room not found")
	ErrSlotNotFound      = errors.New("slot not found")
	ErrSlotAlreadyBooked = errors.New("slot already booked")
	ErrBookingNotFound   = errors.New("booking not found")
	ErrForbidden         = errors.New("forbidden")
	ErrScheduleExists    = errors.New("schedule already exists")
	ErrInvalidRequest    = errors.New("invalid request")
)

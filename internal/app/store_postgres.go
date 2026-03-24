package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(10 * time.Minute)
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error { return s.db.Close() }

func (s *PostgresStore) CreateRoom(name string, description *string, capacity *int) (Room, error) {
	var r Room
	err := s.db.QueryRow(`INSERT INTO rooms(name, description, capacity) VALUES ($1,$2,$3)
	RETURNING id::text,name,description,capacity,created_at`, name, description, capacity).
		Scan(&r.ID, &r.Name, &r.Description, &r.Capacity, &r.CreatedAt)
	return r, err
}

func (s *PostgresStore) ListRooms() ([]Room, error) {
	rows, err := s.db.Query(`SELECT id::text,name,description,capacity,created_at FROM rooms ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Room, 0)
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Capacity, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PostgresStore) RoomExists(roomID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM rooms WHERE id=$1::uuid)`, roomID).Scan(&exists)
	return exists, err
}

func (s *PostgresStore) CreateSchedule(sc Schedule) (Schedule, error) {
	var out Schedule
	err := s.db.QueryRow(`INSERT INTO schedules(room_id,days_of_week,start_time,end_time)
	VALUES($1::uuid,$2,$3,$4) RETURNING id::text,room_id::text,days_of_week,start_time,end_time`,
		sc.RoomID, pqIntArray(sc.DaysOfWeek), sc.StartTime, sc.EndTime).
		Scan(&out.ID, &out.RoomID, pqIntArrayScanner(&out.DaysOfWeek), &out.StartTime, &out.EndTime)
	if err != nil {
		if pgErrContains(err, "duplicate key value violates unique constraint") {
			return Schedule{}, ErrScheduleExists
		}
		if pgErrContains(err, "violates foreign key constraint") {
			return Schedule{}, ErrRoomNotFound
		}
		return Schedule{}, err
	}
	return out, nil
}

func (s *PostgresStore) GetScheduleByRoom(roomID string) (*Schedule, error) {
	var out Schedule
	err := s.db.QueryRow(`SELECT id::text,room_id::text,days_of_week,start_time,end_time FROM schedules WHERE room_id=$1::uuid`, roomID).
		Scan(&out.ID, &out.RoomID, pqIntArrayScanner(&out.DaysOfWeek), &out.StartTime, &out.EndTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func (s *PostgresStore) UpsertSlots(roomID string, slots []Slot) error {
	for _, sl := range slots {
		_, err := s.db.Exec(`INSERT INTO slots(room_id,start_at,end_at)
VALUES($1::uuid,$2,$3) ON CONFLICT (room_id,start_at,end_at) DO NOTHING`, roomID, sl.Start.UTC(), sl.End.UTC())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) ListAvailableSlots(roomID string, date time.Time) ([]Slot, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	rows, err := s.db.Query(`SELECT s.id::text,s.room_id::text,s.start_at,s.end_at
FROM slots s
LEFT JOIN bookings b ON b.slot_id=s.id AND b.status='active'
WHERE s.room_id=$1::uuid AND s.start_at >= $2 AND s.start_at < $3 AND b.id IS NULL
ORDER BY s.start_at ASC`, roomID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Slot, 0)
	for rows.Next() {
		var sl Slot
		if err := rows.Scan(&sl.ID, &sl.RoomID, &sl.Start, &sl.End); err != nil {
			return nil, err
		}
		out = append(out, sl)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetSlot(slotID string) (*Slot, error) {
	var sl Slot
	err := s.db.QueryRow(`SELECT id::text,room_id::text,start_at,end_at FROM slots WHERE id=$1::uuid`, slotID).
		Scan(&sl.ID, &sl.RoomID, &sl.Start, &sl.End)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSlotNotFound
		}
		return nil, err
	}
	return &sl, nil
}

func (s *PostgresStore) GetBookingByID(bookingID string) (*Booking, error) {
	var b Booking
	err := s.db.QueryRow(`SELECT id::text,slot_id::text,user_id::text,status,conference_link,created_at
FROM bookings WHERE id=$1::uuid`, bookingID).
		Scan(&b.ID, &b.SlotID, &b.UserID, &b.Status, &b.ConferenceLink, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (s *PostgresStore) CreateBooking(slotID, userID string, withConferenceLink bool) (Booking, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback()

	var slotStart time.Time
	if err := tx.QueryRowContext(ctx, `SELECT start_at FROM slots WHERE id=$1::uuid`, slotID).Scan(&slotStart); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Booking{}, ErrSlotNotFound
		}
		return Booking{}, err
	}
	if slotStart.Before(time.Now().UTC()) {
		return Booking{}, ErrInvalidRequest
	}

	link := (*string)(nil)
	if withConferenceLink {
		v := fmt.Sprintf("https://meet.example.com/%s", uuid.NewString())
		link = &v
	}

	var out Booking
	err = tx.QueryRowContext(ctx, `INSERT INTO bookings(slot_id,user_id,status,conference_link)
VALUES($1::uuid,$2::uuid,'active',$3)
RETURNING id::text,slot_id::text,user_id::text,status,conference_link,created_at`,
		slotID, userID, link).
		Scan(&out.ID, &out.SlotID, &out.UserID, &out.Status, &out.ConferenceLink, &out.CreatedAt)
	if err != nil {
		if pgErrContains(err, "idx_bookings_active_per_slot") || pgErrContains(err, "duplicate key value") {
			return Booking{}, ErrSlotAlreadyBooked
		}
		return Booking{}, err
	}

	if err := tx.Commit(); err != nil {
		if pgErrContains(err, "duplicate key value") {
			return Booking{}, ErrSlotAlreadyBooked
		}
		return Booking{}, err
	}
	return out, nil
}

func (s *PostgresStore) ListAllBookings(page, pageSize int) ([]Booking, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT count(*) FROM bookings`).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	rows, err := s.db.Query(`SELECT id::text,slot_id::text,user_id::text,status,conference_link,created_at
FROM bookings ORDER BY created_at DESC LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Booking, 0)
	for rows.Next() {
		var b Booking
		if err := rows.Scan(&b.ID, &b.SlotID, &b.UserID, &b.Status, &b.ConferenceLink, &b.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

func (s *PostgresStore) ListMyFutureBookings(userID string, now time.Time) ([]Booking, error) {
	rows, err := s.db.Query(`SELECT b.id::text,b.slot_id::text,b.user_id::text,b.status,b.conference_link,b.created_at
FROM bookings b JOIN slots s ON s.id=b.slot_id
WHERE b.user_id=$1::uuid AND s.start_at >= $2 ORDER BY s.start_at ASC`, userID, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Booking, 0)
	for rows.Next() {
		var b Booking
		if err := rows.Scan(&b.ID, &b.SlotID, &b.UserID, &b.Status, &b.ConferenceLink, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CancelBooking(bookingID, userID string) (Booking, error) {
	var owner string
	err := s.db.QueryRow(`SELECT user_id::text FROM bookings WHERE id=$1::uuid`, bookingID).Scan(&owner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Booking{}, ErrBookingNotFound
		}
		return Booking{}, err
	}
	if owner != userID {
		return Booking{}, ErrForbidden
	}

	_, err = s.db.Exec(`UPDATE bookings SET status='cancelled' WHERE id=$1::uuid AND status='active'`, bookingID)
	if err != nil {
		return Booking{}, err
	}
	var b Booking
	err = s.db.QueryRow(`SELECT id::text,slot_id::text,user_id::text,status,conference_link,created_at
FROM bookings WHERE id=$1::uuid`, bookingID).
		Scan(&b.ID, &b.SlotID, &b.UserID, &b.Status, &b.ConferenceLink, &b.CreatedAt)
	if err != nil {
		return Booking{}, err
	}
	return b, nil
}

func pgErrContains(err error, sub string) bool {
	return err != nil && strings.Contains(err.Error(), sub)
}

type intArray []int

func pqIntArray(v []int) intArray { return intArray(v) }
func (a intArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	out := "{"
	for i, n := range a {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("%d", n)
	}
	out += "}"
	return out, nil
}

type intArrayScanner struct{ target *[]int }

func pqIntArrayScanner(target *[]int) intArrayScanner { return intArrayScanner{target: target} }
func (s intArrayScanner) Scan(src any) error {
	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("unexpected type: %T", src)
	}
	text := string(b)
	if text == "{}" {
		*s.target = []int{}
		return nil
	}
	text = text[1 : len(text)-1]
	parts := strings.Split(text, ",")
	res := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		res = append(res, n)
	}
	*s.target = res
	return nil
}

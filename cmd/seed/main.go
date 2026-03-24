package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/booking?sslmode=disable")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if err := seedRooms(db); err != nil {
		log.Fatalf("seed rooms: %v", err)
	}
	if err := seedSchedules(db); err != nil {
		log.Fatalf("seed schedules: %v", err)
	}
	if err := seedSlots(db); err != nil {
		log.Fatalf("seed slots: %v", err)
	}
	if err := seedBooking(db); err != nil {
		log.Fatalf("seed bookings: %v", err)
	}

	log.Println("seed completed")
}

func seedRooms(db *sql.DB) error {
	_, err := db.Exec(`
INSERT INTO rooms (id, name, description, capacity)
VALUES
  ('00000000-0000-0000-0000-000000000101', 'Room A', 'Main conference room', 8),
  ('00000000-0000-0000-0000-000000000102', 'Room B', 'Daily standup room', 6),
  ('00000000-0000-0000-0000-000000000103', 'Room C', 'Interview room', 4)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name, description = EXCLUDED.description, capacity = EXCLUDED.capacity;
`)
	return err
}

func seedSchedules(db *sql.DB) error {
	_, err := db.Exec(`
INSERT INTO schedules (id, room_id, days_of_week, start_time, end_time)
VALUES
  ('00000000-0000-0000-0000-000000000201', '00000000-0000-0000-0000-000000000101', '{1,2,3,4,5}', '09:00', '18:00'),
  ('00000000-0000-0000-0000-000000000202', '00000000-0000-0000-0000-000000000102', '{1,2,3,4,5}', '10:00', '17:00'),
  ('00000000-0000-0000-0000-000000000203', '00000000-0000-0000-0000-000000000103', '{1,3,5}', '11:00', '16:00')
ON CONFLICT (room_id) DO UPDATE
SET days_of_week = EXCLUDED.days_of_week, start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time;
`)
	return err
}

func seedSlots(db *sql.DB) error {
	rooms := []struct {
		id        string
		startHour int
		endHour   int
	}{
		{id: "00000000-0000-0000-0000-000000000101", startHour: 9, endHour: 18},
		{id: "00000000-0000-0000-0000-000000000102", startHour: 10, endHour: 17},
		{id: "00000000-0000-0000-0000-000000000103", startHour: 11, endHour: 16},
	}

	now := time.Now().UTC()
	base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	for d := 0; d < 7; d++ {
		day := base.AddDate(0, 0, d)
		weekday := day.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			continue
		}
		for _, room := range rooms {
			for h := room.startHour; h < room.endHour; h++ {
				for _, minute := range []int{0, 30} {
					start := time.Date(day.Year(), day.Month(), day.Day(), h, minute, 0, 0, time.UTC)
					end := start.Add(30 * time.Minute)
					if _, err := db.Exec(`
INSERT INTO slots (room_id, start_at, end_at)
VALUES ($1::uuid, $2, $3)
ON CONFLICT (room_id, start_at, end_at) DO NOTHING
`, room.id, start, end); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func seedBooking(db *sql.DB) error {
	const userID = "22222222-2222-2222-2222-222222222222"

	var slotID string
	err := db.QueryRow(`
SELECT id::text
FROM slots
WHERE room_id = '00000000-0000-0000-0000-000000000101'::uuid
  AND start_at > NOW()
ORDER BY start_at
LIMIT 1
`).Scan(&slotID)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
INSERT INTO bookings (slot_id, user_id, status, conference_link)
VALUES ($1::uuid, $2::uuid, 'active', 'https://meet.example.com/demo-seed')
ON CONFLICT DO NOTHING
`, slotID, userID)
	return err
}

func getenv(k, fallback string) string {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	return v
}

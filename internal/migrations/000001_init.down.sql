DROP INDEX IF EXISTS idx_bookings_user;
DROP INDEX IF EXISTS idx_bookings_active_per_slot;
DROP TABLE IF EXISTS bookings;

DROP INDEX IF EXISTS idx_slots_room_date;
DROP TABLE IF EXISTS slots;

DROP TABLE IF EXISTS schedules;
DROP TABLE IF EXISTS rooms;

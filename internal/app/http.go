package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

const (
	adminID = "11111111-1111-1111-1111-111111111111"
	userID  = "22222222-2222-2222-2222-222222222222"
)

type Server struct {
	store Store
	jwt   *JWTManager
	cache SlotsCache
	nowFn func() time.Time

	usersMu sync.Mutex
	users   map[string]registeredUser
}

func NewServer(store Store, jwt *JWTManager, cache SlotsCache) *Server {
	if cache == nil {
		cache = NoopSlotsCache{}
	}
	return &Server{
		store: store,
		jwt:   jwt,
		cache: cache,
		nowFn: func() time.Time { return time.Now().UTC() },
		users: map[string]registeredUser{},
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.HandleFunc("/_info", s.handleInfo)
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	r.Post("/register", s.handleRegister)
	r.Post("/login", s.handleLogin)
	r.Post("/dummyLogin", s.handleDummyLogin)

	r.Group(func(pr chi.Router) {
		pr.Use(s.authMiddleware)
		pr.Get("/rooms/list", s.handleListRooms)
		pr.Post("/rooms/create", s.requireRole(RoleAdmin, s.handleCreateRoom))
		pr.Post("/rooms/{roomId}/schedule/create", s.requireRole(RoleAdmin, s.handleCreateSchedule))
		pr.Get("/rooms/{roomId}/slots/list", s.handleListSlots)

		pr.Post("/bookings/create", s.requireRole(RoleUser, s.handleCreateBooking))
		pr.Get("/bookings/list", s.requireRole(RoleAdmin, s.handleListBookings))
		pr.Get("/bookings/my", s.requireRole(RoleUser, s.handleMyBookings))
		pr.Post("/bookings/{bookingId}/cancel", s.requireRole(RoleUser, s.handleCancelBooking))
	})
	return r
}

func (s *Server) handleInfo(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type registeredUser struct {
	User
	Password string
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Email == "" || req.Password == "" || (req.Role != string(RoleAdmin) && req.Role != string(RoleUser)) {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}

	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	if _, exists := s.users[strings.ToLower(req.Email)]; exists {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "email already exists")
		return
	}

	now := s.nowFn()
	u := User{
		ID:        newUUID(),
		Email:     req.Email,
		Role:      req.Role,
		CreatedAt: &now,
	}
	s.users[strings.ToLower(req.Email)] = registeredUser{
		User:     u,
		Password: req.Password,
	}
	writeJSON(w, http.StatusCreated, UserResponse{User: u})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	s.usersMu.Lock()
	user, exists := s.users[strings.ToLower(req.Email)]
	s.usersMu.Unlock()
	if !exists || user.Password != req.Password {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
		return
	}

	token, err := s.jwt.IssueToken(user.ID, Role(user.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, TokenResponse{Token: token})
}

func (s *Server) handleDummyLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role string `json:"role"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	var uid string
	switch req.Role {
	case string(RoleAdmin):
		uid = adminID
	case string(RoleUser):
		uid = userID
	default:
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid role")
		return
	}
	token, err := s.jwt.IssueToken(uid, Role(req.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

func (s *Server) handleListRooms(w http.ResponseWriter, _ *http.Request) {
	rooms, err := s.store.ListRooms()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
		Capacity    *int    `json:"capacity"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "name is required")
		return
	}
	room, err := s.store.CreateRoom(req.Name, req.Description, req.Capacity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"room": room})
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	var req Schedule
	if !decodeJSON(w, r, &req) {
		return
	}
	req.RoomID = roomID
	if err := ValidateSchedule(req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid schedule")
		return
	}
	sc, err := s.store.CreateSchedule(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrRoomNotFound):
			writeError(w, http.StatusNotFound, "ROOM_NOT_FOUND", "room not found")
		case errors.Is(err, ErrScheduleExists):
			writeError(w, http.StatusConflict, "SCHEDULE_EXISTS", "schedule for this room already exists and cannot be changed")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}

	now := s.nowFn()
	for i := 0; i < 7; i++ {
		day := now.AddDate(0, 0, i)
		slots, _ := GenerateSlotsForDate(roomID, sc, day)
		_ = s.store.UpsertSlots(roomID, slots)
	}
	_ = s.cache.InvalidateRoom(r.Context(), roomID)
	writeJSON(w, http.StatusCreated, map[string]any{"schedule": sc})
}

func (s *Server) handleListSlots(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	dateRaw := r.URL.Query().Get("date")
	if dateRaw == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "date is required")
		return
	}
	date, err := time.Parse("2006-01-02", dateRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid date")
		return
	}
	exists, err := s.store.RoomExists(roomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "ROOM_NOT_FOUND", "room not found")
		return
	}
	if cachedSlots, found, err := s.cache.GetSlots(r.Context(), roomID, dateRaw); err == nil && found {
		writeJSON(w, http.StatusOK, map[string]any{"slots": cachedSlots})
		return
	}
	schedule, err := s.store.GetScheduleByRoom(roomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	if schedule == nil {
		_ = s.cache.SetSlots(r.Context(), roomID, dateRaw, []Slot{}, 30*time.Second)
		writeJSON(w, http.StatusOK, map[string]any{"slots": []Slot{}})
		return
	}

	generated, err := GenerateSlotsForDate(roomID, *schedule, date)
	if err == nil {
		_ = s.store.UpsertSlots(roomID, generated)
	}
	slots, err := s.store.ListAvailableSlots(roomID, date)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	_ = s.cache.SetSlots(r.Context(), roomID, dateRaw, slots, 30*time.Second)
	writeJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

func (s *Server) handleCreateBooking(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	var req struct {
		SlotID               string `json:"slotId"`
		CreateConferenceLink bool   `json:"createConferenceLink"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.SlotID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "slotId is required")
		return
	}
	slot, err := s.store.GetSlot(req.SlotID)
	if err != nil {
		if errors.Is(err, ErrSlotNotFound) {
			writeError(w, http.StatusNotFound, "SLOT_NOT_FOUND", "slot not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	b, err := s.store.CreateBooking(req.SlotID, claims.UserID, req.CreateConferenceLink)
	if err != nil {
		switch {
		case errors.Is(err, ErrSlotNotFound):
			writeError(w, http.StatusNotFound, "SLOT_NOT_FOUND", "slot not found")
		case errors.Is(err, ErrSlotAlreadyBooked):
			writeError(w, http.StatusConflict, "SLOT_ALREADY_BOOKED", "slot is already booked")
		case errors.Is(err, ErrInvalidRequest):
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "slot is in the past")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	_ = s.cache.InvalidateRoom(r.Context(), slot.RoomID)
	writeJSON(w, http.StatusCreated, map[string]any{"booking": b})
}

func (s *Server) handleListBookings(w http.ResponseWriter, r *http.Request) {
	page := 1
	pageSize := 20
	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid pagination")
			return
		}
		page = n
	}
	if v := r.URL.Query().Get("pageSize"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid pagination")
			return
		}
		pageSize = n
	}
	bookings, total, err := s.store.ListAllBookings(page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bookings": bookings,
		"pagination": map[string]any{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (s *Server) handleMyBookings(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	bookings, err := s.store.ListMyFutureBookings(claims.UserID, s.nowFn())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bookings": bookings})
}

func (s *Server) handleCancelBooking(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	bookingID := chi.URLParam(r, "bookingId")
	var roomID string
	if b0, err := s.store.GetBookingByID(bookingID); err == nil {
		if sl, err := s.store.GetSlot(b0.SlotID); err == nil {
			roomID = sl.RoomID
		}
	}
	booking, err := s.store.CancelBooking(bookingID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrBookingNotFound):
			writeError(w, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found")
		case errors.Is(err, ErrForbidden):
			writeError(w, http.StatusForbidden, "FORBIDDEN", "cannot cancel another user's booking")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	if roomID != "" {
		_ = s.cache.InvalidateRoom(r.Context(), roomID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"booking": booking})
}

type ctxKey string

const claimsKey ctxKey = "claims"

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing token")
			return
		}
		claims, err := s.jwt.ParseToken(strings.TrimPrefix(auth, "Bearer "))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}

func (s *Server) requireRole(role Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFromCtx(r)
		if claims.Role != role {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}
		next(w, r)
	}
}

func claimsFromCtx(r *http.Request) Claims {
	v, _ := r.Context().Value(claimsKey).(Claims)
	return v
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func newUUID() string {
	return uuid.NewString()
}

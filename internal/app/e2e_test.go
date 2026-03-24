package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestE2E_CreateRoomScheduleBooking(t *testing.T) {
	store := NewMemoryStore()
	srv := NewServer(store, NewJWTManager("test-secret"), nil)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	adminToken := getToken(t, ts.URL, "admin")
	userToken := getToken(t, ts.URL, "user")

	roomResp := doJSON(t, ts.URL, "POST", "/rooms/create", adminToken, map[string]any{"name": "A-101"}, http.StatusCreated)
	roomID := roomResp["room"].(map[string]any)["id"].(string)

	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	dow := int(tomorrow.Weekday())
	if dow == 0 {
		dow = 7
	}
	doJSON(t, ts.URL, "POST", "/rooms/"+roomID+"/schedule/create", adminToken, map[string]any{
		"daysOfWeek": []int{dow},
		"startTime":  "09:00",
		"endTime":    "10:00",
	}, http.StatusCreated)

	slotsResp := doJSON(t, ts.URL, "GET", "/rooms/"+roomID+"/slots/list?date="+tomorrow.Format("2006-01-02"), userToken, nil, http.StatusOK)
	slots := slotsResp["slots"].([]any)
	if len(slots) == 0 {
		t.Fatalf("expected slots")
	}
	slotID := slots[0].(map[string]any)["id"].(string)

	booking := doJSON(t, ts.URL, "POST", "/bookings/create", userToken, map[string]any{"slotId": slotID}, http.StatusCreated)
	if booking["booking"].(map[string]any)["status"].(string) != "active" {
		t.Fatalf("expected active booking")
	}
}

func TestE2E_CancelBookingIdempotent(t *testing.T) {
	store := NewMemoryStore()
	srv := NewServer(store, NewJWTManager("test-secret"), nil)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	adminToken := getToken(t, ts.URL, "admin")
	userToken := getToken(t, ts.URL, "user")

	roomResp := doJSON(t, ts.URL, "POST", "/rooms/create", adminToken, map[string]any{"name": "A-102"}, http.StatusCreated)
	roomID := roomResp["room"].(map[string]any)["id"].(string)
	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	dow := int(tomorrow.Weekday())
	if dow == 0 {
		dow = 7
	}
	doJSON(t, ts.URL, "POST", "/rooms/"+roomID+"/schedule/create", adminToken, map[string]any{
		"daysOfWeek": []int{dow},
		"startTime":  "09:00",
		"endTime":    "10:00",
	}, http.StatusCreated)
	slotsResp := doJSON(t, ts.URL, "GET", "/rooms/"+roomID+"/slots/list?date="+tomorrow.Format("2006-01-02"), userToken, nil, http.StatusOK)
	slotID := slotsResp["slots"].([]any)[0].(map[string]any)["id"].(string)
	booking := doJSON(t, ts.URL, "POST", "/bookings/create", userToken, map[string]any{"slotId": slotID}, http.StatusCreated)
	bookingID := booking["booking"].(map[string]any)["id"].(string)

	cancel1 := doJSON(t, ts.URL, "POST", "/bookings/"+bookingID+"/cancel", userToken, nil, http.StatusOK)
	if cancel1["booking"].(map[string]any)["status"].(string) != "cancelled" {
		t.Fatalf("expected cancelled")
	}
	cancel2 := doJSON(t, ts.URL, "POST", "/bookings/"+bookingID+"/cancel", userToken, nil, http.StatusOK)
	if cancel2["booking"].(map[string]any)["status"].(string) != "cancelled" {
		t.Fatalf("expected cancelled on second call")
	}
}

func TestE2E_BookingsListAndMy(t *testing.T) {
	store := NewMemoryStore()
	srv := NewServer(store, NewJWTManager("test-secret"), nil)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	adminToken := getToken(t, ts.URL, "admin")
	userToken := getToken(t, ts.URL, "user")

	roomResp := doJSON(t, ts.URL, "POST", "/rooms/create", adminToken, map[string]any{"name": "A-103"}, http.StatusCreated)
	roomID := roomResp["room"].(map[string]any)["id"].(string)
	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	dow := int(tomorrow.Weekday())
	if dow == 0 {
		dow = 7
	}
	doJSON(t, ts.URL, "POST", "/rooms/"+roomID+"/schedule/create", adminToken, map[string]any{
		"daysOfWeek": []int{dow},
		"startTime":  "09:00",
		"endTime":    "10:00",
	}, http.StatusCreated)
	slotsResp := doJSON(t, ts.URL, "GET", "/rooms/"+roomID+"/slots/list?date="+tomorrow.Format("2006-01-02"), userToken, nil, http.StatusOK)
	slotID := slotsResp["slots"].([]any)[0].(map[string]any)["id"].(string)
	doJSON(t, ts.URL, "POST", "/bookings/create", userToken, map[string]any{"slotId": slotID}, http.StatusCreated)

	rooms := doJSON(t, ts.URL, "GET", "/rooms/list", userToken, nil, http.StatusOK)
	if len(rooms["rooms"].([]any)) != 1 {
		t.Fatalf("expected rooms list")
	}
	allBookings := doJSON(t, ts.URL, "GET", "/bookings/list?page=1&pageSize=20", adminToken, nil, http.StatusOK)
	if len(allBookings["bookings"].([]any)) != 1 {
		t.Fatalf("expected one booking in admin list")
	}
	myBookings := doJSON(t, ts.URL, "GET", "/bookings/my", userToken, nil, http.StatusOK)
	if len(myBookings["bookings"].([]any)) != 1 {
		t.Fatalf("expected one booking in my list")
	}
}

func getToken(t *testing.T, baseURL, role string) string {
	t.Helper()
	resp := doJSON(t, baseURL, "POST", "/dummyLogin", "", map[string]any{"role": role}, http.StatusOK)
	return resp["token"].(string)
}

func doJSON(t *testing.T, baseURL, method, path, token string, body any, expected int) map[string]any {
	t.Helper()
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, _ := http.NewRequest(method, baseURL+path, &b)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != expected {
		t.Fatalf("%s %s: expected %d got %d", method, path, expected, res.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err == nil {
		return out
	}
	return map[string]any{}
}

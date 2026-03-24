package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func BenchmarkSlotsListParallel(b *testing.B) {
	store := NewMemoryStore()
	srv := NewServer(store, NewJWTManager("bench-secret"), nil)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	adminToken := benchGetToken(b, ts.URL, "admin")
	userToken := benchGetToken(b, ts.URL, "user")

	roomID := benchCreateRoomAndSchedule(b, ts.URL, adminToken)
	date := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	url := ts.URL + "/rooms/" + roomID + "/slots/list?date=" + date

	transport := &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
		MaxConnsPerHost:     512,
		IdleConnTimeout:     30 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}
	reqTemplate, _ := http.NewRequest(http.MethodGet, url, nil)
	reqTemplate.Header.Set("Authorization", "Bearer "+userToken)

	b.ReportAllocs()
	b.SetParallelism(4)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := reqTemplate.Clone(reqTemplate.Context())
			res, err := client.Do(req)
			if err != nil {
				b.Fatalf("request failed: %v", err)
			}
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
			if res.StatusCode != http.StatusOK {
				b.Fatalf("unexpected status: %d", res.StatusCode)
			}
		}
	})
}

func benchCreateRoomAndSchedule(b *testing.B, baseURL, adminToken string) string {
	b.Helper()
	room := benchDoJSON(b, baseURL, http.MethodPost, "/rooms/create", adminToken, map[string]any{
		"name": "Bench room",
	}, http.StatusCreated)
	roomID := room["room"].(map[string]any)["id"].(string)

	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	dow := int(tomorrow.Weekday())
	if dow == 0 {
		dow = 7
	}
	benchDoJSON(b, baseURL, http.MethodPost, "/rooms/"+roomID+"/schedule/create", adminToken, map[string]any{
		"daysOfWeek": []int{dow},
		"startTime":  "09:00",
		"endTime":    "18:00",
	}, http.StatusCreated)

	return roomID
}

var benchTokenMu sync.Mutex

func benchGetToken(b *testing.B, baseURL, role string) string {
	b.Helper()
	benchTokenMu.Lock()
	defer benchTokenMu.Unlock()
	resp := benchDoJSON(b, baseURL, http.MethodPost, "/dummyLogin", "", map[string]any{"role": role}, http.StatusOK)
	return resp["token"].(string)
}

func benchDoJSON(b *testing.B, baseURL, method, path, token string, body any, expected int) map[string]any {
	b.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			b.Fatal(err)
		}
	}

	req, err := http.NewRequest(method, baseURL+path, &buf)
	if err != nil {
		b.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		b.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != expected {
		b.Fatalf("%s %s expected %d got %d", method, path, expected, res.StatusCode)
	}

	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out
}

package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSignupLoginCreateSurveyAndRespondFlow(t *testing.T) {
	server, err := NewServer(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	client := &http.Client{}
	signupResp := postJSON(t, client, ts.URL+"/api/auth/signup", map[string]string{
		"email": "matt@example.com", "password": "correct horse battery staple", "name": "Matt",
	}, nil)
	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("signup status = %d", signupResp.StatusCode)
	}
	sessionCookie := firstCookie(signupResp, "scheduling_session")
	if sessionCookie == nil {
		t.Fatal("signup did not set session cookie")
	}

	slots := []TimeSlot{{Start: time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)}, {Start: time.Date(2026, 7, 12, 16, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 12, 17, 0, 0, 0, time.UTC)}}
	surveyResp := postJSON(t, client, ts.URL+"/api/surveys", map[string]any{
		"title": "Band practice", "description": "Pick slots", "timezone": "America/New_York", "slots": slots,
	}, sessionCookie)
	if surveyResp.StatusCode != http.StatusCreated {
		t.Fatalf("create survey status = %d", surveyResp.StatusCode)
	}
	var created Survey
	decodeJSON(t, surveyResp, &created)
	if created.ShareToken == "" || len(created.Slots) != 2 {
		t.Fatalf("created survey missing token/slots: %+v", created)
	}

	publicResp, err := client.Get(ts.URL + "/api/public/surveys/" + created.ShareToken)
	if err != nil {
		t.Fatal(err)
	}
	if publicResp.StatusCode != http.StatusOK {
		t.Fatalf("public survey status = %d", publicResp.StatusCode)
	}

	responseResp := postJSON(t, client, ts.URL+"/api/public/surveys/"+created.ShareToken+"/responses", map[string]any{
		"respondent_name": "Dana", "slot_ids": []int64{created.Slots[0].ID},
	}, nil)
	if responseResp.StatusCode != http.StatusCreated {
		t.Fatalf("response status = %d", responseResp.StatusCode)
	}

	resultsReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/surveys/"+created.ShareToken+"/results", nil)
	resultsReq.AddCookie(sessionCookie)
	resultsResp, err := client.Do(resultsReq)
	if err != nil {
		t.Fatal(err)
	}
	if resultsResp.StatusCode != http.StatusOK {
		t.Fatalf("results status = %d", resultsResp.StatusCode)
	}
	var results SurveyResults
	decodeJSON(t, resultsResp, &results)
	if results.SlotCounts[created.Slots[0].ID] != 1 || results.SlotCounts[created.Slots[1].ID] != 0 {
		t.Fatalf("unexpected counts: %+v", results.SlotCounts)
	}
}

func TestDuplicateSlotSelectionsAreRejected(t *testing.T) {
	server, err := NewServer(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()
	client := &http.Client{}

	signupResp := postJSON(t, client, ts.URL+"/api/auth/signup", map[string]string{"email": "dupe@example.com", "password": "good-password", "name": "Dupe"}, nil)
	sessionCookie := firstCookie(signupResp, "scheduling_session")
	slots := []TimeSlot{{Start: time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)}}
	surveyResp := postJSON(t, client, ts.URL+"/api/surveys", map[string]any{"title": "Dupe survey", "timezone": "UTC", "slots": slots}, sessionCookie)
	var created Survey
	decodeJSON(t, surveyResp, &created)

	resp := postJSON(t, client, ts.URL+"/api/public/surveys/"+created.ShareToken+"/responses", map[string]any{
		"respondent_name": "Dana", "slot_ids": []int64{created.Slots[0].ID, created.Slots[0].ID},
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("duplicate response status = %d", resp.StatusCode)
	}
}

func TestHealthOnlyAllowsGet(t *testing.T) {
	server, err := NewServer(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	getRecorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("GET health status = %d", getRecorder.Code)
	}
	assert.Equal(t, "no-store", getRecorder.Header().Get("Cache-Control"))
	var health map[string]string
	if err := json.NewDecoder(getRecorder.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "ok", health["status"])

	postRecorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(postRecorder, httptest.NewRequest(http.MethodPost, "/api/health", nil))
	if postRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST health status = %d", postRecorder.Code)
	}
	if allow := postRecorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("POST health Allow = %q", allow)
	}
}

func TestCORSOnlyAllowsConfiguredOrigins(t *testing.T) {
	server, err := NewServer(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected CORS allow origin for denied site: %q", got)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected localhost dev origin to be allowed, got %q", got)
	}
}

func TestDuplicateCandidateSlotsAreRejected(t *testing.T) {
	server, err := NewServer(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()
	client := &http.Client{}

	signupResp := postJSON(t, client, ts.URL+"/api/auth/signup", map[string]string{"email": "candidate@example.com", "password": "good-password", "name": "Candidate"}, nil)
	sessionCookie := firstCookie(signupResp, "scheduling_session")
	slot := TimeSlot{Start: time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)}
	resp := postJSON(t, client, ts.URL+"/api/surveys", map[string]any{"title": "Duplicate candidates", "timezone": "UTC", "slots": []TimeSlot{slot, slot}}, sessionCookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("duplicate candidate status = %d", resp.StatusCode)
	}
}

func TestPublicSurveyDoesNotExposeCreatorID(t *testing.T) {
	server, err := NewServer(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()
	client := &http.Client{}

	signupResp := postJSON(t, client, ts.URL+"/api/auth/signup", map[string]string{"email": "public@example.com", "password": "good-password", "name": "Public"}, nil)
	sessionCookie := firstCookie(signupResp, "scheduling_session")
	slots := []TimeSlot{{Start: time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)}}
	surveyResp := postJSON(t, client, ts.URL+"/api/surveys", map[string]any{"title": "Public survey", "timezone": "UTC", "slots": slots}, sessionCookie)
	var created Survey
	decodeJSON(t, surveyResp, &created)

	resp, err := client.Get(ts.URL + "/api/public/surveys/" + created.ShareToken)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["created_by"]; ok {
		t.Fatalf("public survey leaked created_by: %+v", payload)
	}
}

func TestTrailingJSONIsRejected(t *testing.T) {
	server, err := NewServer(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/auth/signup", bytes.NewBufferString(`{"email":"trail@example.com","password":"good-password","name":"Trail"}{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("trailing json status = %d", resp.StatusCode)
	}
}

func TestLoginRejectsBadPassword(t *testing.T) {
	server, err := NewServer(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()
	client := &http.Client{}
	postJSON(t, client, ts.URL+"/api/auth/signup", map[string]string{"email": "a@example.com", "password": "good-password", "name": "A"}, nil)
	resp := postJSON(t, client, ts.URL+"/api/auth/login", map[string]string{"email": "a@example.com", "password": "bad-password"}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d", resp.StatusCode)
	}
}

func postJSON(t *testing.T, client *http.Client, url string, body any, cookie *http.Cookie) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func firstCookie(resp *http.Response, name string) *http.Cookie {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

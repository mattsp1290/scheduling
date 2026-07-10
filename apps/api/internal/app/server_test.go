package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

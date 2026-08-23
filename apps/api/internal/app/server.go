package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const sessionCookieName = "scheduling_session"

type Server struct {
	db             *sql.DB
	allowedOrigins map[string]bool
	secureCookies  bool
}

type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type TimeSlot struct {
	ID     int64     `json:"id"`
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Survey int64     `json:"-"`
}

type Survey struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Timezone    string     `json:"timezone"`
	ShareToken  string     `json:"share_token"`
	CreatedBy   int64      `json:"created_by,omitempty"`
	Slots       []TimeSlot `json:"slots"`
}

type PublicSurvey struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Timezone    string     `json:"timezone"`
	ShareToken  string     `json:"share_token"`
	Slots       []TimeSlot `json:"slots"`
}

type Response struct {
	ID             int64   `json:"id"`
	RespondentName string  `json:"respondent_name"`
	SlotIDs        []int64 `json:"slot_ids"`
}

type SurveyResults struct {
	Survey      Survey             `json:"survey"`
	Responses   []Response         `json:"responses"`
	SlotCounts  map[int64]int      `json:"slot_counts"`
	Respondents map[int64][]string `json:"respondents"`
}

func NewServer(dbPath string) (*Server, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Server{db: db, allowedOrigins: loadAllowedOrigins(), secureCookies: os.Getenv("COOKIE_SECURE") == "true"}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Server) Close() error { return s.db.Close() }

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.requireMethod(http.MethodGet, s.handleHealth))
	mux.HandleFunc("/api/auth/signup", s.requireMethod(http.MethodPost, s.handleSignup))
	mux.HandleFunc("/api/auth/login", s.requireMethod(http.MethodPost, s.handleLogin))
	mux.HandleFunc("/api/auth/logout", s.requireMethod(http.MethodPost, s.handleLogout))
	mux.HandleFunc("/api/me", s.requireMethod(http.MethodGet, s.handleMe))
	mux.HandleFunc("/api/surveys", s.handleSurveys)
	mux.HandleFunc("/api/surveys/", s.handleSurveyDetail)
	mux.HandleFunc("/api/public/surveys/", s.handlePublicSurvey)
	return s.withCORS(mux)
}

func (s *Server) migrate() error {
	_, err := s.db.Exec(`
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT NOT NULL UNIQUE, name TEXT NOT NULL, password_hash TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, expires_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS surveys (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, description TEXT NOT NULL, timezone TEXT NOT NULL, share_token TEXT NOT NULL UNIQUE, created_by INTEGER NOT NULL REFERENCES users(id), created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS survey_slots (id INTEGER PRIMARY KEY AUTOINCREMENT, survey_id INTEGER NOT NULL REFERENCES surveys(id) ON DELETE CASCADE, starts_at TEXT NOT NULL, ends_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS responses (id INTEGER PRIMARY KEY AUTOINCREMENT, survey_id INTEGER NOT NULL REFERENCES surveys(id) ON DELETE CASCADE, respondent_name TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS response_slots (response_id INTEGER NOT NULL REFERENCES responses(id) ON DELETE CASCADE, slot_id INTEGER NOT NULL REFERENCES survey_slots(id) ON DELETE CASCADE, PRIMARY KEY(response_id, slot_id));
CREATE INDEX IF NOT EXISTS idx_surveys_created_by ON surveys(created_by);
CREATE INDEX IF NOT EXISTS idx_survey_slots_survey_id ON survey_slots(survey_id);
CREATE INDEX IF NOT EXISTS idx_responses_survey_id ON responses(survey_id);
CREATE INDEX IF NOT EXISTS idx_response_slots_slot_id ON response_slots(slot_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);`)
	return err
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct{ Email, Password, Name string }
	if !decode(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if req.Email == "" || len(req.Password) < 8 || req.Name == "" {
		writeError(w, http.StatusBadRequest, "email, name, and password of at least 8 characters are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}
	res, err := s.db.Exec(`INSERT INTO users(email,name,password_hash,created_at) VALUES(?,?,?,?)`, req.Email, req.Name, string(hash), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		writeError(w, http.StatusConflict, "email is already registered")
		return
	}
	id, _ := res.LastInsertId()
	user := User{ID: id, Email: req.Email, Name: req.Name}
	s.setSession(w, user.ID)
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct{ Email, Password string }
	if !decode(w, r, &req) {
		return
	}
	var user User
	var hash string
	err := s.db.QueryRow(`SELECT id,email,name,password_hash FROM users WHERE email=?`, strings.ToLower(strings.TrimSpace(req.Email))).Scan(&user.ID, &user.Email, &user.Name, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	s.setSession(w, user.ID)
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token=?`, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteLaxMode})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleSurveys(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.listSurveys(w, user.ID)
	case http.MethodPost:
		s.createSurvey(w, r, user.ID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) createSurvey(w http.ResponseWriter, r *http.Request, userID int64) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Title, Description, Timezone string
		Slots                        []TimeSlot
	}
	if !decode(w, r, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.Timezone = strings.TrimSpace(req.Timezone)
	if req.Title == "" || req.Timezone == "" || len(req.Slots) == 0 {
		writeError(w, http.StatusBadRequest, "title, timezone, and at least one slot are required")
		return
	}
	if len(req.Title) > 200 || len(req.Description) > 2000 || len(req.Slots) > 200 {
		writeError(w, http.StatusBadRequest, "survey exceeds maximum title, description, or slot count")
		return
	}
	token := uuid.NewString()
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start transaction")
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO surveys(title,description,timezone,share_token,created_by,created_at) VALUES(?,?,?,?,?,?)`, req.Title, req.Description, req.Timezone, token, userID, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create survey")
		return
	}
	surveyID, _ := res.LastInsertId()
	slots := make([]TimeSlot, 0, len(req.Slots))
	seenSlots := map[string]bool{}
	for _, slot := range req.Slots {
		if !slot.End.After(slot.Start) {
			writeError(w, http.StatusBadRequest, "slot end must be after start")
			return
		}
		slotKey := slot.Start.UTC().Format(time.RFC3339Nano) + "/" + slot.End.UTC().Format(time.RFC3339Nano)
		if seenSlots[slotKey] {
			writeError(w, http.StatusBadRequest, "duplicate candidate slot")
			return
		}
		seenSlots[slotKey] = true
		res, err := tx.Exec(`INSERT INTO survey_slots(survey_id,starts_at,ends_at) VALUES(?,?,?)`, surveyID, slot.Start.UTC().Format(time.RFC3339Nano), slot.End.UTC().Format(time.RFC3339Nano))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create slot")
			return
		}
		slot.ID, _ = res.LastInsertId()
		slot.Survey = surveyID
		slots = append(slots, slot)
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save survey")
		return
	}
	writeJSON(w, http.StatusCreated, Survey{ID: surveyID, Title: req.Title, Description: req.Description, Timezone: req.Timezone, ShareToken: token, CreatedBy: userID, Slots: slots})
}

func (s *Server) listSurveys(w http.ResponseWriter, userID int64) {
	rows, err := s.db.Query(`SELECT id,title,description,timezone,share_token,created_by FROM surveys WHERE created_by=? ORDER BY id DESC`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list surveys")
		return
	}
	defer rows.Close()
	var surveys []Survey
	for rows.Next() {
		var survey Survey
		if err := rows.Scan(&survey.ID, &survey.Title, &survey.Description, &survey.Timezone, &survey.ShareToken, &survey.CreatedBy); err != nil {
			writeError(w, http.StatusInternalServerError, "could not load survey")
			return
		}
		slots, err := s.loadSlots(survey.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load survey slots")
			return
		}
		survey.Slots = slots
		surveys = append(surveys, survey)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not list surveys")
		return
	}
	writeJSON(w, http.StatusOK, surveys)
}

func (s *Server) handleSurveyDetail(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/surveys/"), "/")
	if len(parts) == 2 && parts[1] == "results" && r.Method == http.MethodGet {
		s.getResults(w, parts[0], user.ID)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) handlePublicSurvey(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/public/surveys/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		s.getPublicSurvey(w, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "responses" && r.Method == http.MethodPost {
		s.createResponse(w, r, parts[0])
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s *Server) getPublicSurvey(w http.ResponseWriter, token string) {
	survey, err := s.loadSurveyByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "survey not found")
		return
	}
	writeJSON(w, http.StatusOK, publicSurvey(survey))
}

func (s *Server) createResponse(w http.ResponseWriter, r *http.Request, token string) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	survey, err := s.loadSurveyByToken(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "survey not found")
		return
	}
	valid := map[int64]bool{}
	for _, slot := range survey.Slots {
		valid[slot.ID] = true
	}
	var req struct {
		RespondentName string  `json:"respondent_name"`
		SlotIDs        []int64 `json:"slot_ids"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.RespondentName = strings.TrimSpace(req.RespondentName)
	if req.RespondentName == "" {
		writeError(w, http.StatusBadRequest, "respondent name is required")
		return
	}
	if len(req.RespondentName) > 200 || len(req.SlotIDs) > len(survey.Slots) {
		writeError(w, http.StatusBadRequest, "response exceeds maximum name or slot count")
		return
	}
	seen := map[int64]bool{}
	for _, id := range req.SlotIDs {
		if !valid[id] {
			writeError(w, http.StatusBadRequest, "slot does not belong to survey")
			return
		}
		if seen[id] {
			writeError(w, http.StatusBadRequest, "duplicate slot selection")
			return
		}
		seen[id] = true
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start transaction")
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO responses(survey_id,respondent_name,created_at) VALUES(?,?,?)`, survey.ID, req.RespondentName, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save response")
		return
	}
	responseID, _ := res.LastInsertId()
	for _, id := range req.SlotIDs {
		if _, err := tx.Exec(`INSERT INTO response_slots(response_id,slot_id) VALUES(?,?)`, responseID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not save response slot")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save response")
		return
	}
	writeJSON(w, http.StatusCreated, Response{ID: responseID, RespondentName: req.RespondentName, SlotIDs: req.SlotIDs})
}

func (s *Server) getResults(w http.ResponseWriter, token string, userID int64) {
	survey, err := s.loadSurveyByToken(token)
	if err != nil || survey.CreatedBy != userID {
		writeError(w, http.StatusNotFound, "survey not found")
		return
	}
	rows, err := s.db.Query(`SELECT r.id,r.respondent_name,rs.slot_id FROM responses r LEFT JOIN response_slots rs ON r.id=rs.response_id WHERE r.survey_id=? ORDER BY r.id`, survey.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load results")
		return
	}
	defer rows.Close()
	byID := map[int64]*Response{}
	responseOrder := []int64{}
	counts := map[int64]int{}
	respondents := map[int64][]string{}
	for _, slot := range survey.Slots {
		counts[slot.ID] = 0
		respondents[slot.ID] = []string{}
	}
	for rows.Next() {
		var rid int64
		var name string
		var slot sql.NullInt64
		if err := rows.Scan(&rid, &name, &slot); err != nil {
			writeError(w, http.StatusInternalServerError, "could not read results")
			return
		}
		if byID[rid] == nil {
			byID[rid] = &Response{ID: rid, RespondentName: name}
			responseOrder = append(responseOrder, rid)
		}
		if slot.Valid {
			byID[rid].SlotIDs = append(byID[rid].SlotIDs, slot.Int64)
			counts[slot.Int64]++
			respondents[slot.Int64] = append(respondents[slot.Int64], name)
		}
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not load results")
		return
	}
	responses := make([]Response, 0, len(byID))
	for _, id := range responseOrder {
		responses = append(responses, *byID[id])
	}
	writeJSON(w, http.StatusOK, SurveyResults{Survey: survey, Responses: responses, SlotCounts: counts, Respondents: respondents})
}

func (s *Server) loadSurveyByToken(token string) (Survey, error) {
	var survey Survey
	err := s.db.QueryRow(`SELECT id,title,description,timezone,share_token,created_by FROM surveys WHERE share_token=?`, token).Scan(&survey.ID, &survey.Title, &survey.Description, &survey.Timezone, &survey.ShareToken, &survey.CreatedBy)
	if err != nil {
		return Survey{}, err
	}
	survey.Slots, err = s.loadSlots(survey.ID)
	return survey, err
}

func (s *Server) loadSlots(surveyID int64) ([]TimeSlot, error) {
	rows, err := s.db.Query(`SELECT id,starts_at,ends_at FROM survey_slots WHERE survey_id=? ORDER BY starts_at`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var slots []TimeSlot
	for rows.Next() {
		var slot TimeSlot
		var start, end string
		if err := rows.Scan(&slot.ID, &start, &end); err != nil {
			return nil, err
		}
		var err error
		slot.Start, err = time.Parse(time.RFC3339Nano, start)
		if err != nil {
			return nil, err
		}
		slot.End, err = time.Parse(time.RFC3339Nano, end)
		if err != nil {
			return nil, err
		}
		slot.Survey = surveyID
		slots = append(slots, slot)
	}
	return slots, rows.Err()
}

func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) (User, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "login required")
		return User{}, false
	}
	var user User
	var expires string
	err = s.db.QueryRow(`SELECT u.id,u.email,u.name,s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token=?`, cookie.Value).Scan(&user.ID, &user.Email, &user.Name, &expires)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "login required")
		return User{}, false
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || time.Now().UTC().After(expiresAt) {
		writeError(w, http.StatusUnauthorized, "session expired")
		return User{}, false
	}
	return user, true
}

func (s *Server) setSession(w http.ResponseWriter, userID int64) {
	token := uuid.NewString()
	expires := time.Now().UTC().Add(30 * 24 * time.Hour)
	_, _ = s.db.Exec(`INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)`, token, userID, expires.Format(time.RFC3339Nano))
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/", Expires: expires, HttpOnly: true, Secure: s.secureCookies, SameSite: http.SameSiteLaxMode})
}

func (s *Server) requireMethod(method string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if origin != "" && !s.allowedOrigins[origin] && r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func publicSurvey(survey Survey) PublicSurvey {
	return PublicSurvey{ID: survey.ID, Title: survey.Title, Description: survey.Description, Timezone: survey.Timezone, ShareToken: survey.ShareToken, Slots: survey.Slots}
}

func loadAllowedOrigins() map[string]bool {
	origins := map[string]bool{
		"http://localhost:5173": true,
		"http://127.0.0.1:5173": true,
	}
	for _, origin := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = true
		}
	}
	return origins
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid json: multiple JSON values")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

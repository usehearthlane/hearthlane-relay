package server

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"hearthlane-relay/internal/state"
	"hearthlane-relay/internal/store"
)

const (
	maxBodyBytes      = 16 * 1024
	maxNicknameLength = 200
)

type Server struct {
	st     *state.State
	store  *store.Store
	token  string
	logger *log.Logger
}

type errorResponse struct {
	Error string `json:"error"`
}

type deviceListResponse struct {
	Devices []state.DeviceSummary `json:"devices"`
}

func New(st *state.State, sts *store.Store, token string, logger *log.Logger) *Server {
	return &Server{st: st, store: sts, token: token, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /devices/{deviceID}/location", s.handlePutLocation)
	mux.HandleFunc("GET /devices", s.handleListDevices)
	mux.HandleFunc("GET /devices/{deviceID}/location", s.handleGetLocation)
	mux.HandleFunc("PUT /devices/{deviceID}/nickname", s.handlePutNickname)
	return s.auth(s.logRequests(mux))
}

func (s *Server) handlePutLocation(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := s.requireDeviceID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req locationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	loc, errMsg := buildLocation(&req)
	if errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}
	loc.PublishedAtEpochMs = time.Now().UnixMilli()
	s.st.PublishLocation(deviceID, loc)
	if err := s.persist(); err != nil {
		s.logger.Printf("error persisting state: %v", err)
		writeError(w, http.StatusInternalServerError, "persistence error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetLocation(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := s.requireDeviceID(w, r)
	if !ok {
		return
	}
	loc, found := s.st.Location(deviceID)
	if !found {
		writeError(w, http.StatusNotFound, "location not found")
		return
	}
	writeJSON(w, http.StatusOK, loc)
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, deviceListResponse{Devices: s.st.DeviceList()})
}

func (s *Server) handlePutNickname(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := s.requireDeviceID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req nicknameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Nickname == nil {
		writeError(w, http.StatusBadRequest, "nickname is required")
		return
	}
	nick := *req.Nickname
	if len(nick) > maxNicknameLength {
		writeError(w, http.StatusBadRequest, "nickname too long")
		return
	}
	s.st.SetNickname(deviceID, nick)
	if err := s.persist(); err != nil {
		s.logger.Printf("error persisting state: %v", err)
		writeError(w, http.StatusInternalServerError, "persistence error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) persist() error {
	data, err := s.st.Snapshot()
	if err != nil {
		return fmt.Errorf("serialize state: %w", err)
	}
	return s.store.Save(data)
}

func (s *Server) requireDeviceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("deviceID")
	if !validDeviceID(id) {
		writeError(w, http.StatusBadRequest, "invalid deviceId")
		return "", false
	}
	return id, true
}

func validDeviceID(id string) bool {
	if id == "" {
		return false
	}
	if len(id) > state.MaxDeviceIDLength {
		return false
	}
	if strings.ContainsRune(id, '/') {
		return false
	}
	for _, c := range id {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" && !authorized(r.Header.Get("Authorization"), s.token) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authorized(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(strings.ToLower(header), strings.ToLower(prefix)) {
		return false
	}
	got := []byte(header[len(prefix):])
	want := []byte(token)
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.logger.Printf("method=%s path=%s status=%d duration=%s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Microsecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

type locationRequest struct {
	Latitude          *float64 `json:"latitude"`
	Longitude         *float64 `json:"longitude"`
	Accuracy          *float64 `json:"accuracy"`
	Provider          *string  `json:"provider"`
	RecordedAtEpochMs *int64   `json:"recordedAtEpochMs"`
	PublishedAtEpoch  *int64   `json:"publishedAtEpochMs"`
}

func buildLocation(req *locationRequest) (*state.Location, string) {
	if req.Latitude == nil {
		return nil, "latitude is required"
	}
	if req.Longitude == nil {
		return nil, "longitude is required"
	}
	if err := state.ValidateCoordinates(*req.Latitude, *req.Longitude); err != nil {
		return nil, err.Error()
	}
	loc := &state.Location{Latitude: *req.Latitude, Longitude: *req.Longitude}
	if req.Accuracy != nil {
		if *req.Accuracy < 0 {
			return nil, "accuracy must be >= 0"
		}
		a := *req.Accuracy
		loc.Accuracy = &a
	}
	if req.Provider != nil {
		p := *req.Provider
		loc.Provider = &p
	}
	if req.RecordedAtEpochMs != nil {
		if *req.RecordedAtEpochMs < 0 {
			return nil, "recordedAtEpochMs must be >= 0"
		}
		r := *req.RecordedAtEpochMs
		loc.RecordedAtEpochMs = &r
	}
	return loc, ""
}

type nicknameRequest struct {
	Nickname *string `json:"nickname"`
}

package state

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
	"sync"
)

const MaxDeviceIDLength = 128

func ValidateCoordinates(lat, lon float64) error {
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return errors.New("latitude out of range")
	}
	if math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180 || lon > 180 {
		return errors.New("longitude out of range")
	}
	return nil
}

type Location struct {
	Latitude           float64  `json:"latitude"`
	Longitude          float64  `json:"longitude"`
	Accuracy           *float64 `json:"accuracy,omitempty"`
	Provider           *string  `json:"provider,omitempty"`
	RecordedAtEpochMs  *int64   `json:"recordedAtEpochMs,omitempty"`
	PublishedAtEpochMs int64    `json:"publishedAtEpochMs"`
}

type Device struct {
	Nickname string    `json:"nickname,omitempty"`
	Location *Location `json:"location,omitempty"`
}

type State struct {
	mu      sync.RWMutex
	Devices map[string]*Device `json:"devices"`
}

func New() *State {
	return &State{Devices: make(map[string]*Device)}
}

func (s *State) PublishLocation(deviceID string, loc *Location) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Devices == nil {
		s.Devices = make(map[string]*Device)
	}
	dev, ok := s.Devices[deviceID]
	if !ok {
		dev = &Device{}
		s.Devices[deviceID] = dev
	}
	dev.Location = loc
}

func (s *State) SetNickname(deviceID, nickname string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Devices == nil {
		s.Devices = make(map[string]*Device)
	}
	dev, ok := s.Devices[deviceID]
	if !ok {
		if nickname == "" {
			return
		}
		dev = &Device{}
		s.Devices[deviceID] = dev
	}
	dev.Nickname = nickname
	if nickname == "" && dev.Location == nil {
		delete(s.Devices, deviceID)
	}
}

func (s *State) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s, "", "  ")
}

type DeviceSummary struct {
	DeviceID           string   `json:"deviceId"`
	Nickname           string   `json:"nickname,omitempty"`
	HasLocation        bool     `json:"hasLocation"`
	PublishedAtEpochMs *int64   `json:"publishedAtEpochMs,omitempty"`
	Accuracy           *float64 `json:"accuracy,omitempty"`
}

func (s *State) DeviceList() []DeviceSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.Devices))
	for id := range s.Devices {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	list := make([]DeviceSummary, 0, len(ids))
	for _, id := range ids {
		dev := s.Devices[id]
		ds := DeviceSummary{DeviceID: id, Nickname: dev.Nickname}
		if dev.Location != nil {
			ds.HasLocation = true
			p := dev.Location.PublishedAtEpochMs
			ds.PublishedAtEpochMs = &p
			if dev.Location.Accuracy != nil {
				a := *dev.Location.Accuracy
				ds.Accuracy = &a
			}
		}
		list = append(list, ds)
	}
	return list
}

type LocationView struct {
	DeviceID           string   `json:"deviceId"`
	Latitude           float64  `json:"latitude"`
	Longitude          float64  `json:"longitude"`
	Accuracy           *float64 `json:"accuracy,omitempty"`
	Provider           *string  `json:"provider,omitempty"`
	RecordedAtEpochMs  *int64   `json:"recordedAtEpochMs,omitempty"`
	PublishedAtEpochMs int64    `json:"publishedAtEpochMs"`
}

func (s *State) Location(deviceID string) (LocationView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dev, ok := s.Devices[deviceID]
	if !ok || dev.Location == nil {
		return LocationView{}, false
	}
	l := dev.Location
	v := LocationView{
		DeviceID:           deviceID,
		Latitude:           l.Latitude,
		Longitude:          l.Longitude,
		PublishedAtEpochMs: l.PublishedAtEpochMs,
	}
	if l.Accuracy != nil {
		a := *l.Accuracy
		v.Accuracy = &a
	}
	if l.Provider != nil {
		p := *l.Provider
		v.Provider = &p
	}
	if l.RecordedAtEpochMs != nil {
		r := *l.RecordedAtEpochMs
		v.RecordedAtEpochMs = &r
	}
	return v, true
}

func (s *State) DeviceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Devices)
}

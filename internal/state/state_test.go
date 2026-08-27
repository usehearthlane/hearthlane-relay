package state

import (
	"encoding/json"
	"strings"
	"testing"
)

func newLoc(lat, lon float64, published int64) *Location {
	return &Location{Latitude: lat, Longitude: lon, PublishedAtEpochMs: published}
}

func TestPublishReplacesLocation(t *testing.T) {
	s := New()
	s.PublishLocation("d1", newLoc(1, 2, 100))
	l1, ok1 := s.Location("d1")
	if !ok1 || l1.Latitude != 1 || l1.Longitude != 2 || l1.PublishedAtEpochMs != 100 {
		t.Fatalf("unexpected first location: %+v ok=%v", l1, ok1)
	}
	s.PublishLocation("d1", newLoc(3, 4, 200))
	l2, ok2 := s.Location("d1")
	if !ok2 {
		t.Fatal("expected location after replacement")
	}
	if l2.Latitude != 3 || l2.Longitude != 4 || l2.PublishedAtEpochMs != 200 {
		t.Fatalf("unexpected second location: %+v", l2)
	}
}

func TestAtMostOneLocation(t *testing.T) {
	s := New()
	s.PublishLocation("d1", newLoc(1, 2, 100))
	s.PublishLocation("d1", newLoc(3, 4, 200))
	if s.DeviceCount() != 1 {
		t.Fatalf("expected 1 device, got %d", s.DeviceCount())
	}
	loc, ok := s.Location("d1")
	if !ok || loc.Latitude != 3 {
		t.Fatalf("expected single latest location, got %+v", loc)
	}
}

func TestLocationNotFound(t *testing.T) {
	s := New()
	if _, ok := s.Location("unknown"); ok {
		t.Fatal("expected no location")
	}
}

func TestSetNicknameDoesNotAffectLocation(t *testing.T) {
	s := New()
	s.PublishLocation("d1", newLoc(1, 2, 100))
	s.SetNickname("d1", "Meu celular")
	l, ok := s.Location("d1")
	if !ok || l.Latitude != 1 || l.Longitude != 2 {
		t.Fatalf("location changed by nickname: %+v", l)
	}
}

func TestPublishKeepsNickname(t *testing.T) {
	s := New()
	s.SetNickname("d1", "Meu celular")
	s.PublishLocation("d1", newLoc(1, 2, 100))
	if got := s.DeviceList(); len(got) != 1 || got[0].Nickname != "Meu celular" {
		t.Fatalf("nickname lost after publish: %+v", got)
	}
}

func TestClearNicknameKeepsDeviceWithLocation(t *testing.T) {
	s := New()
	s.PublishLocation("d1", newLoc(1, 2, 100))
	s.SetNickname("d1", "x")
	s.SetNickname("d1", "")
	if s.DeviceCount() != 1 {
		t.Fatalf("device with location should remain, got %d devices", s.DeviceCount())
	}
	if l, ok := s.Location("d1"); !ok || l.Latitude != 1 {
		t.Fatalf("location lost after clearing nickname: %+v", l)
	}
}

func TestClearNicknameRemovesDeviceWithoutLocation(t *testing.T) {
	s := New()
	s.SetNickname("d1", "x")
	s.SetNickname("d1", "")
	if s.DeviceCount() != 0 {
		t.Fatalf("device without location and with cleared nickname should be dropped, got %d", s.DeviceCount())
	}
}

func TestSnapshotShape(t *testing.T) {
	s := New()
	s.PublishLocation("d1", &Location{
		Latitude:           -23.55,
		Longitude:          -46.63,
		PublishedAtEpochMs: 1234567895000,
	})
	s.SetNickname("d1", "Meu celular")
	data, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["devices"]; !ok {
		t.Fatalf("snapshot missing devices key: %s", data)
	}
	var devices map[string]*Device
	if err := json.Unmarshal(parsed["devices"], &devices); err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	dev := devices["d1"]
	if dev == nil || dev.Nickname != "Meu celular" || dev.Location == nil {
		t.Fatalf("unexpected device in snapshot: %+v", dev)
	}
	if dev.Location.Latitude != -23.55 || dev.Location.Longitude != -46.63 {
		t.Fatalf("unexpected location in snapshot: %+v", dev.Location)
	}
}

func TestSnapshotHasNoHistory(t *testing.T) {
	s := New()
	s.PublishLocation("d1", newLoc(1, 2, 100))
	s.PublishLocation("d1", newLoc(3, 4, 200))
	data, _ := s.Snapshot()
	for _, word := range []string{"locations", "history", "events", "timestamp"} {
		if strings.Contains(string(data), word) {
			t.Fatalf("snapshot must not contain %q: %s", word, data)
		}
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	var devices map[string]*Device
	if err := json.Unmarshal(parsed["devices"], &devices); err != nil {
		t.Fatal(err)
	}
	if devices["d1"].Location.Latitude != 3 {
		t.Fatalf("snapshot kept old location: %s", data)
	}
}

func TestDeviceListSortedWithoutCoordinates(t *testing.T) {
	s := New()
	s.PublishLocation("d2", &Location{Latitude: 2, Longitude: 3, PublishedAtEpochMs: 200})
	s.SetNickname("d1", "b")
	s.PublishLocation("d1", &Location{Latitude: 1, Longitude: 2, Accuracy: new(float64), PublishedAtEpochMs: 100})
	list := s.DeviceList()
	if len(list) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(list))
	}
	if list[0].DeviceID != "d1" || list[1].DeviceID != "d2" {
		t.Fatalf("expected sorted list, got %+v", list)
	}
	if list[0].Nickname != "b" || !list[0].HasLocation || list[0].PublishedAtEpochMs == nil {
		t.Fatalf("unexpected summary: %+v", list[0])
	}
	list[1].Accuracy = nil
}

func TestLocationViewDeepCopy(t *testing.T) {
	s := New()
	s.PublishLocation("d1", &Location{Latitude: 1, Longitude: 2, PublishedAtEpochMs: 100})
	v1, _ := s.Location("d1")
	s.PublishLocation("d1", &Location{Latitude: 3, Longitude: 4, PublishedAtEpochMs: 200})
	if v1.Latitude != 1 || v1.Longitude != 2 || v1.PublishedAtEpochMs != 100 {
		t.Fatalf("view mutated by later publish: %+v", v1)
	}
}

func TestValidateCoordinates(t *testing.T) {
	valid := [][2]float64{{0, 0}, {-90, 90}, {90, -180}}
	for _, v := range valid {
		if err := ValidateCoordinates(v[0], v[1]); err != nil {
			t.Fatalf("expected valid %v: %v", v, err)
		}
	}
	invalid := [][2]float64{{91, 0}, {-91, 0}, {0, 181}, {0, -181}}
	for _, v := range invalid {
		if err := ValidateCoordinates(v[0], v[1]); err == nil {
			t.Fatalf("expected invalid %v", v)
		}
	}
}

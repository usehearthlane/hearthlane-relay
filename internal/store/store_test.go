package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hearthlane-relay/internal/state"
)

func TestLoadMissingFile(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "state.json"))
	s, err := st.Load()
	if err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	if s.DeviceCount() != 0 {
		t.Fatalf("expected empty state, got %d devices", s.DeviceCount())
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	writeFile(t, path, `{
  "devices": {
    "d1": {
      "nickname": "Meu celular",
      "location": {
        "latitude": -23.55,
        "longitude": -46.63,
        "accuracy": 50,
        "provider": "network",
        "recordedAtEpochMs": 1234567890000,
        "publishedAtEpochMs": 1234567895000
      }
    }
  }
}`)
	s, err := New(path).Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if s.DeviceCount() != 1 {
		t.Fatalf("expected 1 device, got %d", s.DeviceCount())
	}
	loc, ok := s.Location("d1")
	if !ok {
		t.Fatal("expected location")
	}
	if loc.Latitude != -23.55 || loc.Longitude != -46.63 || loc.PublishedAtEpochMs != 1234567895000 {
		t.Fatalf("unexpected location: %+v", loc)
	}
	if loc.Accuracy == nil || *loc.Accuracy != 50 {
		t.Fatalf("unexpected accuracy: %+v", loc.Accuracy)
	}
	if loc.Provider == nil || *loc.Provider != "network" {
		t.Fatalf("unexpected provider: %+v", loc.Provider)
	}
}

func TestLoadCorruptFileReportsErrorAndKeepsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	writeFile(t, path, `{"devices": not-json`)
	_, err := New(path).Load()
	if err == nil {
		t.Fatal("expected error for corrupt file")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("corrupt file must not be deleted: %v", statErr)
	}
}

func TestLoadInvalidCoordinates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeFile(t, path, `{"devices":{"d1":{"location":{"latitude":91,"longitude":0,"publishedAtEpochMs":10}}}}`)
	if _, err := New(path).Load(); err == nil {
		t.Fatal("expected error for out-of-range latitude")
	}
}

func TestLoadNullDevice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	writeFile(t, path, `{"devices":{"d1":null}}`)
	if _, err := New(path).Load(); err == nil {
		t.Fatal("expected error for null device")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st := New(path)
	data := []byte(`{"devices":{"d1":{"nickname":"x","location":{"latitude":1,"longitude":2,"publishedAtEpochMs":100}}}}`)
	if err := st.Save(data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &b); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	an, _ := json.Marshal(a)
	bn, _ := json.Marshal(b)
	if string(an) != string(bn) {
		t.Fatalf("round trip mismatch: %s != %s", got, data)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestOverwriteOnSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st := New(path)
	if err := st.Save([]byte(`{"devices":{"d1":{"location":{"latitude":1,"longitude":2,"publishedAtEpochMs":100}}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := st.Save([]byte(`{"devices":{"d2":{"location":{"latitude":3,"longitude":4,"publishedAtEpochMs":200}}}}`)); err != nil {
		t.Fatal(err)
	}
	s, err := New(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.DeviceCount() != 1 {
		t.Fatalf("expected updated state, got %d devices", s.DeviceCount())
	}
	if _, ok := s.Location("d2"); !ok {
		t.Fatal("expected d2 to be present after overwrite")
	}
}

func TestSaveToMissingDir(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "missing", "state.json"))
	if err := st.Save([]byte(`{}`)); err == nil {
		t.Fatal("expected error saving to missing directory")
	}
}

func TestLoadAfterSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st := New(path)
	s := state.New()
	s.PublishLocation("d1", &state.Location{Latitude: -23.55, Longitude: -46.63, PublishedAtEpochMs: 123})
	s.SetNickname("d1", "Celular")
	data, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(data); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	loc, ok := loaded.Location("d1")
	if !ok || loc.Latitude != -23.55 || loc.Longitude != -46.63 {
		t.Fatalf("unexpected loaded location: %+v ok=%v", loc, ok)
	}
}

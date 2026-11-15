package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemovePersistenceAndRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	e, err := s.Add("10000000-0000-4000-8000-000000000001", "WATCHING", 3)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(path+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Remove(e.ID); err == nil || ok {
		t.Fatal("failed persistence reported success")
	}
	if items := s.Library(); len(items) != 1 || items[0] != e {
		t.Fatal("failed removal changed memory")
	}
	if err = os.Remove(path + ".tmp"); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Remove(e.ID); err != nil || !ok {
		t.Fatal("remove failed", err)
	}
	restarted, err := New(path)
	if err != nil || len(restarted.Library()) != 0 {
		t.Fatal("removed entry returned after restart", err)
	}
	if ok, err := s.Remove(e.ID); err != nil || ok {
		t.Fatal("missing entry reported removed", err)
	}
	fresh, err := s.Add(e.AnimeID, "PLANNING", 0)
	if err != nil || fresh.ID == e.ID || fresh.Rating != 0 || fresh.EpisodesWatched != 0 {
		t.Fatal("re-add retained deleted state", err)
	}
}

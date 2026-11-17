package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVacuumIntoProducesUsableSnapshot(t *testing.T) {
	st := newTestStore(t)
	if _, err := st.CreateUser("alice", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("bob", "hash", "viewer"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	snap := filepath.Join(dir, "snapshot.db")
	if err := st.VacuumInto(snap); err != nil {
		t.Fatalf("VACUUM INTO: %v", err)
	}
	fi, err := os.Stat(snap)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() == 0 {
		t.Fatal("la instantánea está vacía")
	}
	if st.Size() <= 0 {
		t.Fatalf("Size() devolvió %d", st.Size())
	}

	// The snapshot must open as an independent database with the same rows.
	snapStore, err := Open(snap, nil)
	if err != nil {
		t.Fatalf("abrir instantánea: %v", err)
	}
	defer snapStore.Close()
	n, err := snapStore.CountUsers()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("usuarios en la instantánea: %d, se esperaban 2", n)
	}
}

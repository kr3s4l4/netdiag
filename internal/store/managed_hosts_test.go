package store

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func addManagedTestHost(t *testing.T, st *Store, ip string, lastSeen time.Time, tags []string) {
	t.Helper()
	addManagedTestHostAtSite(t, st, ip, 1, lastSeen, tags)
}

func addManagedTestHostAtSite(t *testing.T, st *Store, ip string, siteID int64, lastSeen time.Time, tags []string) {
	t.Helper()
	id, _, _, _, err := st.UpsertHost(&model.Host{
		SiteID:     siteID,
		IP:         ip,
		DeviceType: model.DevUnknown,
		Status:     model.StatusUp,
		FirstSeen:  lastSeen.Add(-time.Hour),
		LastSeen:   lastSeen,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tags != nil {
		notes := ""
		fav := false
		if err := st.UpdateHostMeta(id, &notes, &tags, &fav); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCountManagedHosts(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()

	addManagedTestHost(t, st, "192.168.1.10", now, nil)
	addManagedTestHost(t, st, "192.168.1.11", now, []string{"infra", "planta-1"})
	addManagedTestHost(t, st, "192.168.1.12", now, []string{"guest"})
	addManagedTestHost(t, st, "192.168.1.13", now, []string{"EXCLUDED"})
	addManagedTestHost(t, st, "192.168.1.14", now.Add(-40*24*time.Hour), nil)

	count, err := st.CountManagedHosts(now.Add(-30 * 24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("se esperaban 2 hosts gestionados, se obtuvieron %d", count)
	}
}

func TestCountManagedHostsBySiteSince(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	since := now.Add(-30 * 24 * time.Hour)

	addManagedTestHostAtSite(t, st, "192.168.2.10", 1, now, nil)
	addManagedTestHostAtSite(t, st, "192.168.2.11", 1, now, []string{"guest"})
	addManagedTestHostAtSite(t, st, "10.0.0.10", 2, now, nil)
	addManagedTestHostAtSite(t, st, "10.0.0.11", 2, now, []string{"excluded"})
	addManagedTestHostAtSite(t, st, "10.0.0.12", 3, now.Add(-40*24*time.Hour), nil)

	counts, err := st.CountManagedHostsBySiteSince(since)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 || counts[1] != 1 || counts[2] != 1 {
		t.Fatalf("conteo por sede inesperado: %#v", counts)
	}
}

package api

import (
	"context"
	"net/http"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/neighbors"
)

const neighborCacheTTL = 15 * time.Minute

// neighborSeeds collects private inventory addresses outside local subnets.
func neighborSeeds(hosts []model.Host) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, h := range hosts {
		if h.IP == "" || seen[h.IP] {
			continue
		}
		seen[h.IP] = true
		out = append(out, h.IP)
	}
	return out
}

// ensureNeighbors refreshes the neighbor cache at most once per TTL. The first
// call is automatic, so the UI shows detected subnets without asking the user.
func (s *Server) ensureNeighbors(ctx context.Context, localCIDRs, seedIPs []string) *neighbors.Result {
	s.neighborMu.RLock()
	res, at, running := s.neighborRes, s.neighborAt, s.neighborRunning
	s.neighborMu.RUnlock()
	if res != nil && !running && time.Since(at) < neighborCacheTTL {
		return res
	}
	if running {
		if res != nil {
			return res
		}
		return &neighbors.Result{Networks: []neighbors.Network{}, LocalNetworks: localCIDRs}
	}

	s.neighborMu.Lock()
	if s.neighborRunning {
		s.neighborMu.Unlock()
		if res != nil {
			return res
		}
		return &neighbors.Result{Networks: []neighbors.Network{}, LocalNetworks: localCIDRs}
	}
	s.neighborRunning = true
	s.neighborMu.Unlock()

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	fresh := neighbors.Discover(runCtx, localCIDRs, seedIPs)

	s.neighborMu.Lock()
	s.neighborRes = fresh
	s.neighborAt = time.Now()
	s.neighborRunning = false
	s.neighborMu.Unlock()
	return fresh
}

func (s *Server) cachedNeighbors() *neighbors.Result {
	s.neighborMu.RLock()
	defer s.neighborMu.RUnlock()
	if s.neighborRes == nil {
		return &neighbors.Result{Networks: []neighbors.Network{}}
	}
	return s.neighborRes
}

func (s *Server) handleNeighborDiscovery(w http.ResponseWriter, r *http.Request) {
	ifaces, err := s.systemInterfaces()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron leer las interfaces: "+err.Error())
		return
	}
	local := []string{}
	for _, iface := range ifaces {
		if iface.Loopback || !iface.Up {
			continue
		}
		for _, a := range iface.Addrs {
			if a.Family == "ipv4" && a.Private {
				local = append(local, a.CIDR)
			}
		}
	}
	hosts, err := s.st.ListAllHostsForSite(0, 20000)
	if err != nil {
		hosts = nil
	}
	res := s.ensureNeighbors(r.Context(), local, neighborSeeds(hosts))
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListNeighborNetworks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cachedNeighbors())
}

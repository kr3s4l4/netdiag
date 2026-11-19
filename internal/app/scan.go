package app

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/api"
	"github.com/kr3s4l4/netdiag/internal/discover"
	"github.com/kr3s4l4/netdiag/internal/history"
	"github.com/kr3s4l4/netdiag/internal/integrations"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
	"github.com/kr3s4l4/netdiag/internal/snmp"
)

// StartScan creates a queued scan and runs it in the background.
func (a *App) StartScan(req api.ScanRequest, actor string) (*model.Scan, error) {
	cfg := a.currentCfg()

	cidr := strings.TrimSpace(req.CIDR)
	if req.NetworkID > 0 && cidr == "" {
		if n, err := a.st.GetNetwork(req.NetworkID); err == nil {
			cidr = n.CIDR
		}
	}
	if cidr == "" {
		return nil, fmt.Errorf("indica una red o un CIDR para escanear")
	}
	if !strings.Contains(cidr, "/") {
		cidr += "/32"
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("CIDR inválido: %w", err)
	}
	if !prefix.Addr().Is4() {
		return nil, fmt.Errorf("solo se admiten redes IPv4")
	}
	prefix = prefix.Masked()
	cidr = prefix.String()

	allowPublic := req.AllowPublic || cfg.Scan.AllowPublic
	if !allowPublic && !netutil.IsPrivate(prefix.Addr().AsSlice()) {
		return nil, fmt.Errorf("la red es pública; activa scan.allow_public para escanearla")
	}

	siteID := req.SiteID
	var network *model.Network
	if req.NetworkID > 0 {
		network, err = a.st.GetNetwork(req.NetworkID)
		if err != nil {
			return nil, fmt.Errorf("red no encontrada")
		}
		if siteID > 0 && siteID != network.SiteID {
			return nil, fmt.Errorf("la red pertenece a otra sede")
		}
		siteID = network.SiteID
	} else {
		created, _, err := a.st.EnsureNetwork(&model.Network{
			SiteID:    siteID,
			Name:      cidr,
			CIDR:      cidr,
			Enabled:   true,
			CreatedAt: time.Now(),
		})
		if err != nil {
			return nil, fmt.Errorf("no se pudo registrar la red: %w", err)
		}
		network = created
		siteID = network.SiteID
	}
	if siteID <= 0 || !a.st.SiteExists(siteID) {
		return nil, fmt.Errorf("la sede indicada no existe")
	}

	profile := strings.ToLower(strings.TrimSpace(req.Profile))
	if profile == "" {
		profile = cfg.Scan.Profile
	}
	switch profile {
	case "quick", "common", "full", "custom":
	default:
		return nil, fmt.Errorf("perfil inválido: %s", profile)
	}
	if profile == "custom" && len(req.Ports) == 0 {
		return nil, fmt.Errorf("el perfil custom requiere una lista de puertos")
	}

	snmpEnabled := cfg.Scan.SNMP
	if req.SNMP != nil {
		snmpEnabled = *req.SNMP
	}
	timeout := cfg.Scan.Timeout.Duration()
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}
	if timeout < 50*time.Millisecond {
		timeout = 50 * time.Millisecond
	}
	if timeout > 10*time.Second {
		timeout = 10 * time.Second
	}
	trigger := "manual"
	if actor == "monitor" || actor == "scheduler" {
		trigger = "monitor"
	}
	options := model.ScanOptions{
		Profile:     profile,
		Ports:       req.Ports,
		SNMP:        snmpEnabled,
		Deep:        req.Deep,
		TimeoutMS:   int(timeout / time.Millisecond),
		Concurrency: cfg.Scan.Concurrency,
		MaxHosts:    cfg.Scan.MaxHosts,
		ResolveDNS:  cfg.Scan.ResolveDNS,
		BannerGrab:  cfg.Scan.BannerGrab,
		AllowPublic: allowPublic,
	}
	fullScan := network != nil && prefix.String() == network.CIDR
	if network == nil {
		fullScan = true
	}

	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil, fmt.Errorf("la aplicación se está apagando")
	}
	if len(a.runs) >= maxConcurrentScans {
		a.mu.Unlock()
		return nil, fmt.Errorf("ya hay %d escaneos en curso; espera a que terminen", maxConcurrentScans)
	}
	if id, ok := a.busyCIDR[cidr]; ok {
		a.mu.Unlock()
		return nil, fmt.Errorf("ya hay un escaneo en curso para %s (id %d)", cidr, id)
	}
	created, err := a.st.CreateScan(&model.Scan{
		SiteID:    siteID,
		NetworkID: network.ID,
		CIDR:      cidr,
		Profile:   profile,
		Trigger:   trigger,
		Options:   options,
	})
	if err != nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("no se pudo crear el escaneo: %w", err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	state := &runState{id: created.ID, networkID: network.ID, cidr: cidr, cancel: cancel}
	a.runs[created.ID] = state
	a.busyCIDR[cidr] = created.ID
	a.wg.Add(1)
	a.mu.Unlock()

	go a.runScan(runCtx, state, created, network, options, fullScan)
	return created, nil
}

// CancelScan stops a running scan.
func (a *App) CancelScan(id int64) error {
	a.mu.Lock()
	state, ok := a.runs[id]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("el escaneo no está en curso")
	}
	state.cancel()
	return nil
}

func (a *App) release(state *runState) {
	a.mu.Lock()
	delete(a.runs, state.id)
	if a.busyCIDR[state.cidr] == state.id {
		delete(a.busyCIDR, state.cidr)
	}
	a.mu.Unlock()
}

func (a *App) runScan(ctx context.Context, state *runState, scan *model.Scan, network *model.Network, opts model.ScanOptions, fullScan bool) {
	defer a.wg.Done()
	defer a.release(state)
	defer state.cancel()

	cfg := a.currentCfg()
	startedAt := time.Now()
	if err := a.st.StartScan(scan.ID, 0); err != nil {
		a.log.Error("no se pudo iniciar el escaneo", "scan", scan.ID, "error", err)
	}
	a.log.Info("escaneo iniciado", "scan", scan.ID, "cidr", scan.CIDR, "perfil", scan.Profile, "origen", scan.Trigger)

	snmpCfg := snmp.Config{
		Communities: cfg.Scan.SNMPCommunities,
		Version:     cfg.Scan.SNMPVersion,
		Timeout:     cfg.Scan.SNMPTimeout.Duration(),
		Retries:     1,
		Deep:        cfg.Diagnostics.SNMPDeep,
		V3User:      cfg.Scan.SNMPv3User,
		V3AuthPass:  cfg.Scan.SNMPv3Auth,
		V3PrivPass:  cfg.Scan.SNMPv3Priv,
		V3AuthProto: cfg.Scan.SNMPv3AuthProto,
		V3PrivProto: cfg.Scan.SNMPv3PrivProto,
	}
	runOpts := discover.Options{
		CIDR:            scan.CIDR,
		NetworkID:       network.ID,
		ScanID:          scan.ID,
		Interface:       network.Interface,
		Profile:         opts.Profile,
		Ports:           opts.Ports,
		Timeout:         time.Duration(opts.TimeoutMS) * time.Millisecond,
		Concurrency:     cfg.Scan.Concurrency,
		PortConcurrency: cfg.Scan.PortConcurrency,
		EnrichWorkers:   8,
		MaxHosts:        opts.MaxHosts,
		ResolveDNS:      opts.ResolveDNS,
		BannerGrab:      opts.BannerGrab,
		SNMP:            opts.SNMP,
		SNMPConfig:      snmpCfg,
		DHCPProbe:       cfg.Diagnostics.DHCPProbe,
		DHCPTimeout:     cfg.Diagnostics.DHCPProbeTimeout.Duration(),
		AllowPublic:     opts.AllowPublic,
		Trigger:         scan.Trigger,
	}

	lastPersist := time.Time{}
	result := a.engine.Run(ctx, runOpts, func(p discover.Progress) {
		if time.Since(lastPersist) > 800*time.Millisecond || p.Stage != "descubriendo" {
			_ = a.st.UpdateScanProgress(scan.ID, p.Stage, p.Scanned, p.Total, p.Found, p.Percent)
			lastPersist = time.Now()
		}
		a.hub.Broadcast("scan.progress", map[string]any{
			"scan_id":  scan.ID,
			"cidr":     scan.CIDR,
			"stage":    p.Stage,
			"scanned":  p.Scanned,
			"total":    p.Total,
			"found":    p.Found,
			"percent":  p.Percent,
			"canceled": p.Canceled,
		})
	})

	now := time.Now()
	seen := make([]int64, 0, len(result.Hosts))
	for i := range result.Hosts {
		h := result.Hosts[i]
		h.SiteID = scan.SiteID
		h.LastScanID = scan.ID
		id, created, prev, changes, err := a.st.UpsertHost(&h)
		if err != nil {
			a.log.Warn("no se pudo guardar el dispositivo", "ip", h.IP, "error", err)
			continue
		}
		h.ID = id
		result.Hosts[i] = h
		seen = append(seen, id)
		_ = a.st.AddMetric(id, h.LatencyMS, true, 0, now)
		if h.SNMP != nil && len(h.SNMP.Neighbors) > 0 {
			if err := a.st.ReplaceNeighbors(id, h.SNMP.Neighbors); err != nil {
				a.log.Warn("no se guardaron los vecinos", "host", h.IP, "error", err)
			}
		}
		a.alerts.EvaluateHost(prev, h, changes)
		event := "host.updated"
		if created {
			event = "host.new"
		}
		a.hub.Broadcast(event, h)
	}

	if result.Err == nil && !result.Canceled {
		a.recordARPObservations(scan.SiteID, scan.ID, result.Hosts, now)
		a.recordDHCPObservations(scan.SiteID, scan.ID, result, now)
		a.recordSNMPDeep(scan.SiteID, scan.ID, result.Hosts, now)
		a.recordPassive(scan, network.Interface, now)
	}

	if fullScan && network.ID > 0 && !result.Canceled {
		missing, err := a.st.MarkMissingHosts(network.ID, seen)
		if err != nil {
			a.log.Warn("no se pudieron marcar dispositivos ausentes", "error", err)
		}
		for _, m := range missing {
			_ = a.st.AddMetric(m.ID, 0, false, 100, now)
			prev := m
			prev.Status = model.StatusUp
			a.alerts.EvaluateHost(&prev, m, []string{"offline"})
			a.hub.Broadcast("host.updated", m)
		}
	}

	status := model.ScanDone
	errMsg := ""
	if result.Err != nil {
		if result.Canceled {
			status = model.ScanCanceled
		} else {
			status = model.ScanError
			errMsg = result.Err.Error()
		}
	}
	if err := a.st.FinishScan(scan.ID, status, result.Found, errMsg); err != nil {
		a.log.Warn("no se pudo cerrar el escaneo", "scan", scan.ID, "error", err)
	}
	if network.ID > 0 {
		_ = a.st.TouchNetworkScan(network.ID, now)
		if usage, err := a.st.ListHosts(model.HostFilter{NetworkID: network.ID, Status: "up", Page: 1, Limit: 1}); err == nil {
			if capacity, err := netutil.CountHosts(network.CIDR); err == nil {
				a.alerts.EvaluateNetwork(*network, usage.Total, capacity)
			}
		}
	}
	if status == model.ScanError {
		if final, err := a.st.GetScan(scan.ID); err == nil {
			a.alerts.EvaluateScanFailure(*final)
		}
	}

	final, err := a.st.GetScan(scan.ID)
	if err != nil {
		final = scan
	}
	a.hub.Broadcast("scan.finished", final)
	severity := model.SeverityInfo
	if status == model.ScanError {
		severity = model.SeverityCritical
	}
	a.integrations.Publish(integrations.Event{
		Kind: "scan.finished", SiteID: final.SiteID, Severity: severity,
		Title:     "Escaneo finalizado: " + final.CIDR,
		Message:   fmt.Sprintf("Estado %s · %d dispositivos encontrados", status, result.Found),
		Data:      map[string]any{"scan_id": scan.ID, "status": status, "found": result.Found, "error": errMsg},
		Timestamp: now,
	})
	if stats, err := a.st.Dashboard(final.SiteID); err == nil {
		a.hub.Broadcast("dashboard.updated", stats)
	}
	// F21: keep a lightweight inventory/topology capture so temporal diffs
	// remain possible after the live inventory has changed.
	if status != model.ScanError && !result.Canceled {
		if err := history.Capture(a.st, final.SiteID); err != nil {
			a.log.Warn("no se pudo capturar el snapshot histórico", "error", err)
		}
	}
	a.log.Info("escaneo finalizado", "scan", scan.ID, "estado", status,
		"encontrados", result.Found, "duracion", time.Since(startedAt).Round(time.Millisecond).String())
}

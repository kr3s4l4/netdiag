package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func jsonStringSlice(raw string) []string {
	var out []string
	if raw == "" || json.Unmarshal([]byte(raw), &out) != nil {
		return nil
	}
	return out
}

// siteSlugReplacer folds the Spanish characters that commonly appear in site
// names before the generic ASCII slug filter runs.
var siteSlugReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "ã", "a", "â", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "õ", "o", "ô", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c",
)

func slugify(name string) string {
	folded := siteSlugReplacer.Replace(strings.ToLower(strings.TrimSpace(name)))
	var b strings.Builder
	dash := true
	for _, r := range folded {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '-', r == '_', unicode.IsSpace(r):
			if !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "sede"
	}
	if len(out) > 48 {
		out = strings.Trim(out[:48], "-")
	}
	return out
}

// ListSites returns every site with its current usage counters.
func (s *Store) ListSites() ([]model.Site, error) {
	rows, err := s.db.Query(`SELECT s.id, s.name, s.slug, s.description, s.created_at, s.updated_at,
		(SELECT COUNT(*) FROM networks n WHERE n.site_id = s.id),
		(SELECT COUNT(*) FROM hosts h WHERE h.site_id = s.id),
		(SELECT COUNT(*) FROM hosts h WHERE h.site_id = s.id AND h.status = 'up'),
		(SELECT MAX(last_scan_at) FROM networks n WHERE n.site_id = s.id)
		FROM sites s ORDER BY s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Site
	for rows.Next() {
		var site model.Site
		var created, updated int64
		var lastScan sql.NullInt64
		if err := rows.Scan(&site.ID, &site.Name, &site.Slug, &site.Description, &created, &updated,
			&site.NetworkCount, &site.HostCount, &site.UpCount, &lastScan); err != nil {
			return nil, err
		}
		site.CreatedAt = fromMS(created)
		site.UpdatedAt = fromMS(updated)
		site.LastScanAt = fromMSNull(lastScan)
		out = append(out, site)
	}
	return out, rows.Err()
}

// GetSite returns one site by ID.
func (s *Store) GetSite(id int64) (*model.Site, error) {
	sites, err := s.ListSites()
	if err != nil {
		return nil, err
	}
	for i := range sites {
		if sites[i].ID == id {
			return &sites[i], nil
		}
	}
	return nil, ErrNotFound
}

// SiteExists reports whether a site exists.
func (s *Store) SiteExists(id int64) bool {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sites WHERE id = ?`, id).Scan(&n)
	return err == nil && n > 0
}

// CreateSite adds a site. The slug is derived from the name and made unique.
func (s *Store) CreateSite(name, description string) (*model.Site, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("el nombre de la sede es obligatorio")
	}
	if len([]rune(name)) > 80 {
		return nil, fmt.Errorf("el nombre de la sede no puede superar 80 caracteres")
	}
	base := slugify(name)
	slug := base
	for suffix := 2; ; suffix++ {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM sites WHERE slug = ?`, slug).Scan(&n); err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, suffix)
	}
	now := nowMS()
	res, err := s.db.Exec(`INSERT INTO sites(name, slug, description, created_at, updated_at) VALUES(?,?,?,?,?)`,
		name, slug, strings.TrimSpace(description), now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetSite(id)
}

// UpdateSite renames or redescribes an existing site.
func (s *Store) UpdateSite(id int64, name, description string) (*model.Site, error) {
	if !s.SiteExists(id) {
		return nil, ErrNotFound
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("el nombre de la sede es obligatorio")
	}
	if len([]rune(name)) > 80 {
		return nil, fmt.Errorf("el nombre de la sede no puede superar 80 caracteres")
	}
	_, err := s.db.Exec(`UPDATE sites SET name=?, description=?, updated_at=? WHERE id=?`,
		name, strings.TrimSpace(description), nowMS(), id)
	if err != nil {
		return nil, err
	}
	return s.GetSite(id)
}

// DeleteSite removes a site, moving its networks and hosts to another site.
// Network CIDR collisions are resolved by keeping the destination network and
// reattaching its hosts. The last remaining site cannot be deleted.
func (s *Store) DeleteSite(id, moveTo int64) error {
	sites, err := s.ListSites()
	if err != nil {
		return err
	}
	if len(sites) <= 1 {
		return fmt.Errorf("no se puede eliminar la única sede")
	}
	found := false
	for _, site := range sites {
		if site.ID == id {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	if moveTo <= 0 {
		for _, site := range sites {
			if site.ID != id {
				moveTo = site.ID
				break
			}
		}
	}
	if moveTo == id || !s.SiteExists(moveTo) {
		return fmt.Errorf("la sede de destino no es válida")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id, cidr FROM networks WHERE site_id = ?`, id)
	if err != nil {
		return err
	}
	type netRow struct {
		id   int64
		cidr string
	}
	var moving []netRow
	for rows.Next() {
		var nr netRow
		if err := rows.Scan(&nr.id, &nr.cidr); err != nil {
			rows.Close()
			return err
		}
		moving = append(moving, nr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, nr := range moving {
		var targetID int64
		err := tx.QueryRow(`SELECT id FROM networks WHERE site_id = ? AND cidr = ?`, moveTo, nr.cidr).Scan(&targetID)
		switch {
		case err == nil:
			if _, err := tx.Exec(`UPDATE hosts SET network_id = ? WHERE network_id = ?`, targetID, nr.id); err != nil {
				return err
			}
			if _, err := tx.Exec(`UPDATE scans SET network_id = ? WHERE network_id = ?`, targetID, nr.id); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM networks WHERE id = ?`, nr.id); err != nil {
				return err
			}
		case err == sql.ErrNoRows:
			if _, err := tx.Exec(`UPDATE networks SET site_id = ? WHERE id = ?`, moveTo, nr.id); err != nil {
				return err
			}
		default:
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE hosts SET site_id = ? WHERE site_id = ?`, moveTo, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE scans SET site_id = ? WHERE site_id = ?`, moveTo, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sites WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ListHostsForExport returns the hosts of a site with their stable identities.
func (s *Store) ListHostsForExport(siteID int64) ([]model.Host, error) {
	rows, err := s.db.Query(`SELECT `+hostColumns+` FROM hosts WHERE site_id = ? ORDER BY id`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *h)
	}
	return out, rows.Err()
}

// ExportSite builds a portable JSON bundle with the site definition, its
// networks and the stable identities plus user metadata of its hosts.
func (s *Store) ExportSite(siteID int64) (*model.SiteBundle, error) {
	site, err := s.GetSite(siteID)
	if err != nil {
		return nil, err
	}
	nets, err := s.ListNetworksBySite(siteID)
	if err != nil {
		return nil, err
	}
	hosts, err := s.ListHostsForExport(siteID)
	if err != nil {
		return nil, err
	}
	bundle := &model.SiteBundle{
		Schema:     "netdiag.site/v1",
		ExportedAt: time.Now().UTC(),
		Site: model.SiteInfo{
			Name:        site.Name,
			Slug:        site.Slug,
			Description: site.Description,
		},
		Networks: make([]model.BundleNetwork, 0, len(nets)),
		Hosts:    make([]model.BundleHost, 0, len(hosts)),
	}
	for _, n := range nets {
		bundle.Networks = append(bundle.Networks, model.BundleNetwork{
			Name: n.Name, CIDR: n.CIDR, Interface: n.Interface, Enabled: n.Enabled,
		})
	}
	for _, h := range hosts {
		bundle.Hosts = append(bundle.Hosts, model.BundleHost{
			Identity: rawIdentity(h.Identity), IP: h.IP, MAC: h.MAC, Hostname: h.Hostname,
			Vendor: h.Vendor, Model: h.Model, OS: h.OS,
			DeviceType: string(h.DeviceType), Favorite: h.Favorite,
			Notes: h.Notes, Tags: h.Tags,
		})
	}
	return bundle, nil
}

// ImportSummary reports what an import changed.
type ImportSummary struct {
	NetworksCreated int `json:"networks_created"`
	HostsCreated    int `json:"hosts_created"`
	HostsUpdated    int `json:"hosts_updated"`
	HostsSkipped    int `json:"hosts_skipped"`
}

// ImportSite merges or replaces a bundle inside an existing site. Merge never
// deletes data; replace clears networks, hosts and scans of the destination
// site first. Hosts are matched by their stable identity (MAC or ip:<addr>).
func (s *Store) ImportSite(siteID int64, bundle *model.SiteBundle, replace bool) (ImportSummary, error) {
	var summary ImportSummary
	if !s.SiteExists(siteID) {
		return summary, ErrNotFound
	}
	if bundle == nil {
		return summary, fmt.Errorf("el paquete está vacío")
	}
	if replace {
		tx, err := s.db.Begin()
		if err != nil {
			return summary, err
		}
		for _, stmt := range []string{
			`DELETE FROM hosts WHERE site_id = ?`,
			`DELETE FROM networks WHERE site_id = ?`,
			`DELETE FROM scans WHERE site_id = ?`,
		} {
			if _, err := tx.Exec(stmt, siteID); err != nil {
				tx.Rollback()
				return summary, err
			}
		}
		if err := tx.Commit(); err != nil {
			return summary, err
		}
	}

	netByCIDR := map[string]int64{}
	if existing, err := s.ListNetworksBySite(siteID); err == nil {
		for _, n := range existing {
			netByCIDR[n.CIDR] = n.ID
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return summary, err
	}
	defer tx.Rollback()
	for _, n := range bundle.Networks {
		cidr := strings.TrimSpace(n.CIDR)
		if cidr == "" || netByCIDR[cidr] != 0 {
			continue
		}
		name := strings.TrimSpace(n.Name)
		if name == "" {
			name = cidr
		}
		res, err := tx.Exec(`INSERT INTO networks(site_id, name, cidr, iface, enabled, is_local, created_at)
			VALUES(?,?,?,?,?,0,?)`, siteID, name, cidr, n.Interface, boolInt(n.Enabled), nowMS())
		if err != nil {
			return summary, err
		}
		id, _ := res.LastInsertId()
		netByCIDR[cidr] = id
		summary.NetworksCreated++
	}

	for i := range bundle.Hosts {
		bh := bundle.Hosts[i]
		identity := strings.ToLower(strings.TrimSpace(rawIdentity(bh.Identity)))
		if identity == "" {
			identity = IdentityFor(bh.IP, bh.MAC)
		}
		if identity != "" {
			identity = fmt.Sprintf("s%d:%s", siteID, identity)
		}
		if identity == "" {
			summary.HostsSkipped++
			continue
		}
		networkID := matchHostNetwork(bh.IP, netByCIDR)
		var id int64
		err := tx.QueryRow(`SELECT id FROM hosts WHERE identity = ?`, identity).Scan(&id)
		if err == sql.ErrNoRows {
			now := nowMS()
			res, err := tx.Exec(`INSERT INTO hosts(identity, site_id, ip, mac, hostname, vendor, model, os,
				device_type, confidence, status, is_gateway, favorite, notes, tags, ports, latency_ms, ttl,
				first_seen, last_seen, network_id, last_scan_id, snmp, evidence)
				VALUES(?,?,?,?,?,?,?,?,?,0,'unknown',0,?,?,?,'[]',0,0,?,?,?,0,'','[]')`,
				identity, siteID, bh.IP, bh.MAC, bh.Hostname, bh.Vendor, bh.Model, bh.OS,
				bh.DeviceType, boolInt(bh.Favorite), bh.Notes, jsonMarshal(nonNilStrings(bh.Tags)),
				now, now, networkID)
			if err != nil {
				return summary, err
			}
			id, _ = res.LastInsertId()
			summary.HostsCreated++
			continue
		}
		if err != nil {
			return summary, err
		}
		// Keep user metadata already present locally; imported values fill gaps.
		var hostname, vendor, model, osName, deviceType, notes, tags string
		var favorite int
		if err := tx.QueryRow(`SELECT hostname, vendor, model, os, device_type, notes, tags, favorite FROM hosts WHERE id = ?`, id).
			Scan(&hostname, &vendor, &model, &osName, &deviceType, &notes, &tags, &favorite); err != nil {
			return summary, err
		}
		localTags := map[string]bool{}
		for _, t := range jsonStringSlice(tags) {
			localTags[t] = true
		}
		for _, t := range bh.Tags {
			if t != "" {
				localTags[t] = true
			}
		}
		mergedTags := make([]string, 0, len(localTags))
		for t := range localTags {
			mergedTags = append(mergedTags, t)
		}
		sort.Strings(mergedTags)
		if bh.Hostname != "" && hostname == "" {
			hostname = bh.Hostname
		}
		if bh.Vendor != "" && vendor == "" {
			vendor = bh.Vendor
		}
		if bh.Model != "" && model == "" {
			model = bh.Model
		}
		if bh.OS != "" && osName == "" {
			osName = bh.OS
		}
		if bh.DeviceType != "" && deviceType == "" {
			deviceType = bh.DeviceType
		}
		if bh.Notes != "" && notes == "" {
			notes = bh.Notes
		}
		if bh.Favorite {
			favorite = 1
		}
		if _, err := tx.Exec(`UPDATE hosts SET site_id=?, hostname=?, vendor=?, model=?, os=?, device_type=?,
			notes=?, tags=?, favorite=?, network_id=CASE WHEN network_id=0 THEN ? ELSE network_id END WHERE id=?`,
			siteID, hostname, vendor, model, osName, deviceType, notes, jsonMarshal(mergedTags), favorite, networkID, id); err != nil {
			return summary, err
		}
		summary.HostsUpdated++
	}
	return summary, tx.Commit()
}

func matchHostNetwork(ip string, netByCIDR map[string]int64) int64 {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return 0
	}
	var best int64
	bestBits := -1
	for cidr, id := range netByCIDR {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil || !prefix.Contains(addr) {
			continue
		}
		if bits := prefix.Bits(); bits > bestBits {
			best, bestBits = id, bits
		}
	}
	return best
}

// Command netdiag is the NetDiag network diagnostic scanner.
//
// It ships as a single binary with three modes of operation:
//
//	netdiag serve    run the web UI and the periodic monitor
//	netdiag scan     discover a network from the terminal
//	netdiag report   render an inventory or alert report
//
// Run "netdiag help" for the full command list.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/kr3s4l4/netdiag/internal/api"
	"github.com/kr3s4l4/netdiag/internal/app"
	"github.com/kr3s4l4/netdiag/internal/auth"
	"github.com/kr3s4l4/netdiag/internal/backup"
	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/container"
	"github.com/kr3s4l4/netdiag/internal/demo"
	"github.com/kr3s4l4/netdiag/internal/discover"
	"github.com/kr3s4l4/netdiag/internal/license"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
	"github.com/kr3s4l4/netdiag/internal/report"
	"github.com/kr3s4l4/netdiag/internal/trace"
	"github.com/kr3s4l4/netdiag/internal/update"
	"gopkg.in/yaml.v3"
)

// Injected with -ldflags at build time.
var (
	version   = "1.5.0"
	commit    = "dev"
	buildDate = ""
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stdout)
		return 0
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "-h", "--help", "help":
		usage(os.Stdout)
		return 0
	case "-v", "--version", "version":
		fmt.Printf("netdiag %s (commit %s, %s/%s, built %s)\n",
			version, shortCommit(), runtime.Version(), runtime.GOARCH, buildDateOrUnknown())
		return 0
	case "serve":
		return serveCmd(rest)
	case "scan":
		return scanCmd(rest)
	case "trace", "traceroute":
		return traceCmd(rest)
	case "hosts", "ls":
		return hostsCmd(rest)
	case "report":
		return reportCmd(rest)
	case "config":
		return configCmd(rest)
	case "doctor":
		return doctorCmd(rest)
	case "user", "users":
		return userCmd(rest)
	case "backup":
		return backupCmd(rest)
	case "license", "licenses":
		return licenseCmd(rest)
	case "container":
		return containerCmd(rest)
	case "demo":
		return demoCmd(rest)
	case "update", "updates":
		return updateCmd(rest)
	default:
		fmt.Fprintf(os.Stderr, "netdiag: comando desconocido %q\n\n", cmd)
		usage(os.Stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `NetDiag — escáner de red y monitor de inventario

Uso:
  netdiag <comando> [opciones]

Comandos:
  serve                 Inicia la interfaz web y el monitor periódico
  scan <CIDR>           Descubre hosts y servicios en una red
  trace <destino>       Traza la ruta hasta un host (saltos y latencia)
  hosts                 Lista el inventario almacenado
  report                Genera un informe (PDF, CSV, JSON o HTML)
  config                Muestra, valida o crea la configuración
  doctor                Comprueba el entorno y los permisos necesarios
  user                  Gestiona usuarios (list, add, passwd, role, delete)
  backup                Crea una copia de seguridad completa en ZIP
  license show|activate|clear|sign  Gestiona la licencia offline
  container inspect|verify|build  Inspecciona o verifica contenedores .ndc
  demo                  Genera datos sintéticos en <data_dir>/demo
  update check|apply|rollback|commit  Gestiona actualizaciones firmadas
  version               Muestra la versión
  help                  Muestra esta ayuda

Opciones globales habituales (cada comando las acepta):
  -c, --config RUTA     Archivo YAML de configuración
      --data-dir RUTA   Directorio de datos (base de datos, informes, clave)
      --db RUTA         Ruta alternativa de la base SQLite

Ejemplos:
  sudo netdiag scan 192.168.1.0/24
  netdiag scan 10.0.0.0/24 --profile full --snmp --wait
  netdiag serve --listen 0.0.0.0:8080
  netdiag report --kind inventory --format pdf
  netdiag demo --seed 1 --sites 3 --hosts 120
  netdiag backup --out /var/backups/netdiag.zip
  netdiag doctor
`)
}

func shortCommit() string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func buildDateOrUnknown() string {
	if buildDate == "" {
		return "sin fecha"
	}
	return buildDate
}

/* ------------------------------------------------------------------ */
/* Shared flags and helpers                                            */
/* ------------------------------------------------------------------ */

type commonFlags struct {
	config    string
	dataDir   string
	db        string
	logLevel  string
	logFormat string
}

func addCommonFlags(fs *flag.FlagSet) *commonFlags {
	cf := &commonFlags{}
	fs.StringVar(&cf.config, "config", "", "archivo de configuración YAML")
	fs.StringVar(&cf.config, "c", "", "alias de --config")
	fs.StringVar(&cf.dataDir, "data-dir", "", "directorio de datos")
	fs.StringVar(&cf.db, "db", "", "ruta de la base de datos SQLite")
	fs.StringVar(&cf.logLevel, "log-level", "", "debug, info, warn o error")
	fs.StringVar(&cf.logFormat, "log-format", "", "text o json")
	return cf
}

// parseFlags accepts flags before or after positional arguments, which the
// standard flag package does not. It reorders the arguments so that the usual
// "netdiag scan 192.168.1.0/24 --profile full" style works.
func parseFlags(fs *flag.FlagSet, args []string) error {
	isBool := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			isBool[f.Name] = true
		}
	})
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				continue
			}
			if !isBool[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return fs.Parse(append(flags, positional...))
}

// loadConfig resolves the configuration applying command-line overrides.
func loadConfig(cf *commonFlags) (*config.Config, string, error) {
	path := cf.config
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, path, err
	}
	if cf.dataDir != "" {
		// If backup_dir was derived from the old data directory, keep deriving
		// it from the new one. An explicitly configured path is left untouched.
		derivedBackup := cfg.Storage.BackupDir == filepath.Join(cfg.Storage.DataDir, "backups")
		cfg.Storage.DataDir = cf.dataDir
		// Keep the database next to the new data directory unless the user
		// explicitly overrides it as well.
		if cf.db == "" {
			cfg.Storage.DB = filepath.Join(cfg.Storage.DataDir, filepath.Base(cfg.Storage.DB))
		}
		if derivedBackup {
			cfg.Storage.BackupDir = ""
		}
	}
	if cf.db != "" {
		cfg.Storage.DB = cf.db
	}
	if err := cfg.Normalize(); err != nil {
		return nil, path, err
	}
	if cf.logLevel != "" {
		cfg.Log.Level = cf.logLevel
	}
	if cf.logFormat != "" {
		cfg.Log.Format = cf.logFormat
	}
	return cfg, path, nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Log.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.EqualFold(cfg.Log.Format, "json") {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(h)
}

func fail(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "netdiag: "+format+"\n", a...)
	return 1
}

/* ------------------------------------------------------------------ */
/* serve                                                               */
/* ------------------------------------------------------------------ */

func serveCmd(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cf := addCommonFlags(fs)
	listen := fs.String("listen", "", "dirección de escucha (ej. 0.0.0.0:8080)")
	noMonitor := fs.Bool("no-monitor", false, "desactiva el monitor periódico al arrancar")
	wifiIface := fs.String("wifi-monitor-iface", "", "interfaz ya configurada en modo monitor para captura 802.11")
	wifiInterval := fs.Int("wifi-scan-interval-sec", 0, "segundos entre capturas WiFi (0 usa la configuración)")
	updateInterval := fs.Int("update-check-interval-hours", 24, "horas entre comprobaciones de actualizaciones firmadas")
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "Uso: netdiag serve [opciones]") }
	if err := parseFlags(fs, args); err != nil {
		return 2
	}
	cfg, path, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
	// The flag advertises the 24h default, but only overrides YAML when the
	// operator sets it explicitly.
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "update-check-interval-hours" {
			cfg.Update.CheckIntervalHours = *updateInterval
		}
	})
	if *listen != "" {
		cfg.Server.Listen = *listen
	}
	if *noMonitor {
		cfg.Monitor.Enabled = false
	}
	if *wifiIface != "" {
		cfg.WiFi.Enabled = true
		cfg.WiFi.Interface = *wifiIface
	}
	if *wifiInterval > 0 {
		cfg.WiFi.Interval = config.Duration(time.Duration(*wifiInterval) * time.Second)
	}
	if err := cfg.Validate(); err != nil {
		return fail("%v", err)
	}
	log := newLogger(cfg)
	slog.SetDefault(log)

	a, err := app.New(cfg, path, app.BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	}, log)
	if err != nil {
		return fail("%v", err)
	}
	defer a.Close()

	log.Info("NetDiag iniciado",
		"version", version,
		"config", path,
		"data_dir", cfg.Storage.DataDir,
		"listen", cfg.Server.Listen,
		"monitor", cfg.Monitor.Enabled,
		"wifi", cfg.WiFi.Enabled && cfg.WiFi.Interface != "",
		"arp_activo", discover.ARPAvailable(),
		"actualizaciones", cfg.Update.Enabled,
		"update_interval_h", cfg.Update.CheckIntervalHours,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := a.Run(ctx); err != nil {
		return fail("%v", err)
	}
	return 0
}

/* ------------------------------------------------------------------ */
/* scan                                                                */
/* ------------------------------------------------------------------ */

func scanCmd(args []string) int {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	cf := addCommonFlags(fs)
	profile := fs.String("profile", "", "perfil: quick, common, full o custom")
	ports := fs.String("ports", "", "puertos para el perfil custom (22,80,8000-8100)")
	network := fs.Int64("network", 0, "ID de una red configurada")
	snmpOn := fs.Bool("snmp", false, "consultar SNMP")
	deep := fs.Bool("deep", false, "enriquecimiento profundo")
	timeout := fs.Int("timeout", 0, "timeout por host en milisegundos")
	allowPublic := fs.Bool("allow-public", false, "permitir redes públicas")
	wait := fs.Bool("wait", true, "esperar al resultado del escaneo")
	progress := fs.Bool("progress", false, "mostrar progreso en stderr")
	asJSON := fs.Bool("json", false, "imprimir el resultado en JSON")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Uso: netdiag scan <CIDR> [opciones]")
		fmt.Fprintln(os.Stderr, "     netdiag scan --network ID [opciones]")
	}
	if err := parseFlags(fs, args); err != nil {
		return 2
	}

	cidr := strings.TrimSpace(fs.Arg(0))
	if cidr == "" && *network == 0 {
		fs.Usage()
		return 2
	}

	cfg, path, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
	log := newLogger(cfg)
	a, err := app.New(cfg, path, app.BuildInfo{Version: version, Commit: commit, BuildDate: buildDate}, log)
	if err != nil {
		return fail("%v", err)
	}
	defer a.Close()

	req := api.ScanRequest{
		CIDR:        cidr,
		NetworkID:   *network,
		Profile:     *profile,
		Ports:       parsePorts(*ports),
		SNMP:        boolPtr(*snmpOn),
		Deep:        *deep,
		TimeoutMS:   *timeout,
		AllowPublic: *allowPublic,
	}

	scan, err := a.StartScan(req, "cli")
	if err != nil {
		return fail("%v", err)
	}
	fmt.Printf("Escaneo #%d iniciado en %s (perfil %s)\n", scan.ID, scan.CIDR, scan.Profile)

	if !*wait {
		return 0
	}

	ticker := time.NewTicker(700 * time.Millisecond)
	defer ticker.Stop()
	var lastLine string
	for {
		cur, err := a.Store().GetScan(scan.ID)
		if err != nil {
			return fail("no se pudo leer el estado del escaneo: %v", err)
		}
		if *progress {
			line := fmt.Sprintf("%5.1f%% · %-12s %d/%d direcciones · %d hosts",
				cur.Progress, cur.Stage, cur.Scanned, cur.Total, cur.HostsFound)
			if line != lastLine {
				fmt.Fprint(os.Stderr, "\r"+line)
				lastLine = line
			}
		}
		switch cur.Status {
		case model.ScanDone, model.ScanError, model.ScanCanceled, model.ScanInterrupted:
			if *progress && lastLine != "" {
				fmt.Fprintln(os.Stderr)
			}
			return scanResult(a, cur, *asJSON)
		}
		<-ticker.C
	}
}

// scanResult prints the terminal state of a scan and returns the exit code.
func scanResult(a *app.App, scan *model.Scan, asJSON bool) int {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(scan)
	} else {
		dur := ""
		if scan.StartedAt != nil && scan.FinishedAt != nil {
			dur = " · " + scan.FinishedAt.Sub(*scan.StartedAt).Round(time.Millisecond).String()
		}
		fmt.Printf("Estado: %s · %d hosts encontrados · %d direcciones analizadas%s\n",
			statusLabel(scan.Status), scan.HostsFound, scan.Scanned, dur)
		if scan.Error != "" {
			fmt.Printf("Error: %s\n", scan.Error)
		}
	}
	if scan.Status != model.ScanDone {
		return 1
	}
	return 0
}

// traceCmd runs an unprivileged traceroute to a target.
func traceCmd(args []string) int {
	fs := flag.NewFlagSet("trace", flag.ExitOnError)
	hops := fs.Int("hops", 20, "número máximo de saltos")
	probes := fs.Int("probes", 2, "sondas por salto")
	timeout := fs.Int("timeout", 1200, "timeout por sonda en milisegundos")
	noResolve := fs.Bool("no-resolve", false, "no resolver nombres inversos")
	asJSON := fs.Bool("json", false, "salida JSON")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Uso: netdiag trace <destino> [opciones]")
		fmt.Fprintln(os.Stderr, "     netdiag trace 192.168.1.1 --hops 10 --json")
	}
	if err := parseFlags(fs, args); err != nil {
		return 2
	}
	target := strings.TrimSpace(fs.Arg(0))
	if target == "" {
		fs.Usage()
		return 2
	}
	opts := trace.Options{
		MaxHops: *hops,
		Probes:  *probes,
		Timeout: time.Duration(*timeout) * time.Millisecond,
		Resolve: !*noResolve,
	}
	res, err := trace.Run(context.Background(), target, opts)
	if err != nil {
		return fail("%v", err)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}
	fmt.Printf("Traza a %s (%s) · %d saltos · %.1f ms\n", res.Target, res.TargetIP, len(res.Hops), res.ElapsedMS)
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SALTO\tIP\tNOMBRE\tRTT\tPÉRDIDA")
	for _, h := range res.Hops {
		name := h.Hostname
		if name == "" && h.IP != "*" {
			name = "—"
		}
		var rtt string
		if len(h.RTTMs) > 0 {
			parts := make([]string, 0, len(h.RTTMs))
			for _, ms := range h.RTTMs {
				parts = append(parts, fmt.Sprintf("%.1f ms", ms))
			}
			rtt = strings.Join(parts, " / ")
		} else {
			rtt = "—"
		}
		mark := ""
		if h.Reached {
			mark = " [destino]"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s%s\t%s\t%d/%d\n", h.TTL, h.IP, name, mark, rtt, h.Loss, res.Probes)
	}
	_ = tw.Flush()
	return 0
}

func statusLabel(s string) string {
	switch s {
	case model.ScanDone:
		return "completado"
	case model.ScanRunning:
		return "en curso"
	case model.ScanQueued:
		return "en cola"
	case model.ScanCanceled:
		return "cancelado"
	case model.ScanError:
		return "error"
	case model.ScanInterrupted:
		return "interrumpido"
	}
	return s
}

func parsePorts(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var a, b int
		if n, _ := fmt.Sscanf(part, "%d-%d", &a, &b); n == 2 {
			if a < 1 || b > 65535 || a > b {
				continue
			}
			for p := a; p <= b; p++ {
				out = append(out, p)
			}
			continue
		}
		var p int
		if n, _ := fmt.Sscanf(part, "%d", &p); n == 1 && p >= 1 && p <= 65535 {
			out = append(out, p)
		}
	}
	return out
}

func boolPtr(v bool) *bool { return &v }

/* ------------------------------------------------------------------ */
/* hosts                                                               */
/* ------------------------------------------------------------------ */

func hostsCmd(args []string) int {
	fs := flag.NewFlagSet("hosts", flag.ExitOnError)
	cf := addCommonFlags(fs)
	search := fs.String("search", "", "filtrar por IP, nombre o MAC")
	status := fs.String("status", "", "filtrar por estado: up, down, unknown")
	dtype := fs.String("type", "", "filtrar por tipo de dispositivo")
	limit := fs.Int("limit", 100, "número máximo de filas")
	asJSON := fs.Bool("json", false, "salida JSON")
	asCSV := fs.Bool("csv", false, "salida CSV")
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "Uso: netdiag hosts [opciones]") }
	if err := parseFlags(fs, args); err != nil {
		return 2
	}

	cfg, path, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
	a, err := app.New(cfg, path, app.BuildInfo{Version: version, Commit: commit, BuildDate: buildDate}, newLogger(cfg))
	if err != nil {
		return fail("%v", err)
	}
	defer a.Close()

	res, err := a.Store().ListHosts(model.HostFilter{
		Search:     *search,
		Status:     *status,
		DeviceType: *dtype,
		Sort:       "ip",
		Page:       1,
		Limit:      *limit,
	})
	if err != nil {
		return fail("%v", err)
	}

	switch {
	case *asJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case *asCSV:
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{"ip", "mac", "hostname", "vendor", "tipo", "estado", "puertos", "latencia_ms", "visto"})
		for _, h := range res.Items {
			_ = w.Write([]string{
				h.IP, h.MAC, h.Hostname, h.Vendor, string(h.DeviceType), string(h.Status),
				fmt.Sprint(h.OpenPortCount), fmt.Sprintf("%.2f", h.LatencyMS),
				h.LastSeen.Format(time.RFC3339),
			})
		}
		w.Flush()
	default:
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "IP\tESTADO\tTIPO\tPUERTOS\tLATENCIA\tNOMBRE / FABRICANTE\tVISTO")
		for _, h := range res.Items {
			label := h.Hostname
			if label == "" {
				label = h.Vendor
			}
			if label == "" {
				label = "—"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%.1f ms\t%s\t%s\n",
				h.IP, h.Status, h.DeviceType, h.OpenPortCount, h.LatencyMS, label, shortAgo(h.LastSeen))
		}
		tw.Flush()
		fmt.Printf("\n%d dispositivos (total %d)\n", len(res.Items), res.Total)
	}
	return 0
}

func shortAgo(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "ahora"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("hace %d h", int(d.Hours()))
	default:
		return fmt.Sprintf("hace %d d", int(d.Hours()/24))
	}
}

/* ------------------------------------------------------------------ */
/* report                                                              */
/* ------------------------------------------------------------------ */

func reportCmd(args []string) int {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	cf := addCommonFlags(fs)
	kind := fs.String("kind", "inventory", "inventory, executive o alerts")
	format := fs.String("format", "pdf", "pdf, csv, json o html")
	title := fs.String("title", "", "título del informe")
	out := fs.String("out", "", "copia el informe a esta ruta")
	from := fs.String("from", "", "fecha inicial AAAA-MM-DD")
	to := fs.String("to", "", "fecha final AAAA-MM-DD")
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "Uso: netdiag report [opciones]") }
	if err := parseFlags(fs, args); err != nil {
		return 2
	}

	cfg, path, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
	a, err := app.New(cfg, path, app.BuildInfo{Version: version, Commit: commit, BuildDate: buildDate}, newLogger(cfg))
	if err != nil {
		return fail("%v", err)
	}
	defer a.Close()

	now := time.Now()
	toT := now
	fromT := now.Add(-7 * 24 * time.Hour)
	if *to != "" {
		parsed, err := time.ParseInLocation("2006-01-02", *to, time.Local)
		if err != nil {
			return fail("fecha --to inválida: %v", err)
		}
		toT = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	if *from != "" {
		parsed, err := time.ParseInLocation("2006-01-02", *from, time.Local)
		if err != nil {
			return fail("fecha --from inválida: %v", err)
		}
		fromT = parsed
	}

	hosts, err := a.Store().ListAllHosts(0)
	if err != nil {
		return fail("no se pudo leer el inventario: %v", err)
	}
	alerts, err := a.Store().ListAlerts(model.AlertFilter{Page: 1, Limit: 500, From: &fromT, To: &toT})
	if err != nil {
		return fail("no se pudieron leer las alertas: %v", err)
	}
	stats, err := a.Store().Dashboard(0)
	if err != nil {
		return fail("no se pudieron calcular las estadísticas: %v", err)
	}

	gen := report.New(filepath.Join(cfg.Storage.DataDir, "reports"), "NetDiag", "NetDiag")
	rec, err := gen.Generate(report.Request{
		Kind:   *kind,
		Demo:   demo.Active(cfg.Storage.DataDir),
		Format: *format,
		Title:  *title,
		Hosts:  hosts,
		Alerts: alerts.Items,
		Stats:  *stats,
		From:   fromT,
		To:     toT,
	})
	if err != nil {
		return fail("%v", err)
	}
	if _, err := a.Store().AddReport(rec); err != nil {
		return fail("informe generado pero no registrado: %v", err)
	}

	if *out != "" {
		if err := copyFile(rec.Path, *out); err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Informe guardado en %s\n", *out)
		return 0
	}
	fmt.Printf("Informe generado: %s\n", rec.Path)
	return 0
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if dir := filepath.Dir(dst); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	outFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(outFile, in); err != nil {
		outFile.Close()
		return err
	}
	return outFile.Close()
}

/* ------------------------------------------------------------------ */
/* config                                                              */
/* ------------------------------------------------------------------ */

func configCmd(args []string) int {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	cf := addCommonFlags(fs)
	force := fs.Bool("force", false, "sobrescribir al crear la configuración")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Uso: netdiag config [show|path|init|validate] [opciones]")
	}
	if err := parseFlags(fs, args); err != nil {
		return 2
	}
	sub := "show"
	if fs.NArg() > 0 {
		sub = fs.Arg(0)
	}

	path := cf.config
	if path == "" {
		path = config.DefaultPath()
	}

	switch sub {
	case "path":
		fmt.Println(path)
		cfg, err := config.Load(path)
		if err == nil {
			fmt.Printf("datos: %s\nbase:  %s\n", cfg.Storage.DataDir, cfg.Storage.DB)
		}
		return 0
	case "init":
		cfg := config.Defaults()
		if cf.dataDir != "" {
			cfg.Storage.DataDir = cf.dataDir
		}
		if cf.db != "" {
			cfg.Storage.DB = cf.db
		}
		if err := cfg.Normalize(); err != nil {
			return fail("%v", err)
		}
		if _, err := os.Stat(path); err == nil && !*force {
			return fail("el archivo %s ya existe; usa --force para sobrescribirlo", path)
		}
		if err := cfg.Save(path); err != nil {
			return fail("%v", err)
		}
		if err := cfg.EnsureDirs(); err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Configuración creada en %s\n", path)
		return 0
	case "validate":
		cfg, path, err := loadConfig(cf)
		if err != nil {
			return fail("%v", err)
		}
		if err := cfg.Validate(); err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Configuración válida: %s\n", path)
		return 0
	case "show":
		data, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fail("%v", err)
			}
			cfg, _, err := loadConfig(cf)
			if err != nil {
				return fail("%v", err)
			}
			data, err = yaml.Marshal(cfg)
			if err != nil {
				return fail("%v", err)
			}
			fmt.Printf("# %s no existe; se muestran los valores por defecto\n", path)
		}
		os.Stdout.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			fmt.Println()
		}
		return 0
	default:
		fs.Usage()
		return 2
	}
}

/* ------------------------------------------------------------------ */
/* doctor                                                              */
/* ------------------------------------------------------------------ */

func doctorCmd(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	cf := addCommonFlags(fs)
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "Uso: netdiag doctor [opciones]") }
	if err := parseFlags(fs, args); err != nil {
		return 2
	}

	fmt.Println("NetDiag · diagnóstico del entorno")
	fmt.Println("──────────────────────────────────")
	okAll := true
	check := func(name string, ok bool, detail string) {
		mark := "OK "
		if !ok {
			mark = "!! "
			okAll = false
		}
		fmt.Printf("%s %-26s %s\n", mark, name, detail)
	}

	path := cf.config
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, _, cfgErr := loadConfig(cf)
	check("configuración", cfgErr == nil, configDetail(path, cfgErr))
	if cfgErr != nil {
		fmt.Println("\nNo se puede continuar sin una configuración válida.")
		return 1
	}

	check("directorio de datos", writable(cfg.Storage.DataDir), cfg.Storage.DataDir)
	check("base de datos", checkDB(cfg), cfg.Storage.DB)
	check("ICMP", checkICMP(), "ping sin privilegios o con capabilities")
	check("ARP activo (pcap)", discover.ARPAvailable(), arpDetail())
	check("DHCP activo (UDP/68)", discover.DHCPAvailable(), "sin raw sockets; SO_REUSEADDR en la IP local")
	check("privilegios", os.Geteuid() == 0, privilegeDetail())
	check("monitor", cfg.Monitor.Enabled, "intervalo "+cfg.Monitor.Interval.Duration().String())
	check("alertas", cfg.Alerts.Enabled, fmt.Sprintf("máximo %d/min", cfg.Alerts.MaxPerMinute))

	ifaces, err := netutil.Interfaces()
	if err == nil {
		fmt.Println("\nInterfaces detectadas:")
		for _, i := range ifaces {
			var addrs []string
			for _, a := range i.Addrs {
				addrs = append(addrs, a.CIDR)
			}
			gw := ""
			if g := netutil.GatewayFor(i.Name, ifaces); g != "" {
				gw = " · gw " + g
			}
			fmt.Printf("  %-12s %-4s %s%s\n", i.Name, upDown(i.Up), strings.Join(addrs, ", "), gw)
		}
	}

	suggested, err := netutil.SuggestNetworks()
	if err == nil && len(suggested) > 0 {
		fmt.Println("\nRedes candidatas para monitorizar:")
		for _, n := range suggested {
			fmt.Printf("  %s  (%s)\n", n.CIDR, n.Interface)
		}
	}

	if okAll {
		fmt.Println("\nTodo listo.")
		return 0
	}
	fmt.Println("\nHay comprobaciones pendientes; revisa los avisos anteriores.")
	return 1
}

func configDetail(path string, err error) string {
	if err != nil {
		return err.Error()
	}
	if _, statErr := os.Stat(path); statErr == nil {
		return path
	}
	return path + " (valores por defecto)"
}

func writable(dir string) bool {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".netdiag-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return true
}

func checkDB(cfg *config.Config) bool {
	f, err := os.OpenFile(cfg.Storage.DB, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func checkICMP() bool {
	ic, err := netutil.OpenICMP()
	if err != nil {
		return false
	}
	defer ic.Close()
	return true
}

func arpDetail() string {
	if discover.ARPAvailable() {
		return "compilado; requiere CAP_NET_RAW"
	}
	return "no compilado (build portable; usa -tags pcap)"
}

func privilegeDetail() string {
	if os.Geteuid() == 0 {
		return "root"
	}
	return "sin root (revisa capabilities o ping_group_range)"
}

func upDown(up bool) string {
	if up {
		return "up"
	}
	return "down"
}

/* ------------------------------------------------------------------ */
/* user                                                                */
/* ------------------------------------------------------------------ */

func userCmd(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(os.Stderr, `Uso: netdiag user <subcomando> [opciones]

Subcomandos:
  list                     Lista los usuarios
  add <usuario> [--role R] Crea un usuario (contraseña por stdin o --password)
  passwd <usuario>         Cambia la contraseña de un usuario
  role <usuario> <rol>     Cambia el rol: admin, operator o viewer
  delete <usuario>         Elimina un usuario y sus sesiones`)
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("user "+sub, flag.ExitOnError)
	cf := addCommonFlags(fs)
	role := fs.String("role", "viewer", "rol: admin, operator o viewer")
	password := fs.String("password", "", "contraseña (si se omite, se lee de stdin)")
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "Uso: netdiag user %s [opciones] [usuario]\n", sub) }
	if err := parseFlags(fs, args[1:]); err != nil {
		return 2
	}

	cfg, path, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
	a, err := app.New(cfg, path, app.BuildInfo{Version: version, Commit: commit, BuildDate: buildDate}, newLogger(cfg))
	if err != nil {
		return fail("%v", err)
	}
	defer a.Close()

	switch sub {
	case "list":
		users, err := a.Store().ListUsers()
		if err != nil {
			return fail("%v", err)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tUSUARIO\tROL\tCREADO\tÚLTIMO ACCESO")
		for _, u := range users {
			last := "—"
			if u.LastLoginAt != nil {
				last = u.LastLoginAt.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", u.ID, u.Username, u.Role, u.CreatedAt.Format("2006-01-02"), last)
		}
		tw.Flush()
		return 0
	case "add":
		username := fs.Arg(0)
		if username == "" {
			return fail("indica el nombre de usuario")
		}
		pw, err := passwordValue(*password)
		if err != nil {
			return fail("%v", err)
		}
		u, err := a.Auth().CreateUser(username, pw, *role)
		if err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Usuario %s creado con rol %s (id %d)\n", u.Username, u.Role, u.ID)
		return 0
	case "passwd":
		username := fs.Arg(0)
		if username == "" {
			return fail("indica el nombre de usuario")
		}
		u, _, err := a.Store().GetUserWithHash(username)
		if err != nil {
			return fail("usuario no encontrado: %v", err)
		}
		pw, err := passwordValue(*password)
		if err != nil {
			return fail("%v", err)
		}
		if err := auth.ValidatePassword(pw); err != nil {
			return fail("%v", err)
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			return fail("%v", err)
		}
		if err := a.Store().UpdatePassword(u.ID, hash); err != nil {
			return fail("%v", err)
		}
		_ = a.Store().DeleteUserSessions(u.ID)
		fmt.Printf("Contraseña actualizada para %s; sesiones cerradas\n", u.Username)
		return 0
	case "role":
		username := fs.Arg(0)
		newRole := fs.Arg(1)
		if username == "" || newRole == "" {
			return fail("uso: netdiag user role <usuario> <admin|operator|viewer>")
		}
		u, _, err := a.Store().GetUserWithHash(username)
		if err != nil {
			return fail("usuario no encontrado: %v", err)
		}
		updated, err := a.Auth().UpdateUserRole(u.ID, newRole)
		if err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Rol de %s actualizado a %s; sesiones cerradas\n", updated.Username, updated.Role)
		return 0
	case "delete":
		username := fs.Arg(0)
		if username == "" {
			return fail("indica el nombre de usuario")
		}
		u, _, err := a.Store().GetUserWithHash(username)
		if err != nil {
			return fail("usuario no encontrado: %v", err)
		}
		if err := a.Auth().DeleteUser(u.ID); err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Usuario %s eliminado\n", u.Username)
		return 0
	default:
		fs.Usage()
		return 2
	}
}

func licenseCmd(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(os.Stderr, `Uso: netdiag license <subcomando> [opciones]

Subcomandos:
  show                     Muestra la licencia activa o el estado Community
  activate <archivo>      Verifica e instala una licencia firmada
  clear                   Elimina la licencia local (no borra datos)
  sign <archivo>          Firma una licencia con la clave del emisor

Opciones:
  --json                  Salida JSON en "show"
  --seed-file RUTA        Semilla Ed25519 para "sign"
  --out RUTA              Archivo de salida para "sign"`)
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("license "+sub, flag.ExitOnError)
	cf := addCommonFlags(fs)
	jsonOut := fs.Bool("json", false, "muestra la licencia en JSON")
	home, _ := os.UserHomeDir()
	seedPath := fs.String("seed-file", filepath.Join(home, ".config", "netdiag", "license-signing.seed"), "archivo con la semilla del emisor (solo sign)")
	outPath := fs.String("out", "", "archivo de salida (solo sign)")
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "Uso: netdiag license %s [opciones]\n", sub) }
	if err := parseFlags(fs, args[1:]); err != nil {
		return 2
	}
	cfg, _, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		return fail("%v", err)
	}
	lic, err := license.New(cfg.Storage.DataDir)
	if err != nil {
		return fail("%v", err)
	}

	switch sub {
	case "show":
		l := lic.Current()
		if *jsonOut {
			if l == nil {
				printUpdateJSON(map[string]any{
					"status": "community", "edition": "community", "problem": lic.Problem(),
					"path": lic.Path(),
				})
				return 0
			}
			printUpdateJSON(l)
			return 0
		}
		fmt.Printf("Archivo: %s\n", lic.Path())
		if l == nil {
			if problem := lic.Problem(); problem != "" {
				fmt.Printf("Estado: inválida\nMotivo: %s\n", problem)
				fmt.Println("Se aplican capacidades Community; los datos se conservan.")
				return 0
			}
			fmt.Println("Estado: Community (sin licencia instalada)")
			return 0
		}
		expiry := l.ExpiresAt
		if t, err := time.Parse(time.RFC3339, l.ExpiresAt); err == nil {
			expiry = t.Format("2006-01-02")
		}
		state := "activa"
		if t, err := time.Parse(time.RFC3339, l.ExpiresAt); err == nil && time.Now().After(t) {
			state = "caducada"
		}
		fmt.Printf("Estado: %s\n", state)
		fmt.Printf("Edición: %s\n", l.Edition)
		fmt.Printf("ID: %s\n", l.LicenseID)
		fmt.Printf("Cliente: %s\n", l.Customer)
		fmt.Printf("Caducidad: %s\n", expiry)
		fmt.Printf("Límites: %d hosts gestionados, %d sedes, %d días de retención\n", l.MaxManagedHosts, l.MaxSites, l.RetentionDays)
		fmt.Printf("Soporte: %s\n", l.SupportLevel)
		if len(l.Capabilities) > 0 {
			fmt.Printf("Capacidades: %s\n", strings.Join(l.Capabilities, ", "))
		}
		if problem := lic.Problem(); problem != "" {
			fmt.Printf("Aviso: %s\n", problem)
		}
		return 0
	case "activate":
		src := fs.Arg(0)
		if src == "" {
			return fail("indica el archivo de licencia")
		}
		activated, err := lic.Activate(src)
		if err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Licencia %s activada para %s (edición %s)\n", activated.LicenseID, activated.Customer, activated.Edition)
		return 0
	case "clear":
		if err := lic.Clear(); err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Licencia eliminada de %s; se aplica Community. Los datos se conservan.\n", lic.Path())
		return 0
	case "sign":
		src := fs.Arg(0)
		if src == "" {
			return fail("indica el archivo de licencia a firmar")
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			return fail("%v", err)
		}
		var l license.License
		if err := json.Unmarshal(raw, &l); err != nil {
			return fail("la licencia no es JSON válido: %v", err)
		}
		seed, err := os.ReadFile(*seedPath)
		if err != nil {
			return fail("no se pudo leer la semilla del emisor: %v", err)
		}
		if err := license.Sign(&l, string(seed)); err != nil {
			return fail("%v", err)
		}
		signed, err := json.MarshalIndent(l, "", "  ")
		if err != nil {
			return fail("%v", err)
		}
		dest := *outPath
		if dest == "" {
			dest = src
		}
		if err := os.WriteFile(dest, append(signed, '\n'), 0o600); err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Licencia %s firmada en %s\n", l.LicenseID, dest)
		return 0
	default:
		fs.Usage()
		return 2
	}
}

func containerKeyArgument(value string) string {
	if value == "" {
		return ""
	}
	if info, err := os.Stat(value); err == nil && !info.IsDir() && info.Size() <= 1<<16 {
		if raw, err := os.ReadFile(value); err == nil {
			return string(raw)
		}
	}
	return value
}

func loadMinisignMaterial(path, keyIDFlag string) (seed, keyID []byte, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("no se pudo leer la clave de firma: %w", err)
	}
	var material struct {
		Seed  string `json:"seed"`
		KeyID string `json:"key_id"`
	}
	encoded := strings.TrimSpace(string(raw))
	if jsonErr := json.Unmarshal(raw, &material); jsonErr != nil {
		material.Seed = encoded // compatibilidad: fichero con solo la semilla en base64
	}
	seed, err = base64.StdEncoding.DecodeString(strings.TrimSpace(material.Seed))
	if err != nil || len(seed) != 32 {
		return nil, nil, errors.New("la semilla Ed25519 debe estar en base64 y tener 32 bytes")
	}
	id := material.KeyID
	if keyIDFlag != "" {
		id = keyIDFlag
	}
	keyID, err = hex.DecodeString(strings.TrimSpace(id))
	if err != nil || len(keyID) != 8 {
		return nil, nil, errors.New("el id de clave minisign debe tener 16 caracteres hexadecimales")
	}
	return seed, keyID, nil
}

func printContainerReport(rep *container.Report, verified bool, problem string) {
	fmt.Printf("Archivo: %s\n", rep.File)
	fmt.Printf("Esquema: %s\n", rep.Schema)
	fmt.Printf("Versión: %s\n", rep.Version)
	fmt.Printf("Tipo: %s\n", rep.Kind)
	fmt.Printf("Creado: %s\n", rep.CreatedAt)
	fmt.Printf("Productor: %s\n", rep.Producer)
	fmt.Printf("Resumen de contenido: %s\n", rep.PayloadSHA256)
	fmt.Println("Ficheros:")
	for _, f := range rep.Files {
		mark := "NO"
		if f.Matches {
			mark = "OK"
		}
		fmt.Printf("  %s  payload/%s  (%d bytes)\n", mark, f.Path, f.Size)
	}
	switch rep.Signature.Status {
	case "valid":
		fmt.Println("Firma: válida")
	case "invalid":
		fmt.Println("Firma: NO VÁLIDA")
	case "absent":
		fmt.Println("Firma: ausente")
	default:
		fmt.Println("Firma: no verificada")
	}
	if rep.Signature.Comment != "" {
		fmt.Printf("Comentario firmado: %s\n", rep.Signature.Comment)
	}
	if problem != "" {
		fmt.Printf("Motivo: %s\n", problem)
	}
	if rep.ContentOK {
		fmt.Println("Contenido: correcto")
	} else {
		fmt.Println("Contenido: NO COINCIDE con el manifiesto")
	}
	if verified {
		if rep.Valid {
			fmt.Println("Resultado: VÁLIDO")
		} else {
			fmt.Println("Resultado: NO VÁLIDO")
		}
	}
	for _, warning := range rep.Warnings {
		fmt.Printf("Aviso: %s\n", warning)
	}
}

func demoCmd(args []string) int {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	seed := fs.Int64("seed", 1, "semilla determinista")
	sites := fs.Int("sites", 3, "número de sedes sintéticas (1-20)")
	hosts := fs.Int("hosts", 120, "número total de hosts sintéticos (1-5000)")
	dataDir := fs.String("data-dir", "", "directorio base de datos; se creará <data_dir>/demo")
	asJSON := fs.Bool("json", false, "salida JSON")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Uso: netdiag demo [opciones]

Genera un dataset sintético en <data_dir>/demo/netdiag.db. No escanea la red,
no abre sockets y no toca la base de datos real.

Opciones:
  --seed N        Semilla determinista (por defecto 1)
  --sites N       Sedes sintéticas (por defecto 3)
  --hosts N       Hosts totales (por defecto 120)
  --data-dir RUTA Directorio base (por defecto el.data_dir de la configuración)
  --json          Salida JSON`)
	}
	if err := parseFlags(fs, args); err != nil {
		return 2
	}
	res, err := demo.Generate(demo.Options{
		Seed: *seed, Sites: *sites, Hosts: *hosts, DataDir: *dataDir,
	})
	if err != nil {
		return fail("%v", err)
	}
	if *asJSON {
		printUpdateJSON(res)
		return 0
	}
	fmt.Println("Dataset demo creado. Datos sintéticos: no se ha escaneado ninguna red.")
	fmt.Printf("  Base:            %s\n", res.DBPath)
	fmt.Printf("  Marcador:        %s\n", filepath.Join(res.DemoDir, demo.MarkerName))
	fmt.Printf("  Sedes / hosts:   %d / %d\n", res.Sites, res.Hosts)
	fmt.Printf("  Alertas / metrics: %d / %d\n", res.Alerts, res.Metrics)
	fmt.Printf("  Hosts gestionados (30 días): %d\n", res.ManagedHosts)
	fmt.Printf("\nPara verlo:\n  netdiag serve --data-dir %s\n", res.DemoDir)
	return 0
}

func containerCmd(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(os.Stderr, `Uso: netdiag container <subcomando> [opciones] <archivo.ndc>

Subcomandos:
  inspect <archivo>       Muestra el manifiesto y comprueba los sha256 sin verificar la firma
  verify --key <pub> <archivo>  Verifica la firma minisign, el comentario y el contenido
  build --version X --out salida.ndc --seed-file clave.json ficheros...  Crea un .ndc

Opciones:
  --key RUTA|BASE64       Clave pública minisign (archivo .pub o base64)
  --json                  Salida JSON en inspect y verify
  --version X             Versión del contenedor (build)
  --kind TIPO             release, evidence o site (build; por defecto evidence)
  --producer NOMBRE       Productor (build; por defecto netdiag)
  --sbom RUTA             Añade sbom.cdx.json (build)
  --seed-file RUTA        JSON con {"seed":"base64","key_id":"hex16"} (build)
  --key-id HEX16          Sobrescribe key_id del JSON (build)
  --created-at RFC3339    Fecha del manifiesto (build; por defecto, ahora)
  --out RUTA              Archivo .ndc de salida (build)`)
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("container "+sub, flag.ExitOnError)
	keyFlag := fs.String("key", "", "clave pública minisign (archivo .pub o base64)")
	jsonOut := fs.Bool("json", false, "salida JSON")
	versionFlag := fs.String("version", "", "versión del contenedor (build)")
	kindFlag := fs.String("kind", "evidence", "release, evidence o site (build)")
	producerFlag := fs.String("producer", "netdiag", "productor (build)")
	seedFlag := fs.String("seed-file", "", "JSON con seed y key_id del emisor (build)")
	keyIDFlag := fs.String("key-id", "", "id de clave minisign en hexadecimal (build)")
	sbomFlag := fs.String("sbom", "", "ruta a sbom.cdx.json (build)")
	createdFlag := fs.String("created-at", "", "fecha RFC3339 del manifiesto (build)")
	outFlag := fs.String("out", "", "archivo .ndc de salida (build)")
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "Uso: netdiag container %s [opciones]\n", sub) }
	if err := parseFlags(fs, args[1:]); err != nil {
		return 2
	}

	switch sub {
	case "inspect", "verify":
		file := fs.Arg(0)
		if file == "" {
			return fail("indica el archivo .ndc")
		}
		var rep *container.Report
		var err error
		verified := sub == "verify"
		if verified {
			if *keyFlag == "" {
				return fail("indica la clave pública con --key")
			}
			rep, err = container.Verify(file, containerKeyArgument(*keyFlag))
		} else {
			rep, err = container.Inspect(file)
		}
		if err != nil {
			return fail("%v", err)
		}
		if *jsonOut {
			printUpdateJSON(rep)
		} else {
			problem := rep.Signature.Problem
			printContainerReport(rep, verified, problem)
		}
		if verified && !rep.Valid {
			return 1
		}
		return 0
	case "build":
		if strings.TrimSpace(*versionFlag) == "" {
			return fail("indica la versión con --version")
		}
		if strings.TrimSpace(*outFlag) == "" {
			return fail("indica el archivo de salida con --out")
		}
		if strings.TrimSpace(*seedFlag) == "" {
			return fail("indica la clave de firma con --seed-file")
		}
		seed, keyID, err := loadMinisignMaterial(*seedFlag, *keyIDFlag)
		if err != nil {
			return fail("%v", err)
		}
		paths := fs.Args()
		if len(paths) == 0 {
			return fail("indica al menos un fichero para el contenedor")
		}
		inputs := make([]container.InputFile, 0, len(paths))
		for _, p := range paths {
			inputs = append(inputs, container.InputFile{Name: filepath.Base(p), Path: p})
		}
		var sbom []byte
		if *sbomFlag != "" {
			sbom, err = os.ReadFile(*sbomFlag)
			if err != nil {
				return fail("%v", err)
			}
		}
		var created time.Time
		if *createdFlag != "" {
			created, err = time.Parse(time.RFC3339, *createdFlag)
			if err != nil {
				return fail("created-at no válido: %v", err)
			}
		}
		if err := container.Build(*outFlag, container.BuildOptions{
			Version:   strings.TrimSpace(*versionFlag),
			Kind:      strings.TrimSpace(*kindFlag),
			Producer:  strings.TrimSpace(*producerFlag),
			Files:     inputs,
			SBOM:      sbom,
			Seed:      seed,
			KeyID:     keyID,
			CreatedAt: created,
		}); err != nil {
			return fail("%v", err)
		}
		info, err := os.Stat(*outFlag)
		if err != nil {
			return fail("%v", err)
		}
		fmt.Printf("Contenedor %s creado: %s, %d ficheros, %d bytes\n", *versionFlag, *outFlag, len(inputs), info.Size())
		return 0
	default:
		fs.Usage()
		return 2
	}
}

func passwordValue(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if env := os.Getenv("NETDIAG_PASSWORD"); env != "" {
		return env, nil
	}
	fmt.Fprint(os.Stderr, "Contraseña: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	pw := strings.TrimRight(line, "\r\n")
	if pw == "" {
		return "", errors.New("contraseña vacía")
	}
	return pw, nil
}

/* ------------------------------------------------------------------ */
/* backup                                                              */
/* ------------------------------------------------------------------ */

/* ------------------------------------------------------------------ */
/* update (F23)                                                        */
/* ------------------------------------------------------------------ */

func updateCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Uso: netdiag update check|apply|rollback|commit [opciones]")
		return 2
	}
	sub := args[0]
	fs := flag.NewFlagSet("update "+sub, flag.ExitOnError)
	cf := addCommonFlags(fs)
	wantVersion := fs.String("version", "", "versión esperada del manifiesto firmado (apply)")
	restart := fs.Bool("restart", false, "reinicia tras instalar (apply)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Uso: netdiag update <accion> [opciones]

Acciones:
  check     Comprueba el manifiesto firmado sin instalar nada
  apply     Descarga, verifica e instala; usa --restart para reiniciar
  rollback  Restaura el binario anterior usando el marcador
  commit    Confirma o revierte una actualización pendiente al arrancar`)
	}
	if err := parseFlags(fs, args[1:]); err != nil {
		return 2
	}
	cfg, _, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
	log := newLogger(cfg)
	m, err := update.New(update.Options{
		Config:         cfg.Update,
		CurrentVersion: version,
		DataDir:        cfg.Storage.DataDir,
		Log:            log,
	})
	if err != nil {
		return fail("%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	switch sub {
	case "check":
		res, err := m.Check(ctx)
		if err != nil {
			return fail("comprobación fallida: %v", err)
		}
		printUpdateJSON(res)
		return 0
	case "apply":
		res, err := m.Apply(ctx, *wantVersion)
		if err != nil {
			return fail("instalación fallida: %v", err)
		}
		printUpdateJSON(res)
		if *restart {
			if err := m.Restart(ctx); err != nil {
				return fail("no se pudo reiniciar: %v", err)
			}
		}
		return 0
	case "rollback":
		res, err := m.Rollback()
		if err != nil {
			return fail("rollback fallido: %v", err)
		}
		printUpdateJSON(res)
		return 0
	case "commit":
		res, err := m.CommitPending()
		if err != nil {
			return fail("confirmación fallida: %v", err)
		}
		printUpdateJSON(res)
		return 0
	default:
		fs.Usage()
		return 2
	}
}

func printUpdateJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(v)
		return
	}
	fmt.Println(string(data))
}

func backupCmd(args []string) int {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	cf := addCommonFlags(fs)
	out := fs.String("out", "", "ruta del ZIP de salida (por defecto, el directorio de copias)")
	keep := fs.Int("keep", 0, "número de copias a conservar (por defecto, el de la configuración)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Uso: netdiag backup [opciones]

Crea un ZIP con la base de datos, config.yaml y secret.key. Sin --out la
copia se guarda en storage.backup_dir y se aplica la retención configurada.`)
	}
	if err := parseFlags(fs, args); err != nil {
		return 2
	}
	cfg, path, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
	if *keep > 0 {
		cfg.Storage.BackupKeep = *keep
	}
	a, err := app.New(cfg, path, app.BuildInfo{Version: version, Commit: commit, BuildDate: buildDate}, newLogger(cfg))
	if err != nil {
		return fail("%v", err)
	}
	defer a.Close()

	mgr := a.Backups()
	var entry *backup.Entry
	if *out != "" {
		entry, err = mgr.CreateTo(*out)
	} else {
		entry, err = mgr.Create()
	}
	if err != nil {
		return fail("no se pudo crear la copia: %v", err)
	}
	fmt.Printf("Copia creada: %s (%s)\n", entry.Name, humanBytes(entry.Size))
	return 0
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

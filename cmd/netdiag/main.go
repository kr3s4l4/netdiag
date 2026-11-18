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
	"encoding/csv"
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
	"github.com/kr3s4l4/netdiag/internal/discover"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
	"github.com/kr3s4l4/netdiag/internal/report"
	"gopkg.in/yaml.v3"
)

// Injected with -ldflags at build time.
var (
	version   = "1.4.0"
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
  hosts                 Lista el inventario almacenado
  report                Genera un informe (PDF, CSV, JSON o HTML)
  config                Muestra, valida o crea la configuración
  doctor                Comprueba el entorno y los permisos necesarios
  user                  Gestiona usuarios (list, add, passwd, role, delete)
  backup                Crea una copia de seguridad completa en ZIP
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
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "Uso: netdiag serve [opciones]") }
	if err := parseFlags(fs, args); err != nil {
		return 2
	}

	cfg, path, err := loadConfig(cf)
	if err != nil {
		return fail("%v", err)
	}
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

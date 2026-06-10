package main

import (
	"bytes"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed templates static
var embeddedFiles embed.FS

// ──────────────────────────────────────────────
// Configuration
// ──────────────────────────────────────────────

type Config struct {
	ProxmoxURL     string
	ProxmoxToken   string
	Port           string
	NotifyURL      string
	TelegramToken  string
	TelegramChatID string
	WhatsAppURL    string
	SMSURL         string
}

func loadConfig() Config {
	return Config{
		ProxmoxURL:     getEnv("PROXMOX_URL", ""),
		ProxmoxToken:   getEnv("PROXMOX_TOKEN", ""),
		Port:           getEnv("PORT", "8080"),
		NotifyURL:      getEnv("NOTIFICATION_SERVICE_URL", ""),
		TelegramToken:  getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID: getEnv("TELEGRAM_CHAT_ID", ""),
		WhatsAppURL:    getEnv("WHATSAPP_WEBHOOK_URL", ""),
		SMSURL:         getEnv("SMS_WEBHOOK_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ──────────────────────────────────────────────
// Proxmox data types
// ──────────────────────────────────────────────

type ProxmoxResource struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Status  string  `json:"status"`
	Node    string  `json:"node"`
	CPU     float64 `json:"cpu"`
	Mem     int64   `json:"mem"`
	MaxMem  int64   `json:"maxmem"`
	Disk    int64   `json:"disk"`
	MaxDisk int64   `json:"maxdisk"`
	Uptime  int64   `json:"uptime"`
	VMID    int     `json:"vmid"`
	Tags    string  `json:"tags"`
}

type ProxmoxResponse struct {
	Data []ProxmoxResource `json:"data"`
}

// ──────────────────────────────────────────────
// Notification types
// ──────────────────────────────────────────────

type NotifyRequest struct {
	Service  string   `json:"service"`
	Status   string   `json:"status"`
	Message  string   `json:"message"`
	Channels []string `json:"channels"`
}

type NotifyResponse struct {
	Success bool              `json:"success"`
	Results map[string]string `json:"results"`
}

// ──────────────────────────────────────────────
// Template data
// ──────────────────────────────────────────────

type DashboardStats struct {
	Total   int
	Running int
	Stopped int
	VMs     int
	LXCs    int
}

type TemplateConfig struct {
	HasTelegram bool
	HasWhatsApp bool
	HasSMS      bool
	HasWebhook  bool
}

type TemplateData struct {
	Resources   []ProxmoxResource
	Stats       DashboardStats
	Config      TemplateConfig
	Error       string
	LastRefresh string
}

// ──────────────────────────────────────────────
// Server
// ──────────────────────────────────────────────

type Server struct {
	cfg        Config
	httpClient *http.Client
	tmpl       *template.Template
	mu         sync.RWMutex
	cache      []ProxmoxResource
	cacheTime  time.Time
}

func NewServer(cfg Config) (*Server, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	funcMap := template.FuncMap{
		"formatBytes":  formatBytes,
		"formatUptime": formatUptime,
		"pct":          pct,
		"cpuPct":       func(v float64) string { return fmt.Sprintf("%.1f", v*100) },
		"statusClass": func(status string) string {
			if status == "running" {
				return "text-emerald-400"
			}
			return "text-red-400"
		},
		"statusDot": func(status string) string {
			if status == "running" {
				return "bg-emerald-500"
			}
			return "bg-red-500"
		},
		"typeLabel": func(t string) string {
			if t == "qemu" {
				return "VM"
			}
			return "LXC"
		},
		"typeBadge": func(t string) string {
			if t == "qemu" {
				return "bg-violet-500/20 text-violet-300 border-violet-500/30"
			}
			return "bg-sky-500/20 text-sky-300 border-sky-500/30"
		},
		"barColor": func(pct float64) string {
			switch {
			case pct >= 90:
				return "bg-red-500"
			case pct >= 75:
				return "bg-amber-500"
			default:
				return "bg-sky-500"
			}
		},
		// cpuColor takes a raw CPU fraction (0.0–1.0) and returns the bar color class.
		"cpuColor": func(cpu float64) string {
			p := cpu * 100
			switch {
			case p >= 90:
				return "bg-red-500"
			case p >= 75:
				return "bg-amber-500"
			default:
				return "bg-sky-500"
			}
		},
		"not": func(v bool) bool { return !v },
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(embeddedFiles, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return &Server{
		cfg:        cfg,
		httpClient: client,
		tmpl:       tmpl,
	}, nil
}

func (s *Server) Routes() http.Handler {
	staticFS, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/icons/icon-192.png", func(w http.ResponseWriter, r *http.Request) { serveIcon(w, 192) })
	mux.HandleFunc("/icons/icon-512.png", func(w http.ResponseWriter, r *http.Request) { serveIcon(w, 512) })
	mux.HandleFunc("/api/resources", s.resourcesHandler)
	mux.HandleFunc("/api/notify", s.notifyHandler)
	mux.HandleFunc("/", s.indexHandler)
	return mux
}

// ──────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────

func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	resources, err := s.fetchProxmox()
	data := TemplateData{
		LastRefresh: time.Now().Format("15:04:05"),
		Config: TemplateConfig{
			HasTelegram: s.cfg.TelegramToken != "" && s.cfg.TelegramChatID != "",
			HasWhatsApp: s.cfg.WhatsAppURL != "",
			HasSMS:      s.cfg.SMSURL != "",
			HasWebhook:  s.cfg.NotifyURL != "",
		},
	}
	if err != nil {
		data.Error = err.Error()
	} else {
		data.Resources = resources
		for _, res := range resources {
			data.Stats.Total++
			if res.Status == "running" {
				data.Stats.Running++
			} else {
				data.Stats.Stopped++
			}
			if res.Type == "qemu" {
				data.Stats.VMs++
			} else {
				data.Stats.LXCs++
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := s.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (s *Server) resourcesHandler(w http.ResponseWriter, r *http.Request) {
	resources, err := s.fetchProxmox()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(resources)
}

func (s *Server) notifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NotifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		req.Message = fmt.Sprintf("Service '%s' — status: %s", req.Service, req.Status)
	}

	results := make(map[string]string)
	for _, ch := range req.Channels {
		var err error
		switch strings.ToLower(ch) {
		case "webhook":
			err = s.sendWebhook(req)
		case "telegram":
			err = s.sendTelegram(req)
		case "whatsapp":
			err = s.sendToURL(s.cfg.WhatsAppURL, "WHATSAPP_WEBHOOK_URL", req)
		case "sms":
			err = s.sendToURL(s.cfg.SMSURL, "SMS_WEBHOOK_URL", req)
		default:
			err = fmt.Errorf("unknown channel")
		}
		if err != nil {
			results[ch] = "error: " + err.Error()
		} else {
			results[ch] = "ok"
		}
	}

	allOK := true
	for _, v := range results {
		if strings.HasPrefix(v, "error") {
			allOK = false
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(NotifyResponse{Success: allOK, Results: results})
}

// ──────────────────────────────────────────────
// Proxmox client
// ──────────────────────────────────────────────

func (s *Server) fetchProxmox() ([]ProxmoxResource, error) {
	// Return demo data when Proxmox is not configured
	if s.cfg.ProxmoxURL == "" || s.cfg.ProxmoxToken == "" {
		return demoData(), nil
	}

	// Serve from cache if fresh (< 15 s)
	s.mu.RLock()
	if time.Since(s.cacheTime) < 15*time.Second && s.cache != nil {
		data := s.cache
		s.mu.RUnlock()
		return data, nil
	}
	s.mu.RUnlock()

	url := strings.TrimRight(s.cfg.ProxmoxURL, "/") + "/api2/json/cluster/resources?type=vm"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "PVEAPIToken="+s.cfg.ProxmoxToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxmox request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxmox returned HTTP %d", resp.StatusCode)
	}

	var proxResp ProxmoxResponse
	if err := json.NewDecoder(resp.Body).Decode(&proxResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	s.mu.Lock()
	s.cache = proxResp.Data
	s.cacheTime = time.Now()
	s.mu.Unlock()

	return proxResp.Data, nil
}

// ──────────────────────────────────────────────
// Notification senders
// ──────────────────────────────────────────────

func (s *Server) sendWebhook(req NotifyRequest) error {
	if s.cfg.NotifyURL == "" {
		return fmt.Errorf("NOTIFICATION_SERVICE_URL not set")
	}
	return s.postJSON(s.cfg.NotifyURL, req)
}

func (s *Server) sendTelegram(req NotifyRequest) error {
	if s.cfg.TelegramToken == "" || s.cfg.TelegramChatID == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID not set")
	}
	text := fmt.Sprintf(
		"🏠 *Homelab Alert*\n\n*Service:* `%s`\n*Status:* `%s`\n*Message:* %s\n*Time:* %s",
		req.Service, req.Status, req.Message,
		time.Now().Format("2006-01-02 15:04:05"),
	)
	payload := map[string]interface{}{
		"chat_id":    s.cfg.TelegramChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	url := "https://api.telegram.org/bot" + s.cfg.TelegramToken + "/sendMessage"
	b, _ := json.Marshal(payload)
	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *Server) sendToURL(targetURL, envKey string, req NotifyRequest) error {
	if targetURL == "" {
		return fmt.Errorf("%s not set", envKey)
	}
	return s.postJSON(targetURL, req)
}

func (s *Server) postJSON(url string, body interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// ──────────────────────────────────────────────
// Icon generation (single-binary, no assets)
// ──────────────────────────────────────────────

func serveIcon(w http.ResponseWriter, size int) {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	bg := color.NRGBA{R: 15, G: 23, B: 42, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	accent := color.NRGBA{R: 56, G: 189, B: 248, A: 255}
	hi := color.NRGBA{R: 14, G: 165, B: 233, A: 255}
	green := color.NRGBA{R: 52, G: 211, B: 153, A: 255}
	red := color.NRGBA{R: 248, G: 113, B: 113, A: 255}

	cx := size / 2
	// Roof: triangle via filled trapezoid rows
	roofTop := size * 15 / 100
	roofBase := size * 45 / 100
	roofH := roofBase - roofTop
	for y := roofTop; y <= roofBase; y++ {
		progress := float64(y-roofTop) / float64(roofH)
		halfW := int(float64(size)*0.38*progress) + 1
		for x := cx - halfW; x <= cx+halfW; x++ {
			img.SetNRGBA(x, y, hi)
		}
	}

	// Body
	bodyL := cx - size*28/100
	bodyR := cx + size*28/100
	bodyT := roofBase
	bodyB := size * 80 / 100
	for y := bodyT; y <= bodyB; y++ {
		for x := bodyL; x <= bodyR; x++ {
			img.SetNRGBA(x, y, accent)
		}
	}

	// Door
	doorW := size * 12 / 100
	doorH := size * 20 / 100
	doorL := cx - doorW/2
	doorR := cx + doorW/2
	doorT := bodyB - doorH
	for y := doorT; y <= bodyB; y++ {
		for x := doorL; x <= doorR; x++ {
			img.SetNRGBA(x, y, bg)
		}
	}

	// Server rack dots
	dotR := size * 3 / 100
	if dotR < 2 {
		dotR = 2
	}
	drawCircle(img, cx-size*12/100, bodyT+size*10/100, dotR, green)
	drawCircle(img, cx-size*12/100, bodyT+size*20/100, dotR, red)
	drawCircle(img, cx-size*12/100, bodyT+size*30/100, dotR, green)

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	png.Encode(w, img) //nolint:errcheck
}

func drawCircle(img *image.NRGBA, cx, cy, r int, c color.NRGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

// ──────────────────────────────────────────────
// Template helpers
// ──────────────────────────────────────────────

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return strconv.FormatInt(b, 10) + " B"
	}
}

func formatUptime(s int64) string {
	if s <= 0 {
		return "offline"
	}
	d := s / 86400
	h := (s % 86400) / 3600
	m := (s % 3600) / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

func pct(used, total int64) float64 {
	if total == 0 {
		return 0
	}
	v := float64(used) / float64(total) * 100
	if v > 100 {
		return 100
	}
	return v
}

// ──────────────────────────────────────────────
// Demo data (shown when Proxmox is not configured)
// ──────────────────────────────────────────────

func demoData() []ProxmoxResource {
	return []ProxmoxResource{
		{ID: "qemu/100", VMID: 100, Name: "ubuntu-server", Type: "qemu", Status: "running", Node: "pve-01", CPU: 0.127, Mem: 2684354560, MaxMem: 4294967296, Disk: 10737418240, MaxDisk: 32212254720, Uptime: 349200, Tags: "production"},
		{ID: "qemu/101", VMID: 101, Name: "windows-server", Type: "qemu", Status: "stopped", Node: "pve-01", CPU: 0, Mem: 0, MaxMem: 8589934592, Disk: 21474836480, MaxDisk: 64424509440, Uptime: 0, Tags: ""},
		{ID: "qemu/102", VMID: 102, Name: "k8s-master", Type: "qemu", Status: "running", Node: "pve-02", CPU: 0.43, Mem: 3758096384, MaxMem: 4294967296, Disk: 5368709120, MaxDisk: 32212254720, Uptime: 86410, Tags: "kubernetes"},
		{ID: "lxc/200", VMID: 200, Name: "nginx-proxy", Type: "lxc", Status: "running", Node: "pve-01", CPU: 0.021, Mem: 134217728, MaxMem: 268435456, Disk: 1073741824, MaxDisk: 10737418240, Uptime: 604800, Tags: "networking"},
		{ID: "lxc/201", VMID: 201, Name: "pihole", Type: "lxc", Status: "running", Node: "pve-01", CPU: 0.05, Mem: 125829120, MaxMem: 268435456, Disk: 536870912, MaxDisk: 5368709120, Uptime: 1209600, Tags: "networking"},
		{ID: "lxc/202", VMID: 202, Name: "homeassistant", Type: "lxc", Status: "running", Node: "pve-02", CPU: 0.38, Mem: 1610612736, MaxMem: 2147483648, Disk: 8589934592, MaxDisk: 16106127360, Uptime: 2592000, Tags: "automation"},
		{ID: "lxc/203", VMID: 203, Name: "gitea", Type: "lxc", Status: "stopped", Node: "pve-02", CPU: 0, Mem: 0, MaxMem: 1073741824, Disk: 3221225472, MaxDisk: 21474836480, Uptime: 0, Tags: "devops"},
	}
}

// ──────────────────────────────────────────────
// Main
// ──────────────────────────────────────────────

func main() {
	cfg := loadConfig()

	srv, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("initialising server: %v", err)
	}

	addr := ":" + cfg.Port
	log.Printf("╔══════════════════════════════════════╗")
	log.Printf("║   Homelab Dashboard  —  port %s    ║", cfg.Port)
	log.Printf("╚══════════════════════════════════════╝")
	if cfg.ProxmoxURL == "" {
		log.Printf("⚠  PROXMOX_URL not set — serving demo data")
	} else {
		log.Printf("✓  Proxmox: %s", cfg.ProxmoxURL)
	}

	channels := []string{}
	if cfg.NotifyURL != "" {
		channels = append(channels, "webhook")
	}
	if cfg.TelegramToken != "" {
		channels = append(channels, "telegram")
	}
	if cfg.WhatsAppURL != "" {
		channels = append(channels, "whatsapp")
	}
	if cfg.SMSURL != "" {
		channels = append(channels, "sms")
	}
	if len(channels) > 0 {
		log.Printf("✓  Notifications: %s", strings.Join(channels, ", "))
	} else {
		log.Printf("⚠  No notification channels configured")
	}

	log.Printf("→  http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, srv.Routes()))
}

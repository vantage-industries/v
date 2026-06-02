package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vantageos.local/control-plane/backend/internal/store"
)

const sessionCookieName = "vantageos_session"

var bootstrapNextActions = map[string]string{
	store.BootstrapFirstRun:        "Bring up the onboarding portal",
	store.BootstrapOnboarding:      "Connect a client to the setup SSID and open the portal",
	store.BootstrapWanProbe:        "Verify WAN reachability without blocking setup",
	store.BootstrapGuidedSetup:     "Create the first device and confirm network policy",
	store.BootstrapApplyPending:    "Apply the candidate router configuration",
	store.BootstrapVerifyCandidate: "Confirm the candidate configuration",
	store.BootstrapOperational:     "Review devices and rotate credentials",
	store.BootstrapRecovery:        "Restore onboarding access and continue recovery",
}

type pendingRollbackInfo struct {
	timer     *time.Timer
	deadline  time.Time
	revision  int
	confirmed bool
}

type Server struct {
	store           *store.Store
	mux             *http.ServeMux
	configRoot      string // root for config file writes; empty = use absolute paths
	overrideHostapd string // override for hostapd config path
	overridePskFile string // override for wpa_psk_file path
	overrideNetdDir string // override for systemd/network dir
	overrideDnsmasq string // override for dnsmasq config path

	pendingRollback   *pendingRollbackInfo
	pendingRollbackMu sync.Mutex
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) SetConfigRoot(root string) {
	s.configRoot = root
}

func (s *Server) SetOverrideHostapdPath(path string) {
	s.overrideHostapd = path
}

func (s *Server) SetOverridePskFilePath(path string) {
	s.overridePskFile = path
}

func (s *Server) SetOverrideNetworkdDir(dir string) {
	s.overrideNetdDir = dir
}

func (s *Server) SetOverrideDnsmasqPath(path string) {
	s.overrideDnsmasq = path
}

func (s *Server) getHostapdPath() string {
	if s.overrideHostapd != "" {
		return s.overrideHostapd
	}
	return s.configRoot + store.HostapdConfigPath
}

func (s *Server) getPskFilePath() string {
	if s.overridePskFile != "" {
		return s.overridePskFile
	}
	return s.configRoot + store.WpaPskFilePath
}

func (s *Server) getNetworkdDir() string {
	if s.overrideNetdDir != "" {
		return s.overrideNetdDir
	}
	return s.configRoot + store.NetworkdDir
}

func (s *Server) getDnsmasqPath() string {
	if s.overrideDnsmasq != "" {
		return s.overrideDnsmasq
	}
	return s.configRoot + store.DnsmasqConfigPath
}

const (
	suricataEveLog    = "/var/log/suricata/eve.json"
	trafficStatPath   = "/sys/class/net/wlan0/statistics"
	pollInterval      = 60 * time.Second
)

func readIntFromFile(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var val int64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &val); err != nil {
		return 0, err
	}
	return val, nil
}

func (s *Server) syncRootSSHPassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("password is required")
	}
	// In tests/dev overrides, do not touch host /etc/shadow.
	if s.configRoot != "" {
		return nil
	}

	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader("root:" + password + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chpasswd failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Server) StartBackgroundPollers() {
	go func() {
		for {
			time.Sleep(pollInterval)

			s.collectSuricataStats()
			s.collectTrafficStats()
		}
	}()
}

func (s *Server) collectSuricataStats() {
	status := s.store.SuricataStatus()
	if !status.Enabled {
		return
	}
	data, err := os.ReadFile(suricataEveLog)
	if err != nil {
		return
	}
	var alertsTotal, packetsTotal int64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry struct {
			EventType string   `json:"event_type"`
			Alert     struct{} `json:"alert"`
			Stats     *struct {
				Decoder *struct {
					PktsTotal int64 `json:"pkts_total"`
				} `json:"decoder"`
			} `json:"stats"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		switch entry.EventType {
		case "alert":
			alertsTotal++
		case "stats":
			if entry.Stats != nil && entry.Stats.Decoder != nil {
				packetsTotal = entry.Stats.Decoder.PktsTotal
			}
		}
	}
	if alertsTotal > 0 || packetsTotal > 0 {
		s.store.UpdateSuricataStats(alertsTotal, packetsTotal)
		s.store.AppendSuricataPoint(alertsTotal, packetsTotal)
	}
}

func (s *Server) collectTrafficStats() {
	rxBytes, err := readIntFromFile(trafficStatPath + "/rx_bytes")
	if err != nil {
		return
	}
	txBytes, err := readIntFromFile(trafficStatPath + "/tx_bytes")
	if err != nil {
		return
	}

	prevRx, prevTx, sampled, lastTime := s.store.GetTrafficCounters()
	s.store.UpdateTrafficCounters(rxBytes, txBytes)

	if !sampled {
		return
	}

	elapsed := time.Since(lastTime).Seconds()
	if elapsed <= 0 {
		elapsed = pollInterval.Seconds()
	}

	rxDelta := rxBytes - prevRx
	txDelta := txBytes - prevTx

	if rxDelta < 0 {
		rxDelta = 0
	}
	if txDelta < 0 {
		txDelta = 0
	}

	rxRate := float64(rxDelta) / elapsed
	txRate := float64(txDelta) / elapsed
	s.store.AppendTrafficPoint(rxRate, txRate, rxBytes, txBytes)
}

func NewHandler(st *store.Store) *Server {
	server := &Server{store: st, mux: http.NewServeMux()}

	// Public endpoints (no auth required)
	server.mux.HandleFunc("GET /healthz", server.healthz)
	server.mux.HandleFunc("GET /api/v1/bootstrap", server.bootstrap)
	server.mux.HandleFunc("GET /api/v1/status", server.status)
	server.mux.HandleFunc("GET /api/v1/auth/session", server.authSession)
	server.mux.HandleFunc("POST /api/v1/auth/login", server.authLogin)
	server.mux.HandleFunc("POST /api/v1/auth/logout", server.authLogout)
	server.mux.HandleFunc("POST /api/v1/auth/setup", server.authSetup)
	server.mux.HandleFunc("POST /api/v1/recovery/press", server.recoveryPress)
	server.mux.HandleFunc("POST /api/v1/recovery/reset", server.recoveryReset)
	server.mux.HandleFunc("GET /api/v1/events", server.events)

	// Auth-required endpoints
	server.mux.HandleFunc("POST /api/v1/bootstrap/state", server.requireAuth(server.bootstrapState))
	server.mux.HandleFunc("GET /api/v1/networks", server.requireAuth(server.listNetworks))
	server.mux.HandleFunc("POST /api/v1/networks", server.requireAuth(server.upsertNetwork))
	server.mux.HandleFunc("GET /api/v1/devices", server.requireAuth(server.listDevices))
	server.mux.HandleFunc("POST /api/v1/devices", server.requireAuth(server.createDevice))
	server.mux.HandleFunc("GET /api/v1/devices/{device_id}", server.requireAuth(server.getDevice))
	server.mux.HandleFunc("POST /api/v1/devices/{device_id}/revoke", server.requireAuth(server.revokeDeviceCredential))
	server.mux.HandleFunc("POST /api/v1/devices/{device_id}/finalize", server.requireAuth(server.finalizeDevice))
	server.mux.HandleFunc("POST /api/v1/devices/{device_id}/rotate", server.requireAuth(server.rotateDevice))
	server.mux.HandleFunc("GET /api/v1/config/current", server.requireAuth(server.currentConfig))
	server.mux.HandleFunc("GET /api/v1/config/history", server.requireAuth(server.configHistory))
	server.mux.HandleFunc("POST /api/v1/config/apply", server.requireAuth(server.applyConfig))
	server.mux.HandleFunc("POST /api/v1/config/confirm", server.requireAuth(server.confirmConfig))
	server.mux.HandleFunc("POST /api/v1/config/rollback", server.requireAuth(server.rollbackConfig))
	server.mux.HandleFunc("GET /api/v1/tailscale", server.requireAuth(server.tailscaleStatus))
	server.mux.HandleFunc("POST /api/v1/tailscale/enroll", server.requireAuth(server.tailscaleEnroll))
	server.mux.HandleFunc("GET /api/v1/suricata/status", server.requireAuth(server.suricataStatus))
	server.mux.HandleFunc("GET /api/v1/suricata/history", server.requireAuth(server.suricataHistory))
	server.mux.HandleFunc("POST /api/v1/suricata/enable", server.requireAuth(server.suricataEnable))
	server.mux.HandleFunc("POST /api/v1/suricata/disable", server.requireAuth(server.suricataDisable))
	server.mux.HandleFunc("GET /api/v1/traffic/history", server.requireAuth(server.trafficHistory))

	return server
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !s.store.ValidateSession(cookie.Value) {
			writeError(w, http.StatusUnauthorized, "authenticated session required")
			return
		}
		next(w, r)
	}
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	summary := s.store.Summary()
	tailscale := s.store.TailscaleStatus()
	setup := buildSetupPayload(summary, s.store.ListNetworks(), tailscale.Enabled)
	writeJSON(w, http.StatusOK, bootstrapEnvelope{
		ProductName:  "VantageOS",
		Version:      "0.1.0",
		UIMode:       "spa",
		LaunchBand:   "5 GHz only",
		NetworkModel: networkModel{Zones: []string{"Main", "IoT"}, LegacyPlanned: true},
		Setup:        setup,
		PasswordSet:  summary.AdminAuth.PasswordHash != "",
		Recovery:     summary.Recovery,
	})
}

func (s *Server) bootstrapState(w http.ResponseWriter, r *http.Request) {
	var req bootstrapStateRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.State) == "" {
		writeError(w, http.StatusBadRequest, "state is required")
		return
	}

	item, changed, err := s.store.TransitionBootstrapState(req.State, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"item": item, "changed": changed})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	summary := s.store.Summary()
	tailscale := s.store.TailscaleStatus()
	suricata := s.store.SuricataStatus()
	setup := buildSetupPayload(summary, s.store.ListNetworks(), tailscale.Enabled)

	var pendingRollback *pendingRollbackStatus
	s.pendingRollbackMu.Lock()
	if s.pendingRollback != nil && !s.pendingRollback.confirmed {
		expiresIn := time.Until(s.pendingRollback.deadline)
		if expiresIn > 0 {
			pendingRollback = &pendingRollbackStatus{
				Revision:    s.pendingRollback.revision,
				ExpiresInMs: int(expiresIn.Milliseconds()),
			}
		}
	}
	s.pendingRollbackMu.Unlock()

	writeJSON(w, http.StatusOK, statusEnvelope{
		Timestamp:        float64(time.Now().UnixNano()) / 1e9,
		Services:         defaultServiceStatuses(tailscale, suricata),
		Tailscale:        tailscale,
		Networks:         s.store.ListNetworks(),
		Totals:           summary.Totals,
		LatestEvents:     summary.Events,
		ActiveRevision:   summary.ActiveRevision,
		Setup:            setup,
		BootstrapState:   summary.BootstrapState,
		PasswordSet:      summary.AdminAuth.PasswordHash != "",
		Recovery:         summary.Recovery,
		PendingRollback:  pendingRollback,
	})
}

func (s *Server) listNetworks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListNetworks()})
}

func (s *Server) upsertNetwork(w http.ResponseWriter, r *http.Request) {
	var req upsertNetworkRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = "main"
	}
	if slug != "main" {
		writeError(w, http.StatusBadRequest, "only the 'main' network is supported in this release")
		return
	}
	authMode := strings.TrimSpace(req.AuthMode)
	if authMode == "" {
		authMode = "wpa2-psk"
	}

	item, err := s.store.UpsertNetwork(store.Network{
		Slug:     "main",
		Name:     strings.TrimSpace(req.Name),
		SSID:     strings.TrimSpace(req.SSID),
		Zone:     "trusted",
		Band:     strings.TrimSpace(req.Band),
		AuthMode: authMode,
		Enabled:  req.Enabled == nil || *req.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDevices()})
}

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	var req createDeviceRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	networkSlug := strings.TrimSpace(req.NetworkSlug)
	if name == "" || networkSlug == "" {
		writeError(w, http.StatusBadRequest, "name and network_slug are required")
		return
	}
	if networkSlug != "main" {
		writeError(w, http.StatusBadRequest, "only network_slug='main' is supported in this release")
		return
	}

	includeFallback := true
	if req.IncludeFallback != nil {
		includeFallback = *req.IncludeFallback
	}

	result, credentials, err := s.store.CreateOnboardingDevice(name, networkSlug, includeFallback)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"device": result, "credentials": credentials})
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("device_id")
	device, ok := s.store.GetDevice(deviceID)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": device, "credentials": s.store.GetDeviceCredentials(deviceID)})
}

func (s *Server) revokeDeviceCredential(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("device_id")
	var req revokeCredentialRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Kind != "secure" && req.Kind != "fallback" {
		writeError(w, http.StatusBadRequest, "kind must be secure or fallback")
		return
	}

	item, ok, err := s.store.RevokeCredential(deviceID, req.Kind)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) finalizeDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("device_id")
	var req finalizeDeviceRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.EnrolledVia != "secure" && req.EnrolledVia != "fallback" {
		writeError(w, http.StatusBadRequest, "enrolled_via must be secure or fallback")
		return
	}

	item, ok, err := s.store.MarkDeviceEnrolled(deviceID, req.EnrolledVia, req.MatchedCredentialID, req.RxBytes, req.TxBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if req.EnrolledVia == "secure" {
		_, _, _ = s.store.RevokeCredential(deviceID, "fallback")
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) rotateDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("device_id")
	device, ok := s.store.GetDevice(deviceID)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": device, "status": "queued"})
}

func (s *Server) currentConfig(w http.ResponseWriter, r *http.Request) {
	active := s.store.GetActiveConfigRevision()
	revision := 0
	status := "draft"
	if active != nil {
		revision = active.Revision
		status = active.Status
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"revision": revision,
		"status":   status,
		"platform": "router-appliance",
		"target":   "raspberrypi5",
		"active":   active,
	})
}

func (s *Server) configHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListConfigRevisions()})
}

func writeConfigFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// RegenerateRuntimeConfigs writes single-SSID WPA2 configs on startup.
// Only writes if setupPSK exists (meaning the wizard was completed at least once).
func (s *Server) RegenerateRuntimeConfigs() error {
	setupPSK := s.store.GetSetupPSK()
	if setupPSK == "" {
		return nil // wizard not yet completed; fallback Yocto configs handle this
	}

	networks := s.store.ListNetworks()
	mainSSID := s.store.MainSSID()

	hostapdConfig := store.GenerateSingleSSIDConfig(mainSSID, networks)
	if err := writeConfigFile(s.getHostapdPath(), hostapdConfig); err != nil {
		return fmt.Errorf("hostapd config: %w", err)
	}

	creds := s.store.ListActiveCredentials()
	pskContent := store.GenerateWpaPskFile(setupPSK, creds)
	if err := writeConfigFile(s.getPskFilePath(), pskContent); err != nil {
		return fmt.Errorf("wpa_psk_file: %w", err)
	}

	wlan0Config := store.GenerateWlan0NetworkConfigSubnet(store.MainSubnetCIDR)
	if err := writeConfigFile(s.getNetworkdDir()+"/25-wlan0.network", wlan0Config); err != nil {
		return fmt.Errorf("wlan0 network config: %w", err)
	}

	_ = os.Remove(s.getNetworkdDir() + "/25-wlan0-1.network")

	dnsmasqConfig := store.GenerateDnsmasqConfigOperational()
	if err := writeConfigFile(s.getDnsmasqPath(), dnsmasqConfig); err != nil {
		return fmt.Errorf("dnsmasq config: %w", err)
	}

	return nil
}

func (s *Server) applyConfig(w http.ResponseWriter, r *http.Request) {
	var req applyConfigRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	networks := s.store.ListNetworks()
	if len(networks) == 0 {
		writeError(w, http.StatusBadRequest, "no networks configured — create at least one network before applying")
		return
	}
	mainSSID := s.store.MainSSID()
	setupPSK, err := s.store.GetOrCreateSetupPSK()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Write WPA2-PSK hostapd config (single SSID, no dual-BSS)
	hostapdConfig := store.GenerateSingleSSIDConfig(mainSSID, networks)
	if err := writeConfigFile(s.getHostapdPath(), hostapdConfig); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write hostapd config: "+err.Error())
		return
	}

	creds := s.store.ListActiveCredentials()
	pskContent := store.GenerateWpaPskFile(setupPSK, creds)
	if err := writeConfigFile(s.getPskFilePath(), pskContent); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write wpa_psk_file: "+err.Error())
		return
	}

	// Write operational subnet network config
	wlan0Config := store.GenerateWlan0NetworkConfigSubnet(store.MainSubnetCIDR)
	if err := writeConfigFile(s.getNetworkdDir()+"/25-wlan0.network", wlan0Config); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write network config: "+err.Error())
		return
	}

	_ = os.Remove(s.getNetworkdDir() + "/25-wlan0-1.network")

	dnsmasqConfig := store.GenerateDnsmasqConfigOperational()
	if err := writeConfigFile(s.getDnsmasqPath(), dnsmasqConfig); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write dnsmasq config: "+err.Error())
		return
	}

	// Create revision and transition state before restarting services
	revision, err := s.store.CreateConfigRevision(req.Title, req.Note, "applied")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.store.TransitionBootstrapState(store.BootstrapOperational, "setup completed, config applied")

	// Send response before restarting hostapd (WiFi will drop)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "applied",
		"item":      revision,
		"main_ssid": mainSSID,
	})

	// Start 60s auto-rollback timer — user must confirm via the UI
	s.pendingRollbackMu.Lock()
	if s.pendingRollback != nil {
		s.pendingRollback.timer.Stop()
	}
	rollbackTimer := time.AfterFunc(60*time.Second, func() {
		s.pendingRollbackMu.Lock()
		pr := s.pendingRollback
		s.pendingRollbackMu.Unlock()
		if pr == nil || pr.confirmed {
			return
		}
		log.Printf("auto-rollback: no confirmation within 60s, reverting to revision %d", pr.revision-1)
		_, err := s.store.RollbackConfigRevision(pr.revision - 1)
		if err != nil {
			log.Printf("auto-rollback failed: %v", err)
			return
		}
		if err := s.RegenerateRuntimeConfigs(); err != nil {
			log.Printf("auto-rollback: regenerate configs failed: %v", err)
		}
		runCommand("networkctl", "reload")
		runCommand("systemctl", "reload-or-restart", "vantageos-hostapd.service")
		runCommand("systemctl", "reload-or-restart", "dnsmasq.service")
	})
	s.pendingRollback = &pendingRollbackInfo{
		revision:  revision.Revision,
		confirmed: false,
		deadline:  time.Now().Add(60 * time.Second),
		timer:     rollbackTimer,
	}
	s.pendingRollbackMu.Unlock()

	// Restart services asynchronously — the response is already sent
	go func() {
		runCommand("networkctl", "reload")
		runCommand("systemctl", "reload-or-restart", "vantageos-hostapd.service")
		runCommand("systemctl", "reload-or-restart", "dnsmasq.service")
	}()
}

func (s *Server) confirmConfig(w http.ResponseWriter, r *http.Request) {
	s.pendingRollbackMu.Lock()
	if s.pendingRollback != nil {
		s.pendingRollback.confirmed = true
		s.pendingRollback.timer.Stop()
		s.pendingRollback = nil
	}
	s.pendingRollbackMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
}

func (s *Server) rollbackConfig(w http.ResponseWriter, r *http.Request) {
	var req rollbackConfigRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var targetRevision int
	if req.Revision == nil {
		history := s.store.ListConfigRevisions()
		if len(history) < 2 {
			writeError(w, http.StatusBadRequest, "no prior revision to rollback to")
			return
		}
		targetRevision = history[1].Revision
	} else {
		targetRevision = *req.Revision
	}

	revision, err := s.store.RollbackConfigRevision(targetRevision)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.RegenerateRuntimeConfigs(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to regenerate runtime configs: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "rolled-back", "item": revision})

	go func() {
		runCommand("networkctl", "reload")
		runCommand("systemctl", "reload-or-restart", "vantageos-hostapd.service")
		runCommand("systemctl", "reload-or-restart", "dnsmasq.service")
	}()
}

func (s *Server) tailscaleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.TailscaleStatus())
}

func (s *Server) tailscaleEnroll(w http.ResponseWriter, r *http.Request) {
	var req tailscaleEnrollRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status, err := s.store.RequestTailscaleEnrollment(req.Hostname, req.AdvertiseRoutes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": status.State, "tailscale": status})
}

func (s *Server) suricataStatus(w http.ResponseWriter, r *http.Request) {
	status := s.store.SuricataStatus()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) suricataEnable(w http.ResponseWriter, r *http.Request) {
	status, err := s.store.RequestSuricataEnrollment()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	go runCommand("systemctl", "start", "suricata.service")
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) suricataDisable(w http.ResponseWriter, r *http.Request) {
	status, err := s.store.DisableSuricata()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	go runCommand("systemctl", "stop", "suricata.service")
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) suricataHistory(w http.ResponseWriter, r *http.Request) {
	snapshot := s.store.SuricataSnapshot()
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) trafficHistory(w http.ResponseWriter, r *http.Request) {
	snapshot := s.store.TrafficSnapshot()
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	_, _ = fmt.Fprint(w, "event: ready\n")
	_, _ = fmt.Fprint(w, "data: {}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprint(w, "event: heartbeat\n")
			_, _ = fmt.Fprintf(w, "data: {\"timestamp\": %.3f}\n\n", float64(time.Now().UnixNano())/1e9)
			flusher.Flush()
		}
	}
}

func (s *Server) authSession(w http.ResponseWriter, r *http.Request) {
	adminStatus := s.store.AdminStatus()
	if !adminStatus.PasswordSet {
		writeJSON(w, http.StatusOK, adminStatus)
		return
	}
	adminStatus.Authenticated = false
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && s.store.ValidateSession(cookie.Value) {
		adminStatus.Authenticated = true
	}
	writeJSON(w, http.StatusOK, adminStatus)
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	token, err := s.store.VerifyAndCreateSession(req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	s.store.ClearSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) authSetup(w http.ResponseWriter, r *http.Request) {
	adminStatus := s.store.AdminStatus()
	if adminStatus.PasswordSet {
		writeError(w, http.StatusBadRequest, "password already set")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.SetAdminPassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.syncRootSSHPassword(req.Password); err != nil {
		log.Printf("authSetup: failed to sync SSH password: %v", err)
	}

	token, err := s.store.VerifyAndCreateSession(req.Password)
	if err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	} else {
		log.Printf("authSetup: session creation failed after password set: %v", err)
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "password_set"})
}

func (s *Server) recoveryPress(w http.ResponseWriter, r *http.Request) {
	state := s.store.RecordRecoveryPress()
	writeJSON(w, http.StatusOK, map[string]any{"recovery": state})
}

func (s *Server) recoveryReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	if err := s.store.RecoverPassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.syncRootSSHPassword(req.Password); err != nil {
		log.Printf("recoveryReset: failed to sync SSH password: %v", err)
	}

	token, err := s.store.VerifyAndCreateSession(req.Password)
	if err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
}

func buildSetupPayload(summary store.Summary, networks []store.Network, tailscaleEnabled bool) setupPayload {
	state := store.BootstrapFirstRun
	var bootstrap *store.BootstrapState
	if summary.BootstrapState != nil {
		bootstrap = summary.BootstrapState
		if strings.TrimSpace(bootstrap.State) != "" {
			state = bootstrap.State
		}
	}

	checklist := []checklistItem{
		{Key: "portal", Label: "Open the onboarding portal", Done: state != store.BootstrapFirstRun},
		{Key: "networks", Label: "Review main network", Done: len(networks) >= 1},
		{Key: "device", Label: "Create the first device", Done: summary.Totals.DeviceCount > 0},
		{Key: "secure", Label: "Confirm secure QR onboarding", Done: summary.Totals.ActiveCredentialCount > 0},
		{Key: "apply", Label: "Apply and verify router config", Done: state == store.BootstrapApplyPending || state == store.BootstrapVerifyCandidate || state == store.BootstrapOperational},
		{Key: "tailscale", Label: "Enable remote admin with Tailscale", Done: tailscaleEnabled},
	}

	result := setupPayload{
		NeedsSetup:          state != store.BootstrapOperational,
		State:               state,
		StateLabel:          strings.ReplaceAll(state, "_", " "),
		Stage:               state,
		NextAction:          bootstrapNextActions[state],
		LastTransitionAt:    "",
		SetupCompletedAt:    "",
		RecoveryRequestedAt: "",
		Checklist:           checklist,
	}
	if result.NextAction == "" {
		result.NextAction = "Continue setup"
	}
	if bootstrap != nil {
		result.LastTransitionAt = bootstrap.LastTransitionAt
		result.SetupCompletedAt = bootstrap.SetupCompletedAt
		result.RecoveryRequestedAt = bootstrap.RecoveryRequestedAt
	}
	return result
}

func defaultServiceStatuses(tailscale store.TailscaleState, suricata store.SuricataState) []serviceStatus {
	tailscaleState := strings.TrimSpace(tailscale.State)
	if !tailscale.Enabled {
		tailscaleState = "not configured"
	}
	if tailscaleState == "" {
		tailscaleState = "configured"
	}

	suricataState := strings.TrimSpace(suricata.State)
	if !suricata.Enabled {
		suricataState = "disabled"
	}
	if suricataState == "" {
		suricataState = "available"
	}

	return []serviceStatus{
		{Name: "systemd-networkd", State: "healthy"},
		{Name: "hostapd", State: "healthy"},
		{Name: "nftables", State: "planned"},
		{Name: "dnsmasq", State: "planned"},
		{Name: "tailscale", State: tailscaleState},
		{Name: "suricata", State: suricataState},
	}
}

func decodeJSON(body io.ReadCloser, target any) error {
	defer body.Close()
	decoder := json.NewDecoder(io.LimitReader(body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type bootstrapEnvelope struct {
	ProductName  string            `json:"product_name"`
	Version      string            `json:"version"`
	UIMode       string            `json:"ui_mode"`
	LaunchBand   string            `json:"launch_band"`
	NetworkModel networkModel      `json:"network_model"`
	Setup        setupPayload      `json:"setup"`
	PasswordSet  bool              `json:"password_set"`
	Recovery     store.RecoveryState `json:"recovery"`
}

type networkModel struct {
	Zones         []string `json:"zones"`
	LegacyPlanned bool     `json:"legacy_planned"`
}

type checklistItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Done  bool   `json:"done"`
}

type setupPayload struct {
	NeedsSetup          bool            `json:"needs_setup"`
	State               string          `json:"state"`
	StateLabel          string          `json:"state_label"`
	Stage               string          `json:"stage"`
	NextAction          string          `json:"next_action"`
	LastTransitionAt    string          `json:"last_transition_at,omitempty"`
	SetupCompletedAt    string          `json:"setup_completed_at,omitempty"`
	RecoveryRequestedAt string          `json:"recovery_requested_at,omitempty"`
	Checklist           []checklistItem `json:"checklist"`
}

type pendingRollbackStatus struct {
	Revision    int `json:"revision"`
	ExpiresInMs int `json:"expires_in_ms"`
}

type statusEnvelope struct {
	Timestamp        float64                `json:"timestamp"`
	Services         []serviceStatus        `json:"services"`
	Tailscale        store.TailscaleState   `json:"tailscale"`
	Networks         []store.Network        `json:"networks"`
	Totals           store.Totals           `json:"totals"`
	LatestEvents     []store.Event          `json:"latest_events"`
	ActiveRevision   *store.ConfigRevision  `json:"active_revision"`
	Setup            setupPayload           `json:"setup"`
	BootstrapState   *store.BootstrapState  `json:"bootstrap_state"`
	PasswordSet      bool                   `json:"password_set"`
	Recovery         store.RecoveryState    `json:"recovery"`
	PendingRollback  *pendingRollbackStatus `json:"pending_rollback,omitempty"`
}

type serviceStatus struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type bootstrapStateRequest struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type tailscaleEnrollRequest struct {
	Hostname        string   `json:"hostname,omitempty"`
	AdvertiseRoutes []string `json:"advertise_routes,omitempty"`
}

type upsertNetworkRequest struct {
	Slug     string `json:"slug"`
	Name     string `json:"name,omitempty"`
	SSID     string `json:"ssid,omitempty"`
	Zone     string `json:"zone,omitempty"`
	Band     string `json:"band,omitempty"`
	AuthMode string `json:"auth_mode,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

type createDeviceRequest struct {
	Name            string `json:"name"`
	NetworkSlug     string `json:"network_slug"`
	IncludeFallback *bool  `json:"include_fallback,omitempty"`
}

type revokeCredentialRequest struct {
	Kind string `json:"kind"`
}

type finalizeDeviceRequest struct {
	EnrolledVia         string `json:"enrolled_via"`
	MatchedCredentialID string `json:"matched_credential_id,omitempty"`
	RxBytes             *int64 `json:"rx_bytes,omitempty"`
	TxBytes             *int64 `json:"tx_bytes,omitempty"`
}

type applyConfigRequest struct {
	Title string `json:"title,omitempty"`
	Note  string `json:"note,omitempty"`
}

type rollbackConfigRequest struct {
	Revision *int `json:"revision,omitempty"`
}

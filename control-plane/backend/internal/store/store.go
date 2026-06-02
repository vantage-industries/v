package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var defaultNetworks = []Network{
	{
		Slug:      "main",
		Name:      "Main",
		SSID:      "VantageOS",
		Zone:      "trusted",
		Band:      "5ghz",
		AuthMode:  "wpa2-psk",
		Enabled:   true,
		CreatedAt: nowISO(),
	},
}

type Store struct {
	mu   sync.Mutex
	path string
	doc  Document

	suricataPoints     []DataPoint
	rxRatePoints       []DataPoint
	txRatePoints       []DataPoint
	lastRxBytes        int64
	lastTxBytes        int64
	lastTrafficSample  time.Time
	lastSuricataAlerts int64
	lastSuricataPkts   int64
}

func New(path string) *Store {
	if strings.TrimSpace(path) == "" {
		path = DefaultStatePath
	}
	return &Store{path: path}
}

func (s *Store) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	if data, err := os.ReadFile(s.path); err == nil {
		if err := json.Unmarshal(data, &s.doc); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dirty := false
	if s.seedNetworksLocked() {
		dirty = true
	}
	if s.normalizeSingleNetworkLocked() {
		dirty = true
	}
	if s.seedBootstrapLocked() {
		dirty = true
	}
	if s.seedTailscaleLocked() {
		dirty = true
	}
	if s.seedSuricataLocked() {
		dirty = true
	}
	if s.seedInitialRevisionLocked() {
		dirty = true
	}
	if s.seedRecoveryLocked() {
		dirty = true
	}

	if dirty {
		return s.saveLocked()
	}

	return nil
}

func (s *Store) ListNetworks() []Network {
	s.mu.Lock()
	defer s.mu.Unlock()

	networks := append([]Network(nil), s.doc.Networks...)
	sort.Slice(networks, func(i, j int) bool { return networks[i].Name < networks[j].Name })
	return networks
}

func (s *Store) GetNetwork(slug string) (Network, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, network := range s.doc.Networks {
		if network.Slug == slug {
			return network, true
		}
	}
	return Network{}, false
}

func (s *Store) UpsertNetwork(input Network) (Network, error) {
	if strings.TrimSpace(input.Slug) == "" {
		return Network{}, errors.New("slug is required")
	}
	if strings.TrimSpace(input.SSID) == "" {
		return Network{}, errors.New("ssid is required")
	}

	if strings.TrimSpace(input.Name) == "" {
		input.Name = strings.ToUpper(input.Slug[:1]) + input.Slug[1:]
	}
	if strings.TrimSpace(input.Zone) == "" {
		input.Zone = input.Slug
	}
	if strings.TrimSpace(input.Band) == "" {
		input.Band = "5ghz"
	}
	if strings.TrimSpace(input.AuthMode) == "" {
		input.AuthMode = "wpa3"
	}
	if input.CreatedAt == "" {
		input.CreatedAt = nowISO()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := input
	for i := range s.doc.Networks {
		if s.doc.Networks[i].Slug == input.Slug {
			updated.CreatedAt = s.doc.Networks[i].CreatedAt
			s.doc.Networks[i].Name = input.Name
			s.doc.Networks[i].SSID = input.SSID
			s.doc.Networks[i].Zone = input.Zone
			s.doc.Networks[i].Band = input.Band
			s.doc.Networks[i].AuthMode = input.AuthMode
			s.doc.Networks[i].Enabled = input.Enabled
			if err := s.saveLocked(); err != nil {
				return Network{}, err
			}
			return s.doc.Networks[i], nil
		}
	}

	s.doc.Networks = append(s.doc.Networks, updated)
	if err := s.saveLocked(); err != nil {
		return Network{}, err
	}
	return updated, nil
}

func (s *Store) ListDevices() []DeviceView {
	s.mu.Lock()
	defer s.mu.Unlock()

	views := make([]DeviceView, 0, len(s.doc.Devices))
	for _, device := range s.doc.Devices {
		views = append(views, s.deviceViewLocked(device))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].CreatedAt > views[j].CreatedAt })
	return views
}

func (s *Store) GetDevice(deviceID string) (DeviceView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, device := range s.doc.Devices {
		if device.ID == deviceID {
			return s.deviceViewLocked(device), true
		}
	}
	return DeviceView{}, false
}

func (s *Store) GetDeviceCredentials(deviceID string) []Credential {
	s.mu.Lock()
	defer s.mu.Unlock()

	credentials := make([]Credential, 0)
	for _, credential := range s.doc.Credentials {
		if credential.DeviceID == deviceID {
			credentials = append(credentials, credential)
		}
	}
	sort.Slice(credentials, func(i, j int) bool { return credentials[i].CreatedAt < credentials[j].CreatedAt })
	return credentials
}

func (s *Store) ListActiveCredentials() []Credential {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := make([]Credential, 0)
	for _, c := range s.doc.Credentials {
		if c.Active {
			active = append(active, c)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].CreatedAt < active[j].CreatedAt
	})
	return active
}

func (s *Store) TailscaleStatus() TailscaleState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.tailscaleStatusLocked()
}

func (s *Store) RequestTailscaleEnrollment(hostname string, advertiseRoutes []string) (TailscaleState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seedTailscaleLocked()

	status := s.tailscaleStatusLocked()
	status.Enabled = true
	status.State = "enrollment requested"
	status.Hostname = strings.TrimSpace(hostname)
	status.AdvertiseRoutes = normalizeStrings(advertiseRoutes)
	status.RequestedAt = nowISO()
	s.doc.Tailscale = status
	s.appendEventLocked("tailscale.enrollment_requested", map[string]any{
		"hostname":         status.Hostname,
		"advertise_routes": status.AdvertiseRoutes,
	})
	if err := s.saveLocked(); err != nil {
		return TailscaleState{}, err
	}

	return s.tailscaleStatusLocked(), nil
}

func (s *Store) CreateOnboardingDevice(name, networkSlug string, includeFallback bool) (DeviceView, []Credential, error) {
	if strings.TrimSpace(name) == "" {
		return DeviceView{}, nil, errors.New("name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	network, ok := s.networkBySlugLocked(networkSlug)
	if !ok {
		return DeviceView{}, nil, fmt.Errorf("unknown network: %s", networkSlug)
	}

	deviceID, err := randomID()
	if err != nil {
		return DeviceView{}, nil, err
	}
	secureSecret, err := randomString(32, defaultSecureAlphabet)
	if err != nil {
		return DeviceView{}, nil, err
	}

	createdAt := nowISO()
	device := DeviceRecord{
		ID:          deviceID,
		Name:        name,
		NetworkSlug: networkSlug,
		JoinState:   "pending",
		RxBytes:     0,
		TxBytes:     0,
		CreatedAt:   createdAt,
	}
	s.doc.Devices = append(s.doc.Devices, device)

	secureCredential := Credential{
		ID:        mustRandomID(),
		DeviceID:  deviceID,
		Kind:      "secure",
		Secret:    secureSecret,
		Active:    true,
		CreatedAt: createdAt,
		QRPayload: wifiQRPayload(network.SSID, secureSecret, "WPA"),
	}
	s.doc.Credentials = append(s.doc.Credentials, secureCredential)

	credentials := []Credential{secureCredential}
	if includeFallback {
		fallbackSecret, err := randomString(8, defaultFallbackAlphabet)
		if err != nil {
			return DeviceView{}, nil, err
		}
		fallbackCredential := Credential{
			ID:        mustRandomID(),
			DeviceID:  deviceID,
			Kind:      "fallback",
			Secret:    fallbackSecret,
			Active:    true,
			CreatedAt: createdAt,
			QRPayload: wifiQRPayload(network.SSID, fallbackSecret, "WPA"),
		}
		s.doc.Credentials = append(s.doc.Credentials, fallbackCredential)
		credentials = append(credentials, fallbackCredential)
	}

	s.appendEventLocked("device.created", map[string]any{
		"device_id":        deviceID,
		"network_slug":     networkSlug,
		"include_fallback": includeFallback,
	})

	if err := s.saveLocked(); err != nil {
		return DeviceView{}, nil, err
	}

	return s.deviceViewLocked(device), credentials, nil
}

func (s *Store) RevokeCredential(deviceID, kind string) (Credential, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.doc.Credentials {
		credential := &s.doc.Credentials[i]
		if credential.DeviceID == deviceID && credential.Kind == kind {
			if !credential.Active {
				return *credential, true, nil
			}
			credential.Active = false
			credential.RevokedAt = nowISO()
			s.appendEventLocked("credential.revoked", map[string]any{
				"device_id":       deviceID,
				"credential_kind": kind,
				"credential_id":   credential.ID,
			})
			if err := s.saveLocked(); err != nil {
				return Credential{}, false, err
			}
			return *credential, true, nil
		}
	}

	return Credential{}, false, nil
}

func (s *Store) MarkDeviceEnrolled(deviceID, enrolledVia, matchedCredentialID string, rxBytes, txBytes *int64) (DeviceView, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.doc.Devices {
		if s.doc.Devices[i].ID != deviceID {
			continue
		}

		now := nowISO()
		device := &s.doc.Devices[i]
		device.JoinState = "enrolled"
		device.EnrolledVia = enrolledVia
		if device.FirstSeenAt == "" {
			device.FirstSeenAt = now
		}
		device.LastSeenAt = now
		if rxBytes != nil {
			device.RxBytes = *rxBytes
		}
		if txBytes != nil {
			device.TxBytes = *txBytes
		}

		s.appendEventLocked("device.enrolled", map[string]any{
			"device_id":             deviceID,
			"enrolled_via":          enrolledVia,
			"matched_credential_id": matchedCredentialID,
			"rx_bytes":              rxBytes,
			"tx_bytes":              txBytes,
		})
		if err := s.saveLocked(); err != nil {
			return DeviceView{}, false, err
		}
		return s.deviceViewLocked(*device), true, nil
	}

	return DeviceView{}, false, nil
}

func (s *Store) TransitionBootstrapState(state, reason string) (BootstrapState, bool, error) {
	if err := validateBootstrapState(state); err != nil {
		return BootstrapState{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.doc.BootstrapState.State == "" {
		s.seedBootstrapLocked()
	}

	current := s.doc.BootstrapState
	if current.State == state {
		return current, false, nil
	}

	now := nowISO()
	updated := current
	updated.PreviousState = current.State
	updated.State = state
	updated.LastReason = reason
	updated.UpdatedAt = now
	updated.LastTransitionAt = now
	if state == BootstrapOperational && updated.SetupCompletedAt == "" {
		updated.SetupCompletedAt = now
	}
	if state == BootstrapRecovery && updated.RecoveryRequestedAt == "" {
		updated.RecoveryRequestedAt = now
	}
	s.doc.BootstrapState = updated
	s.doc.BootstrapHistory = append(s.doc.BootstrapHistory, BootstrapTransition{
		ID:        len(s.doc.BootstrapHistory) + 1,
		FromState: current.State,
		ToState:   state,
		Reason:    reason,
		CreatedAt: now,
	})
	s.appendEventLocked("bootstrap.state_changed", map[string]any{
		"from_state": current.State,
		"to_state":   state,
		"reason":     reason,
	})
	if err := s.saveLocked(); err != nil {
		return BootstrapState{}, false, err
	}
	return s.doc.BootstrapState, true, nil
}

func (s *Store) GetBootstrapState() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.BootstrapState.State
}

func (s *Store) GetSetupPSK() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.SetupPSK
}

func (s *Store) ListBootstrapTransitions() []BootstrapTransition {
	s.mu.Lock()
	defer s.mu.Unlock()
	transitions := append([]BootstrapTransition(nil), s.doc.BootstrapHistory...)
	sort.Slice(transitions, func(i, j int) bool { return transitions[i].ID < transitions[j].ID })
	return transitions
}

func (s *Store) ListConfigRevisions() []ConfigRevision {
	s.mu.Lock()
	defer s.mu.Unlock()
	revisions := append([]ConfigRevision(nil), s.doc.ConfigRevisions...)
	sort.Slice(revisions, func(i, j int) bool { return revisions[i].Revision > revisions[j].Revision })
	return revisions
}

func (s *Store) GetActiveConfigRevision() *ConfigRevision {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeRevisionLocked()
}

func (s *Store) CreateConfigRevision(title, note, status string) (ConfigRevision, error) {
	if strings.TrimSpace(title) == "" {
		title = "Applied configuration"
	}
	if strings.TrimSpace(note) == "" {
		note = "Snapshot captured from the control plane"
	}
	if strings.TrimSpace(status) == "" {
		status = "applied"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	revision := s.nextRevisionLocked()
	s.doc.ConfigRevisions = s.deactivateAllRevisionsLocked()
	snapshot := s.currentSnapshotLocked()
	record := ConfigRevision{
		ID:        mustRandomID(),
		Revision:  revision,
		Title:     title,
		Note:      note,
		Status:    status,
		Active:    true,
		Snapshot:  snapshot,
		CreatedAt: nowISO(),
	}
	s.doc.ConfigRevisions = append(s.doc.ConfigRevisions, record)
	s.appendEventLocked("config.applied", map[string]any{
		"revision": revision,
		"title":    title,
	})
	if err := s.saveLocked(); err != nil {
		return ConfigRevision{}, err
	}
	return record, nil
}

func (s *Store) RollbackConfigRevision(revisionNumber int) (ConfigRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := -1
	for i := range s.doc.ConfigRevisions {
		if s.doc.ConfigRevisions[i].Revision == revisionNumber {
			index = i
			break
		}
	}
	if index < 0 {
		return ConfigRevision{}, fmt.Errorf("revision not found")
	}
	if s.doc.ConfigRevisions[index].Active {
		return s.doc.ConfigRevisions[index], nil
	}

	s.doc.ConfigRevisions = s.deactivateAllRevisionsLocked()
	s.doc.ConfigRevisions[index].Active = true
	s.doc.ConfigRevisions[index].Status = "rolled-back"
	s.appendEventLocked("config.rolled_back", map[string]any{
		"revision": revisionNumber,
	})
	if err := s.saveLocked(); err != nil {
		return ConfigRevision{}, err
	}
	return s.doc.ConfigRevisions[index], nil
}

func (s *Store) Summary() Summary {
	s.mu.Lock()
	defer s.mu.Unlock()

	totals := Totals{NetworkCount: len(s.doc.Networks), DeviceCount: len(s.doc.Devices), CredentialCount: len(s.doc.Credentials)}
	for _, credential := range s.doc.Credentials {
		if credential.Active {
			totals.ActiveCredentialCount++
		}
	}

	latest := s.latestEventsLocked(5)
	active := s.activeRevisionLocked()
	bootstrap := s.doc.BootstrapState
	return Summary{
		Totals:         totals,
		Events:         latest,
		ActiveRevision: active,
		BootstrapState: &bootstrap,
		AdminAuth:      s.doc.AdminAuth,
		Recovery:       s.doc.Recovery,
	}
}

func (s *Store) latestEventsLocked(limit int) []Event {
	if limit <= 0 || len(s.doc.Events) == 0 {
		return nil
	}
	start := len(s.doc.Events) - limit
	if start < 0 {
		start = 0
	}
	selected := append([]Event(nil), s.doc.Events[start:]...)
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}
	return selected
}

func (s *Store) activeRevisionLocked() *ConfigRevision {
	for i := len(s.doc.ConfigRevisions) - 1; i >= 0; i-- {
		if s.doc.ConfigRevisions[i].Active {
			revision := s.doc.ConfigRevisions[i]
			return &revision
		}
	}
	return nil
}

func (s *Store) nextRevisionLocked() int {
	max := 0
	for _, revision := range s.doc.ConfigRevisions {
		if revision.Revision > max {
			max = revision.Revision
		}
	}
	return max + 1
}

func (s *Store) deactivateAllRevisionsLocked() []ConfigRevision {
	updated := make([]ConfigRevision, len(s.doc.ConfigRevisions))
	copy(updated, s.doc.ConfigRevisions)
	for i := range updated {
		updated[i].Active = false
	}
	return updated
}

func (s *Store) currentSnapshotLocked() ConfigSnapshot {
	devices := make([]DeviceView, 0, len(s.doc.Devices))
	for _, device := range s.doc.Devices {
		devices = append(devices, s.deviceViewLocked(device))
	}
	networks := append([]Network(nil), s.doc.Networks...)
	credentials := make([]Credential, len(s.doc.Credentials))
	for i, credential := range s.doc.Credentials {
		credentials[i] = credential
		if credential.Active {
			credentials[i].Secret = "redacted"
		}
	}
	return ConfigSnapshot{Networks: networks, Devices: devices, Credentials: credentials}
}

func (s *Store) deviceViewLocked(device DeviceRecord) DeviceView {
	return DeviceView{
		ID:              device.ID,
		Name:            device.Name,
		NetworkSlug:     device.NetworkSlug,
		NetworkName:     s.networkNameLocked(device.NetworkSlug),
		JoinState:       device.JoinState,
		EnrolledVia:     device.EnrolledVia,
		FirstSeenAt:     device.FirstSeenAt,
		LastSeenAt:      device.LastSeenAt,
		RxBytes:         device.RxBytes,
		TxBytes:         device.TxBytes,
		CreatedAt:       device.CreatedAt,
		CredentialCount: s.credentialCountLocked(device.ID),
	}
}

func (s *Store) networkNameLocked(slug string) string {
	for _, network := range s.doc.Networks {
		if network.Slug == slug {
			return network.Name
		}
	}
	return slug
}

func (s *Store) networkBySlugLocked(slug string) (Network, bool) {
	for _, network := range s.doc.Networks {
		if network.Slug == slug {
			return network, true
		}
	}
	return Network{}, false
}

func (s *Store) credentialCountLocked(deviceID string) int {
	count := 0
	for _, credential := range s.doc.Credentials {
		if credential.DeviceID == deviceID {
			count++
		}
	}
	return count
}

func (s *Store) appendEventLocked(kind string, payload any) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		payloadBytes = []byte(`{}`)
	}
	s.doc.Events = append(s.doc.Events, Event{Kind: kind, Payload: string(payloadBytes), CreatedAt: nowISO()})
}

func appendDataPointRing(points []DataPoint, ts float64, value float64) []DataPoint {
	points = append(points, DataPoint{Timestamp: ts, Value: value})
	if len(points) > MaxHistoryPoints {
		points = points[len(points)-MaxHistoryPoints:]
	}
	return points
}

func (s *Store) AppendSuricataPoint(alertsTotal, packetsTotal int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delta := alertsTotal - s.lastSuricataAlerts
	if delta < 0 {
		delta = alertsTotal // counter reset, use the full value
	}
	s.lastSuricataAlerts = alertsTotal
	s.lastSuricataPkts = packetsTotal
	ts := float64(time.Now().UnixMilli()) / 1000
	s.suricataPoints = appendDataPointRing(s.suricataPoints, ts, float64(delta))
}

func (s *Store) SuricataSnapshot() SuricataSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	series := make([]DataPoint, len(s.suricataPoints))
	copy(series, s.suricataPoints)
	status := s.suricataStatusLocked()
	return SuricataSnapshot{
		AlertsTotal:  status.AlertsTotal,
		PacketsTotal: status.PacketsTotal,
		Series:       series,
	}
}

func (s *Store) AppendTrafficPoint(rxRate, txRate float64, rxBytes, txBytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := float64(time.Now().UnixMilli()) / 1000
	s.rxRatePoints = appendDataPointRing(s.rxRatePoints, ts, rxRate)
	s.txRatePoints = appendDataPointRing(s.txRatePoints, ts, txRate)
}

func (s *Store) TrafficSnapshot() TrafficSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	rxSeries := make([]DataPoint, len(s.rxRatePoints))
	copy(rxSeries, s.rxRatePoints)
	txSeries := make([]DataPoint, len(s.txRatePoints))
	copy(txSeries, s.txRatePoints)
	return TrafficSnapshot{
		RxSeries: rxSeries,
		TxSeries: txSeries,
	}
}

func (s *Store) GetTrafficCounters() (rxBytes, txBytes int64, sampled bool, lastTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRxBytes, s.lastTxBytes, !s.lastTrafficSample.IsZero(), s.lastTrafficSample
}

func (s *Store) UpdateTrafficCounters(rxBytes, txBytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRxBytes = rxBytes
	s.lastTxBytes = txBytes
	s.lastTrafficSample = time.Now()
}

func (s *Store) seedNetworksLocked() bool {
	if len(s.doc.Networks) > 0 {
		return false
	}
	networks := make([]Network, len(defaultNetworks))
	copy(networks, defaultNetworks)
	s.doc.Networks = networks
	return true
}

func (s *Store) normalizeSingleNetworkLocked() bool {
	if len(s.doc.Networks) == 0 {
		return false
	}

	mainIdx := -1
	for i, n := range s.doc.Networks {
		if n.Slug == "main" {
			mainIdx = i
			break
		}
	}
	if mainIdx < 0 {
		mainIdx = 0
	}

	main := s.doc.Networks[mainIdx]
	changed := false

	if main.Slug != "main" {
		main.Slug = "main"
		changed = true
	}
	if strings.TrimSpace(main.Name) == "" {
		main.Name = "Main"
		changed = true
	}
	if strings.TrimSpace(main.Zone) != "trusted" {
		main.Zone = "trusted"
		changed = true
	}
	if strings.TrimSpace(main.Band) == "" {
		main.Band = "5ghz"
		changed = true
	}
	if strings.TrimSpace(main.AuthMode) != "wpa2-psk" {
		main.AuthMode = "wpa2-psk"
		changed = true
	}
	if strings.TrimSpace(main.SSID) == "" {
		main.SSID = defaultOperationalSSID()
		changed = true
	}
	if !main.Enabled {
		main.Enabled = true
		changed = true
	}

	if len(s.doc.Networks) != 1 {
		changed = true
	}
	s.doc.Networks = []Network{main}

	return changed
}

func (s *Store) seedBootstrapLocked() bool {
	if s.doc.BootstrapState.State != "" {
		return false
	}
	now := nowISO()
	s.doc.BootstrapState = BootstrapState{
		State:            BootstrapFirstRun,
		LastReason:       "seeded on first boot",
		CreatedAt:        now,
		UpdatedAt:        now,
		LastTransitionAt: now,
	}
	s.doc.BootstrapHistory = append(s.doc.BootstrapHistory, BootstrapTransition{
		ID:        1,
		FromState: "bootstrap",
		ToState:   BootstrapFirstRun,
		Reason:    "seeded on first boot",
		CreatedAt: now,
	})
	s.appendEventLocked("bootstrap.seeded", map[string]any{"state": BootstrapFirstRun})
	return true
}

func (s *Store) seedTailscaleLocked() bool {
	if s.doc.Tailscale.State != "" {
		return false
	}
	s.doc.Tailscale = TailscaleState{
		Enabled: false,
		State:   "not configured",
	}
	return true
}

func (s *Store) tailscaleStatusLocked() TailscaleState {
	status := s.doc.Tailscale
	if strings.TrimSpace(status.State) == "" {
		status.State = "not configured"
	}
	status.AdvertiseRoutes = append([]string(nil), status.AdvertiseRoutes...)
	return status
}

func (s *Store) seedSuricataLocked() bool {
	if s.doc.Suricata.State != "" {
		return false
	}
	s.doc.Suricata = SuricataState{
		Enabled: false,
		State:   "not configured",
	}
	return true
}

func (s *Store) SuricataStatus() SuricataState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.suricataStatusLocked()
}

func (s *Store) suricataStatusLocked() SuricataState {
	status := s.doc.Suricata
	if strings.TrimSpace(status.State) == "" {
		status.State = "not configured"
	}
	return status
}

func (s *Store) RequestSuricataEnrollment() (SuricataState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seedSuricataLocked()

	status := s.suricataStatusLocked()
	status.Enabled = true
	status.State = "enabled"
	status.UpdatedAt = nowISO()
	s.doc.Suricata = status
	s.appendEventLocked("suricata.enabled", map[string]any{
		"state": status.State,
	})
	if err := s.saveLocked(); err != nil {
		return SuricataState{}, err
	}

	return s.suricataStatusLocked(), nil
}

func (s *Store) DisableSuricata() (SuricataState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seedSuricataLocked()

	status := s.suricataStatusLocked()
	status.Enabled = false
	status.State = "disabled"
	status.UpdatedAt = nowISO()
	s.doc.Suricata = status
	s.appendEventLocked("suricata.disabled", map[string]any{
		"state": status.State,
	})
	if err := s.saveLocked(); err != nil {
		return SuricataState{}, err
	}

	return s.suricataStatusLocked(), nil
}

func (s *Store) UpdateSuricataStats(alertsTotal, packetsTotal int64) (SuricataState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seedSuricataLocked()

	status := s.suricataStatusLocked()
	status.AlertsTotal = alertsTotal
	status.PacketsTotal = packetsTotal
	status.UpdatedAt = nowISO()
	s.doc.Suricata = status
	if err := s.saveLocked(); err != nil {
		return SuricataState{}, err
	}

	return s.suricataStatusLocked(), nil
}

func (s *Store) seedRecoveryLocked() bool {
	if s.doc.Recovery.Stage != "" {
		return false
	}
	s.doc.Recovery = defaultRecoveryState()
	return true
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func (s *Store) seedInitialRevisionLocked() bool {
	if len(s.doc.ConfigRevisions) > 0 {
		return false
	}
	revision := ConfigRevision{
		ID:        mustRandomID(),
		Revision:  1,
		Title:     "Initial configuration",
		Note:      "Seeded from defaults",
		Status:    "applied",
		Active:    true,
		Snapshot:  s.currentSnapshotLocked(),
		CreatedAt: nowISO(),
	}
	s.doc.ConfigRevisions = append(s.doc.ConfigRevisions, revision)
	s.appendEventLocked("config.seeded", map[string]any{"revision": 1, "title": "Initial configuration"})
	return true
}

func (s *Store) saveLocked() error {
	tempFile, err := os.CreateTemp(filepath.Dir(s.path), "vantageos-state-*.json")
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(tempFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(s.doc); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return err
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return err
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return err
	}
	if err := os.Rename(tempFile.Name(), s.path); err != nil {
		os.Remove(tempFile.Name())
		return err
	}

	dir, err := os.Open(filepath.Dir(s.path))
	if err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	return nil
}

func mustRandomID() string {
	id, err := randomID()
	if err != nil {
		panic(err)
	}
	return id
}

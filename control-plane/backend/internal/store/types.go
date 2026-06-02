package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const DefaultStatePath = "/var/lib/vantageos/vantageos-state.json"
const MaxHistoryPoints = 60

const (
	BootstrapFirstRun        = "first_run"
	BootstrapOnboarding      = "onboarding_portal"
	BootstrapWanProbe        = "wan_probe"
	BootstrapGuidedSetup     = "guided_setup"
	BootstrapApplyPending    = "apply_pending"
	BootstrapVerifyCandidate = "verify_candidate"
	BootstrapOperational     = "operational"
	BootstrapRecovery        = "recovery"
)

var validBootstrapStates = map[string]struct{}{
	BootstrapFirstRun:        {},
	BootstrapOnboarding:      {},
	BootstrapWanProbe:        {},
	BootstrapGuidedSetup:     {},
	BootstrapApplyPending:    {},
	BootstrapVerifyCandidate: {},
	BootstrapOperational:     {},
	BootstrapRecovery:        {},
}

const (
	defaultSecureAlphabet   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	defaultFallbackAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

const (
	RecoveryStageIdle      = "idle"
	RecoveryStageListening = "listening"
	RecoveryStageActive    = "active"
	RecoveryTriggerPinhole = "pinhole_button"
)

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func validateBootstrapState(state string) error {
	if _, ok := validBootstrapStates[strings.TrimSpace(state)]; !ok {
		return fmt.Errorf("unknown bootstrap state: %s", state)
	}
	return nil
}

func randomString(length int, alphabet string) (string, error) {
	if length <= 0 {
		return "", nil
	}
	if alphabet == "" {
		return "", errors.New("alphabet cannot be empty")
	}

	max := big.NewInt(int64(len(alphabet)))
	var builder strings.Builder
	builder.Grow(length)

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}

	return builder.String(), nil
}

func randomID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func wifiQRPayload(ssid, secret, security string) string {
	escapedSSID := escapeWifiValue(ssid)
	escapedSecret := escapeWifiValue(secret)
	return "WIFI:T:" + security + ";S:" + escapedSSID + ";P:" + escapedSecret + ";H:false;;"
}

func escapeWifiValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, ";", `\;`)
	value = strings.ReplaceAll(value, ",", `\,`)
	value = strings.ReplaceAll(value, ":", `\:`)
	return value
}

type Network struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SSID      string `json:"ssid"`
	Zone      string `json:"zone"`
	Band      string `json:"band"`
	AuthMode  string `json:"auth_mode"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

type DeviceRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	NetworkSlug string `json:"network_slug"`
	JoinState   string `json:"join_state"`
	EnrolledVia string `json:"enrolled_via,omitempty"`
	FirstSeenAt string `json:"first_seen_at,omitempty"`
	LastSeenAt  string `json:"last_seen_at,omitempty"`
	RxBytes     int64  `json:"rx_bytes"`
	TxBytes     int64  `json:"tx_bytes"`
	CreatedAt   string `json:"created_at"`
}

type DeviceView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	NetworkSlug     string `json:"network_slug"`
	NetworkName     string `json:"network_name"`
	JoinState       string `json:"join_state"`
	EnrolledVia     string `json:"enrolled_via,omitempty"`
	FirstSeenAt     string `json:"first_seen_at,omitempty"`
	LastSeenAt      string `json:"last_seen_at,omitempty"`
	RxBytes         int64  `json:"rx_bytes"`
	TxBytes         int64  `json:"tx_bytes"`
	CreatedAt       string `json:"created_at"`
	CredentialCount int    `json:"credential_count"`
}

type Credential struct {
	ID        string `json:"id"`
	DeviceID  string `json:"device_id"`
	Kind      string `json:"kind"`
	Secret    string `json:"secret"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	RevokedAt string `json:"revoked_at,omitempty"`
	QRPayload string `json:"qr_payload"`
}

type TailscaleState struct {
	Enabled         bool     `json:"enabled"`
	State           string   `json:"state"`
	Hostname        string   `json:"hostname,omitempty"`
	AdvertiseRoutes []string `json:"advertise_routes,omitempty"`
	RequestedAt     string   `json:"requested_at,omitempty"`
}

type SuricataState struct {
	Enabled     bool   `json:"enabled"`
	State       string `json:"state"`
	AlertsTotal int64  `json:"alerts_total,omitempty"`
	PacketsTotal int64 `json:"packets_total,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type AdminAuthState struct {
	PasswordHash      string `json:"password_hash,omitempty"`
	PasswordUpdatedAt string `json:"password_updated_at,omitempty"`
	SessionTokenHash  string `json:"session_token_hash,omitempty"`
	SessionCreatedAt  string `json:"session_created_at,omitempty"`
	SessionExpiresAt  string `json:"session_expires_at,omitempty"`
	FailedLoginCount  int    `json:"failed_login_count,omitempty"`
	LastFailedLoginAt string `json:"last_failed_login_at,omitempty"`
}

type RecoveryState struct {
	Stage                 string `json:"stage"`
	Active                bool   `json:"active"`
	RequiredPresses       int    `json:"required_presses"`
	PressWindowSeconds    int    `json:"press_window_seconds"`
	RecoveryWindowSeconds int    `json:"recovery_window_seconds"`
	PressCount            int    `json:"press_count"`
	PressWindowStartedAt  string `json:"press_window_started_at,omitempty"`
	PressWindowExpiresAt  string `json:"press_window_expires_at,omitempty"`
	ActivatedAt           string `json:"activated_at,omitempty"`
	RecoveryExpiresAt     string `json:"recovery_expires_at,omitempty"`
	LastPressAt           string `json:"last_press_at,omitempty"`
	TriggerSource         string `json:"trigger_source,omitempty"`
	PreviousBootstrapState string `json:"previous_bootstrap_state,omitempty"`
}

type AdminStatus struct {
	PasswordSet       bool          `json:"password_set"`
	Authenticated     bool          `json:"authenticated"`
	PasswordUpdatedAt  string        `json:"password_updated_at,omitempty"`
	SessionExpiresAt   string        `json:"session_expires_at,omitempty"`
	Recovery           RecoveryState `json:"recovery"`
}

type Event struct {
	Kind      string `json:"kind"`
	Payload   string `json:"payload"`
	CreatedAt string `json:"created_at"`
}

type BootstrapState struct {
	State               string `json:"state"`
	PreviousState       string `json:"previous_state,omitempty"`
	LastReason          string `json:"last_reason,omitempty"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	LastTransitionAt    string `json:"last_transition_at"`
	SetupCompletedAt    string `json:"setup_completed_at,omitempty"`
	RecoveryRequestedAt string `json:"recovery_requested_at,omitempty"`
}

type BootstrapTransition struct {
	ID        int    `json:"id"`
	FromState string `json:"from_state"`
	ToState   string `json:"to_state"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"created_at"`
}

type ConfigSnapshot struct {
	Networks    []Network    `json:"networks"`
	Devices     []DeviceView `json:"devices"`
	Credentials []Credential `json:"credentials"`
}

type ConfigRevision struct {
	ID        string         `json:"id"`
	Revision  int            `json:"revision"`
	Title     string         `json:"title"`
	Note      string         `json:"note"`
	Status    string         `json:"status"`
	Active    bool           `json:"active"`
	Snapshot  ConfigSnapshot `json:"snapshot"`
	CreatedAt string         `json:"created_at"`
}

type Totals struct {
	NetworkCount          int `json:"network_count"`
	DeviceCount           int `json:"device_count"`
	CredentialCount       int `json:"credential_count"`
	ActiveCredentialCount int `json:"active_credential_count"`
}

type DataPoint struct {
	Timestamp float64 `json:"timestamp"`
	Value     float64 `json:"value"`
}

type SuricataSnapshot struct {
	AlertsTotal  int64       `json:"alerts_total"`
	PacketsTotal int64       `json:"packets_total"`
	Series       []DataPoint `json:"series"`
}

type TrafficSnapshot struct {
	RxSeries []DataPoint `json:"rx_series"`
	TxSeries []DataPoint `json:"tx_series"`
}

type Summary struct {
	Totals         Totals          `json:"totals"`
	Events         []Event         `json:"events"`
	ActiveRevision *ConfigRevision `json:"active_revision,omitempty"`
	BootstrapState *BootstrapState `json:"bootstrap_state,omitempty"`
	AdminAuth      AdminAuthState  `json:"admin_auth,omitempty"`
	Recovery       RecoveryState   `json:"recovery,omitempty"`
}

type Document struct {
	BootstrapState   BootstrapState        `json:"bootstrap_state"`
	BootstrapHistory []BootstrapTransition `json:"bootstrap_history"`
	AdminAuth        AdminAuthState        `json:"admin_auth"`
	Recovery         RecoveryState         `json:"recovery"`
	Tailscale        TailscaleState        `json:"tailscale"`
	Suricata         SuricataState         `json:"suricata"`
	Networks         []Network             `json:"networks"`
	Devices          []DeviceRecord        `json:"devices"`
	Credentials      []Credential          `json:"credentials"`
	Events           []Event               `json:"events"`
	ConfigRevisions  []ConfigRevision      `json:"config_revisions"`
	SetupPSK         string                `json:"setup_psk,omitempty"`
	SetupSSID        string                `json:"setup_ssid,omitempty"`
}

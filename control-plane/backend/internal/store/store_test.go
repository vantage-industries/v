package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeSeedsCoreState(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	summary := st.Summary()
	if summary.BootstrapState == nil {
		t.Fatal("expected bootstrap state")
	}
	if summary.BootstrapState.State != BootstrapFirstRun {
		t.Fatalf("unexpected bootstrap state: %s", summary.BootstrapState.State)
	}
	if summary.Totals.NetworkCount != 1 {
		t.Fatalf("unexpected network count: %d", summary.Totals.NetworkCount)
	}
	if summary.Totals.DeviceCount != 0 {
		t.Fatalf("unexpected device count: %d", summary.Totals.DeviceCount)
	}
	if got := st.ListBootstrapTransitions(); len(got) != 1 || got[0].ToState != BootstrapFirstRun {
		t.Fatalf("unexpected bootstrap history: %#v", got)
	}
	if got := st.ListConfigRevisions(); len(got) != 1 || !got[0].Active {
		t.Fatalf("unexpected config revisions: %#v", got)
	}
}

func TestBootstrapTransitionPersistsAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st := New(path)
	if err := st.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	updated, changed, err := st.TransitionBootstrapState(BootstrapGuidedSetup, "opened the portal")
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if !changed {
		t.Fatal("expected transition to change state")
	}
	if updated.State != BootstrapGuidedSetup {
		t.Fatalf("unexpected state: %s", updated.State)
	}

	again, changed, err := st.TransitionBootstrapState(BootstrapGuidedSetup, "duplicate request")
	if err != nil {
		t.Fatalf("duplicate transition: %v", err)
	}
	if changed {
		t.Fatal("expected duplicate transition to be idempotent")
	}
	if again.State != BootstrapGuidedSetup {
		t.Fatalf("unexpected repeated state: %s", again.State)
	}

	reloaded := New(path)
	if err := reloaded.Initialize(); err != nil {
		t.Fatalf("reinitialize: %v", err)
	}
	if got := reloaded.Summary(); got.BootstrapState == nil || got.BootstrapState.State != BootstrapGuidedSetup {
		t.Fatalf("bootstrap state did not persist: %#v", got.BootstrapState)
	}
	if got := reloaded.ListBootstrapTransitions(); len(got) != 2 {
		t.Fatalf("unexpected transition count: %#v", got)
	}
}

func TestDeviceCreationAndCredentialRevocation(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	device, credentials, err := st.CreateOnboardingDevice("Kitchen camera", "main", true)
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	if device.CredentialCount != 2 {
		t.Fatalf("unexpected credential count: %d", device.CredentialCount)
	}
	if len(credentials) != 2 {
		t.Fatalf("unexpected credential list: %#v", credentials)
	}

	revoked, ok, err := st.RevokeCredential(device.ID, "fallback")
	if err != nil {
		t.Fatalf("revoke fallback: %v", err)
	}
	if !ok || revoked.Active {
		t.Fatalf("expected fallback revocation, got %#v", revoked)
	}

	revokedAgain, ok, err := st.RevokeCredential(device.ID, "fallback")
	if err != nil {
		t.Fatalf("revoke fallback twice: %v", err)
	}
	if !ok || revokedAgain.Active {
		t.Fatalf("expected idempotent revoke, got %#v", revokedAgain)
	}
}

func TestConfigRevisionLifecycle(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	revision, err := st.CreateConfigRevision("Operator update", "Applied by test", "applied")
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if revision.Revision != 2 || !revision.Active {
		t.Fatalf("unexpected revision after apply: %#v", revision)
	}

	rolledBack, err := st.RollbackConfigRevision(1)
	if err != nil {
		t.Fatalf("rollback revision: %v", err)
	}
	if rolledBack.Revision != 1 || rolledBack.Status != "rolled-back" || !rolledBack.Active {
		t.Fatalf("unexpected revision after rollback: %#v", rolledBack)
	}

	rolledBackAgain, err := st.RollbackConfigRevision(1)
	if err != nil {
		t.Fatalf("rollback revision twice: %v", err)
	}
	if rolledBackAgain.Revision != 1 || !rolledBackAgain.Active {
		t.Fatalf("unexpected idempotent rollback: %#v", rolledBackAgain)
	}
}

func TestGenerateWpaPskFileIncludesActiveCredentials(t *testing.T) {
	content := GenerateWpaPskFile("setup-pass-123", []Credential{
		{Kind: "secure", Secret: "secure-pass-001", Active: true},
		{Kind: "fallback", Secret: "fallback8", Active: true},
		{Kind: "secure", Secret: "secure-pass-001", Active: true}, // duplicate
		{Kind: "secure", Secret: "short", Active: true},           // invalid length
		{Kind: "secure", Secret: "inactive-pass", Active: false},  // inactive
	})

	if !strings.Contains(content, "setup-pass-123") {
		t.Fatalf("missing setup PSK in generated file: %q", content)
	}
	if !strings.Contains(content, "secure-pass-001") {
		t.Fatalf("missing secure credential in generated file: %q", content)
	}
	if !strings.Contains(content, "fallback8") {
		t.Fatalf("missing fallback credential in generated file: %q", content)
	}
	if strings.Contains(content, "inactive-pass") {
		t.Fatalf("inactive credential should not be included: %q", content)
	}
	if strings.Contains(content, "short") {
		t.Fatalf("invalid short credential should not be included: %q", content)
	}
	if got := strings.Count(content, "secure-pass-001"); got != 1 {
		t.Fatalf("expected deduplicated secure credential once, got %d in %q", got, content)
	}
}

func TestCreateOnboardingDeviceUsesWPAQRCodePayload(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	_, credentials, err := st.CreateOnboardingDevice("Sensor", "main", true)
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	if len(credentials) == 0 {
		t.Fatal("expected credentials")
	}
	for _, cred := range credentials {
		if !strings.Contains(cred.QRPayload, "WIFI:T:WPA;") {
			t.Fatalf("unexpected QR payload security for %s: %s", cred.Kind, cred.QRPayload)
		}
	}
}

func TestInitializeNormalizesToSingleMainNetwork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st := New(path)
	if err := st.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	_, err := st.UpsertNetwork(Network{
		Slug:     "iot",
		Name:     "IoT",
		SSID:     "Legacy-IoT",
		Zone:     "iot",
		Band:     "5ghz",
		AuthMode: "wpa3",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("upsert legacy iot network: %v", err)
	}

	reloaded := New(path)
	if err := reloaded.Initialize(); err != nil {
		t.Fatalf("reinitialize: %v", err)
	}
	networks := reloaded.ListNetworks()
	if len(networks) != 1 {
		t.Fatalf("expected a single normalized network, got %d: %#v", len(networks), networks)
	}
	if networks[0].Slug != "main" {
		t.Fatalf("expected normalized slug main, got %#v", networks[0])
	}
	if networks[0].Zone != "trusted" {
		t.Fatalf("expected normalized trusted zone, got %#v", networks[0])
	}
	if networks[0].AuthMode != "wpa2-psk" {
		t.Fatalf("expected normalized wpa2-psk auth, got %#v", networks[0])
	}
}

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vantageos.local/control-plane/backend/internal/api"
	"vantageos.local/control-plane/backend/internal/store"
)

type bootstrapResponse struct {
	Setup struct {
		State      string           `json:"state"`
		NeedsSetup bool             `json:"needs_setup"`
		Checklist  []checklistEntry `json:"checklist"`
	} `json:"setup"`
}

type statusResponse struct {
	Setup struct {
		State      string           `json:"state"`
		NeedsSetup bool             `json:"needs_setup"`
		Checklist  []checklistEntry `json:"checklist"`
	} `json:"setup"`
	Totals struct {
		DeviceCount int `json:"device_count"`
	} `json:"totals"`
	BootstrapState struct {
		State string `json:"state"`
	} `json:"bootstrap_state"`
	Tailscale struct {
		Enabled         bool     `json:"enabled"`
		State           string   `json:"state"`
		Hostname        string   `json:"hostname"`
		AdvertiseRoutes []string `json:"advertise_routes"`
		RequestedAt     string   `json:"requested_at"`
	} `json:"tailscale"`
}

type tailscaleResponse struct {
	Enabled bool   `json:"enabled"`
	State   string `json:"state"`
}

type checklistEntry struct {
	Key  string `json:"key"`
	Done bool   `json:"done"`
}

type createDeviceResponse struct {
	Device struct {
		ID              string `json:"id"`
		JoinState       string `json:"join_state"`
		EnrolledVia     string `json:"enrolled_via"`
		NetworkName     string `json:"network_name"`
		CredentialCount int    `json:"credential_count"`
	} `json:"device"`
	Credentials []struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Secret string `json:"secret"`
	} `json:"credentials"`
}

type listEnvelope struct {
	Items []struct {
		ID              string `json:"id"`
		JoinState       string `json:"join_state"`
		CredentialCount int    `json:"credential_count"`
	} `json:"items"`
}

func newHandler(t *testing.T) (*store.Store, http.Handler) {
	t.Helper()
	st := store.New(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	handler := api.NewHandler(st)
	handler.SetConfigRoot(t.TempDir())
	return st, handler
}

func newHandlerWithAuth(t *testing.T) (*store.Store, http.Handler, string) {
	t.Helper()
	st, handler := newHandler(t)
	if err := st.SetAdminPassword("test-password"); err != nil {
		t.Fatalf("set admin password: %v", err)
	}
	token, err := st.VerifyAndCreateSession("test-password")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return st, handler, token
}

func doJSON(t *testing.T, handler http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return doJSONWithCookie(t, handler, method, target, body, "")
}

func doJSONWithCookie(t *testing.T, handler http.Handler, method, target string, body any, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, target, reader)
	if cookie != "" {
		req.Header.Set("Cookie", "vantageos_session="+cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func checklistDone(items []checklistEntry, key string) bool {
	for _, item := range items {
		if item.Key == key {
			return item.Done
		}
	}
	return false
}

func TestBootstrapAndStatusExposePersistedState(t *testing.T) {
	st, handler, token := newHandlerWithAuth(t)
	_ = st

	do := func(method, target string, body any) *httptest.ResponseRecorder {
		return doJSONWithCookie(t, handler, method, target, body, token)
	}

	healthRec := doJSON(t, handler, http.MethodGet, "/healthz", nil)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status: %d", healthRec.Code)
	}

	bootstrapRec := doJSON(t, handler, http.MethodGet, "/api/v1/bootstrap", nil)
	if bootstrapRec.Code != http.StatusOK {
		t.Fatalf("bootstrap status: %d", bootstrapRec.Code)
	}
	var bootstrap bootstrapResponse
	if err := json.Unmarshal(bootstrapRec.Body.Bytes(), &bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if bootstrap.Setup.State != store.BootstrapFirstRun || !bootstrap.Setup.NeedsSetup {
		t.Fatalf("unexpected bootstrap payload: %#v", bootstrap)
	}
	if done := checklistDone(bootstrap.Setup.Checklist, "tailscale"); done {
		t.Fatalf("unexpected bootstrap tailscale checklist state: %#v", bootstrap.Setup.Checklist)
	}

	transitionRec := do("POST", "/api/v1/bootstrap/state", map[string]any{
		"state":  store.BootstrapGuidedSetup,
		"reason": "opened portal",
	})
	if transitionRec.Code != http.StatusOK {
		t.Fatalf("transition status: %d", transitionRec.Code)
	}

	deviceRec := do("POST", "/api/v1/devices", map[string]any{
		"name":             "Kitchen camera",
		"network_slug":     "main",
		"include_fallback": true,
	})
	if deviceRec.Code != http.StatusCreated {
		t.Fatalf("device status: %d", deviceRec.Code)
	}
	var createdDevice createDeviceResponse
	if err := json.Unmarshal(deviceRec.Body.Bytes(), &createdDevice); err != nil {
		t.Fatalf("decode device response: %v", err)
	}
	if len(createdDevice.Credentials) != 2 || createdDevice.Device.CredentialCount != 2 {
		t.Fatalf("unexpected device response: %#v", createdDevice)
	}

	finalizeRec := do("POST", "/api/v1/devices/"+createdDevice.Device.ID+"/finalize", map[string]any{
		"enrolled_via":          "secure",
		"matched_credential_id": createdDevice.Credentials[0].ID,
	})
	if finalizeRec.Code != http.StatusOK {
		t.Fatalf("finalize status: %d", finalizeRec.Code)
	}

	devicesListRec := do("GET", "/api/v1/devices", nil)
	if devicesListRec.Code != http.StatusOK {
		t.Fatalf("devices list status: %d", devicesListRec.Code)
	}
	var devicesList listEnvelope
	if err := json.Unmarshal(devicesListRec.Body.Bytes(), &devicesList); err != nil {
		t.Fatalf("decode devices list: %v", err)
	}
	if len(devicesList.Items) != 1 || devicesList.Items[0].JoinState != "enrolled" {
		t.Fatalf("unexpected devices list: %#v", devicesList)
	}

	statusRec := doJSON(t, handler, http.MethodGet, "/api/v1/status", nil)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status code: %d", statusRec.Code)
	}
	var status statusResponse
	if err := json.Unmarshal(statusRec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Setup.State != store.BootstrapGuidedSetup || !status.Setup.NeedsSetup {
		t.Fatalf("unexpected status setup: %#v", status)
	}
	if status.Totals.DeviceCount != 1 {
		t.Fatalf("unexpected device count: %d", status.Totals.DeviceCount)
	}
	if status.BootstrapState.State != store.BootstrapGuidedSetup {
		t.Fatalf("unexpected bootstrap state: %#v", status.BootstrapState)
	}
	if done := checklistDone(status.Setup.Checklist, "tailscale"); done {
		t.Fatalf("unexpected status tailscale checklist state before enroll: %#v", status.Setup.Checklist)
	}

	currentConfigRec := do("GET", "/api/v1/config/current", nil)
	if currentConfigRec.Code != http.StatusOK {
		t.Fatalf("current config status: %d", currentConfigRec.Code)
	}

	historyRec := do("GET", "/api/v1/config/history", nil)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("config history status: %d", historyRec.Code)
	}

	networksRec := do("GET", "/api/v1/networks", nil)
	if networksRec.Code != http.StatusOK {
		t.Fatalf("networks status: %d", networksRec.Code)
	}

	upsertRec := do("POST", "/api/v1/networks", map[string]any{
		"slug":      "main",
		"name":      "Main",
		"ssid":      "VantageOS-Prod",
		"zone":      "trusted",
		"band":      "5ghz",
		"auth_mode": "wpa2-psk",
	})
	if upsertRec.Code != http.StatusCreated {
		t.Fatalf("upsert network status: %d", upsertRec.Code)
	}

	applyRec := do("POST", "/api/v1/config/apply", map[string]any{})
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status: %d", applyRec.Code)
	}

	rollbackRec := do("POST", "/api/v1/config/rollback", map[string]any{})
	if rollbackRec.Code != http.StatusOK {
		t.Fatalf("rollback status: %d", rollbackRec.Code)
	}

	tailscaleStatusRec := do("GET", "/api/v1/tailscale", nil)
	if tailscaleStatusRec.Code != http.StatusOK {
		t.Fatalf("tailscale status code: %d", tailscaleStatusRec.Code)
	}
	var tailscale tailscaleResponse
	if err := json.Unmarshal(tailscaleStatusRec.Body.Bytes(), &tailscale); err != nil {
		t.Fatalf("decode tailscale: %v", err)
	}
	if tailscale.State != "not configured" || tailscale.Enabled {
		t.Fatalf("unexpected tailscale status: %#v", tailscale)
	}

	tailscaleEnrollRec := do("POST", "/api/v1/tailscale/enroll", map[string]any{})
	if tailscaleEnrollRec.Code != http.StatusOK {
		t.Fatalf("tailscale enroll status: %d", tailscaleEnrollRec.Code)
	}
	var tailscaleEnroll struct {
		Status    string            `json:"status"`
		Tailscale tailscaleResponse `json:"tailscale"`
	}
	if err := json.Unmarshal(tailscaleEnrollRec.Body.Bytes(), &tailscaleEnroll); err != nil {
		t.Fatalf("decode tailscale enroll: %v", err)
	}
	if tailscaleEnroll.Status != "enrollment requested" || !tailscaleEnroll.Tailscale.Enabled {
		t.Fatalf("unexpected tailscale enroll response: %#v", tailscaleEnroll)
	}

	statusAfterEnrollRec := doJSON(t, handler, http.MethodGet, "/api/v1/status", nil)
	if statusAfterEnrollRec.Code != http.StatusOK {
		t.Fatalf("status after tailscale enroll code: %d", statusAfterEnrollRec.Code)
	}
	var statusAfterEnroll statusResponse
	if err := json.Unmarshal(statusAfterEnrollRec.Body.Bytes(), &statusAfterEnroll); err != nil {
		t.Fatalf("decode status after tailscale enroll: %v", err)
	}
	if !statusAfterEnroll.Tailscale.Enabled || statusAfterEnroll.Tailscale.State != "enrollment requested" || statusAfterEnroll.Tailscale.RequestedAt == "" {
		t.Fatalf("unexpected tailscale status after enroll: %#v", statusAfterEnroll.Tailscale)
	}
	if done := checklistDone(statusAfterEnroll.Setup.Checklist, "tailscale"); !done {
		t.Fatalf("expected tailscale checklist to be done after enroll: %#v", statusAfterEnroll.Setup.Checklist)
	}

	bootstrapAfterEnrollRec := doJSON(t, handler, http.MethodGet, "/api/v1/bootstrap", nil)
	if bootstrapAfterEnrollRec.Code != http.StatusOK {
		t.Fatalf("bootstrap after tailscale enroll code: %d", bootstrapAfterEnrollRec.Code)
	}
	var bootstrapAfterEnroll bootstrapResponse
	if err := json.Unmarshal(bootstrapAfterEnrollRec.Body.Bytes(), &bootstrapAfterEnroll); err != nil {
		t.Fatalf("decode bootstrap after tailscale enroll: %v", err)
	}
	if done := checklistDone(bootstrapAfterEnroll.Setup.Checklist, "tailscale"); !done {
		t.Fatalf("expected bootstrap tailscale checklist to be done after enroll: %#v", bootstrapAfterEnroll.Setup.Checklist)
	}

	configApplyRec := do("POST", "/api/v1/config/apply", map[string]any{})
	if configApplyRec.Code != http.StatusOK {
		t.Fatalf("config apply status: %d", configApplyRec.Code)
	}
	configRollbackRec := do("POST", "/api/v1/config/rollback", map[string]any{})
	if configRollbackRec.Code != http.StatusOK {
		t.Fatalf("config rollback status: %d", configRollbackRec.Code)
	}
}

func TestBootstrapStateEndpointRejectsUnknownStates(t *testing.T) {
	_, handler, token := newHandlerWithAuth(t)
	rec := doJSONWithCookie(t, handler, http.MethodPost, "/api/v1/bootstrap/state", map[string]any{"state": "bogus"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSingleNetworkModeRejectsNonMainNetwork(t *testing.T) {
	_, handler, token := newHandlerWithAuth(t)

	upsertRec := doJSONWithCookie(t, handler, http.MethodPost, "/api/v1/networks", map[string]any{
		"slug":      "iot",
		"name":      "IoT",
		"ssid":      "VantageOS-IoT",
		"zone":      "iot",
		"band":      "5ghz",
		"auth_mode": "wpa2-psk",
	}, token)
	if upsertRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-main upsert, got %d", upsertRec.Code)
	}

	deviceRec := doJSONWithCookie(t, handler, http.MethodPost, "/api/v1/devices", map[string]any{
		"name":             "Camera",
		"network_slug":     "iot",
		"include_fallback": true,
	}, token)
	if deviceRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-main device network, got %d", deviceRec.Code)
	}
}

func TestApplyWritesSetupAndDeviceCredentialsToPSKFile(t *testing.T) {
	root := t.TempDir()
	st := store.New(filepath.Join(root, "state.json"))
	if err := st.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := st.SetAdminPassword("test-password"); err != nil {
		t.Fatalf("set admin password: %v", err)
	}
	token, err := st.VerifyAndCreateSession("test-password")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	handler := api.NewHandler(st)
	handler.SetConfigRoot(root)

	deviceRec := doJSONWithCookie(t, handler, http.MethodPost, "/api/v1/devices", map[string]any{
		"name":             "Kitchen camera",
		"network_slug":     "main",
		"include_fallback": true,
	}, token)
	if deviceRec.Code != http.StatusCreated {
		t.Fatalf("device status: %d", deviceRec.Code)
	}
	var created createDeviceResponse
	if err := json.Unmarshal(deviceRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode device response: %v", err)
	}
	if len(created.Credentials) == 0 {
		t.Fatalf("expected generated credentials: %#v", created)
	}

	applyRec := doJSONWithCookie(t, handler, http.MethodPost, "/api/v1/config/apply", map[string]any{}, token)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("apply status: %d", applyRec.Code)
	}

	pskPath := root + store.WpaPskFilePath
	data, err := os.ReadFile(pskPath)
	if err != nil {
		t.Fatalf("read psk file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, st.GetSetupPSK()) {
		t.Fatalf("setup PSK missing from file: %q", content)
	}
	if !strings.Contains(content, created.Credentials[0].Secret) {
		t.Fatalf("generated credential missing from file: %q", content)
	}
}

func TestAuthSessionEndpoints(t *testing.T) {
	_, handler := newHandler(t)

	sessionRec := doJSON(t, handler, http.MethodGet, "/api/v1/auth/session", nil)
	if sessionRec.Code != http.StatusOK {
		t.Fatalf("session status: %d", sessionRec.Code)
	}
	var session store.AdminStatus
	if err := json.Unmarshal(sessionRec.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.PasswordSet {
		t.Fatalf("expected password not set on fresh handler: %#v", session)
	}

	setupRec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/setup", map[string]any{"password": "test-password"})
	if setupRec.Code != http.StatusCreated {
		t.Fatalf("setup status: %d", setupRec.Code)
	}

	sessionAfterRec := doJSON(t, handler, http.MethodGet, "/api/v1/auth/session", nil)
	if sessionAfterRec.Code != http.StatusOK {
		t.Fatalf("session after setup status: %d", sessionAfterRec.Code)
	}
	var sessionAfter store.AdminStatus
	if err := json.Unmarshal(sessionAfterRec.Body.Bytes(), &sessionAfter); err != nil {
		t.Fatalf("decode session after setup: %v", err)
	}
	if !sessionAfter.PasswordSet {
		t.Fatalf("expected password set after setup: %#v", sessionAfter)
	}

	loginRec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "test-password"})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status: %d", loginRec.Code)
	}

	badLoginRec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "wrong"})
	if badLoginRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad password, got %d", badLoginRec.Code)
	}

	logoutRec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/logout", nil)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status: %d", logoutRec.Code)
	}
}

func TestAuthSessionRequiresCookieForAuthenticatedState(t *testing.T) {
	_, handler := newHandler(t)

	setupRec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/setup", map[string]any{"password": "test-password"})
	if setupRec.Code != http.StatusCreated {
		t.Fatalf("setup status: %d", setupRec.Code)
	}

	var sessionToken string
	for _, cookie := range setupRec.Result().Cookies() {
		if cookie.Name == "vantageos_session" {
			sessionToken = cookie.Value
			break
		}
	}
	if sessionToken == "" {
		t.Fatal("expected session cookie after setup")
	}

	noCookieRec := doJSON(t, handler, http.MethodGet, "/api/v1/auth/session", nil)
	if noCookieRec.Code != http.StatusOK {
		t.Fatalf("session (no cookie) status: %d", noCookieRec.Code)
	}
	var noCookieStatus store.AdminStatus
	if err := json.Unmarshal(noCookieRec.Body.Bytes(), &noCookieStatus); err != nil {
		t.Fatalf("decode no-cookie session: %v", err)
	}
	if !noCookieStatus.PasswordSet {
		t.Fatalf("expected password_set=true, got %#v", noCookieStatus)
	}
	if noCookieStatus.Authenticated {
		t.Fatalf("expected authenticated=false without cookie, got %#v", noCookieStatus)
	}

	withCookieRec := doJSONWithCookie(t, handler, http.MethodGet, "/api/v1/auth/session", nil, sessionToken)
	if withCookieRec.Code != http.StatusOK {
		t.Fatalf("session (with cookie) status: %d", withCookieRec.Code)
	}
	var withCookieStatus store.AdminStatus
	if err := json.Unmarshal(withCookieRec.Body.Bytes(), &withCookieStatus); err != nil {
		t.Fatalf("decode with-cookie session: %v", err)
	}
	if !withCookieStatus.Authenticated {
		t.Fatalf("expected authenticated=true with valid cookie, got %#v", withCookieStatus)
	}
}

func TestRecoveryPressFlow(t *testing.T) {
	st, handler := newHandler(t)

	for i := 0; i < 9; i++ {
		pressRec := doJSON(t, handler, http.MethodPost, "/api/v1/recovery/press", nil)
		if pressRec.Code != http.StatusOK {
			t.Fatalf("press %d status: %d", i+1, pressRec.Code)
		}
		var pressResp struct {
			Recovery store.RecoveryState `json:"recovery"`
		}
		if err := json.Unmarshal(pressRec.Body.Bytes(), &pressResp); err != nil {
			t.Fatalf("decode press %d: %v", i+1, err)
		}
		if pressResp.Recovery.PressCount != i+1 {
			t.Fatalf("expected press count %d, got %d", i+1, pressResp.Recovery.PressCount)
		}
	}

	activatesRec := doJSON(t, handler, http.MethodPost, "/api/v1/recovery/press", nil)
	if activatesRec.Code != http.StatusOK {
		t.Fatalf("activating press status: %d", activatesRec.Code)
	}
	var activateResp struct {
		Recovery store.RecoveryState `json:"recovery"`
	}
	if err := json.Unmarshal(activatesRec.Body.Bytes(), &activateResp); err != nil {
		t.Fatalf("decode activating press: %v", err)
	}
	if !activateResp.Recovery.Active {
		t.Fatalf("expected recovery to be active after 10 presses")
	}
	if activateResp.Recovery.Stage != store.RecoveryStageActive {
		t.Fatalf("expected recovery stage active, got %s", activateResp.Recovery.Stage)
	}

	adminStatus := st.AdminStatus()
	if !adminStatus.Recovery.Active {
		t.Fatal("expected recovery active in admin status")
	}

	resetRec := doJSON(t, handler, http.MethodPost, "/api/v1/recovery/reset", map[string]any{"password": "new-password"})
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset status: %d", resetRec.Code)
	}

	adminAfter := st.AdminStatus()
	if adminAfter.Recovery.Active {
		t.Fatal("expected recovery to be inactive after password reset")
	}

	loginRec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]any{"password": "new-password"})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login with new password failed: %d", loginRec.Code)
	}
}

func TestEventStreamEmitsReadyEvent(t *testing.T) {
	_, handler := newHandler(t)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("unexpected content type: %q", got)
	}

	buf := make([]byte, 128)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("read event stream: %v", err)
	}
	payload := string(buf[:n])
	if !strings.Contains(payload, "event: ready") || !strings.Contains(payload, "data: {}") {
		t.Fatalf("unexpected event payload: %q", payload)
	}
}

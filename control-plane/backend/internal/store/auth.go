package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	RequiredPresses       = 10
	TapWindowSeconds      = 20
	RecoveryWindowSeconds = 900
	SessionDuration       = 4 * time.Hour
	HashIterations        = 100000
	HashKeyLen            = 32
	LoginBackoff          = 5 * time.Minute
	MaxFailedLogins       = 5
)

func deriveKey(password, salt string) string {
	key := []byte(password)
	saltBytes := []byte(salt)
	for i := 0; i < HashIterations; i++ {
		mac := hmac.New(sha256.New, key)
		mac.Write(saltBytes)
		key = mac.Sum(nil)
	}
	return hex.EncodeToString(key)
}

func randomSessionToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func randSalt() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func defaultRecoveryState() RecoveryState {
	return RecoveryState{
		Stage:                 RecoveryStageIdle,
		Active:                false,
		RequiredPresses:       RequiredPresses,
		PressWindowSeconds:    TapWindowSeconds,
		RecoveryWindowSeconds: RecoveryWindowSeconds,
	}
}

func (s *Store) AdminStatus() AdminStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := AdminStatus{
		PasswordSet: s.doc.AdminAuth.PasswordHash != "",
		Recovery:    s.doc.Recovery,
	}
	if s.doc.AdminAuth.SessionExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, s.doc.AdminAuth.SessionExpiresAt)
		if err == nil && time.Now().UTC().Before(expiresAt) {
			status.Authenticated = true
			status.SessionExpiresAt = s.doc.AdminAuth.SessionExpiresAt
		}
	}
	status.PasswordUpdatedAt = s.doc.AdminAuth.PasswordUpdatedAt
	return status
}

func (s *Store) SetAdminPassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("password cannot be empty")
	}
	if len(password) < 4 {
		return errors.New("password must be at least 4 characters")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	salt, err := randSalt()
	if err != nil {
		return err
	}
	hash := deriveKey(password, salt)
	s.doc.AdminAuth.PasswordHash = hash + ":" + salt
	s.doc.AdminAuth.PasswordUpdatedAt = nowISO()
	s.doc.AdminAuth.FailedLoginCount = 0

	token, err := randomSessionToken()
	if err != nil {
		return err
	}
	tokenHash := hashToken(token)
	now := time.Now().UTC()
	s.doc.AdminAuth.SessionTokenHash = tokenHash
	s.doc.AdminAuth.SessionCreatedAt = now.Format(time.RFC3339)
	s.doc.AdminAuth.SessionExpiresAt = now.Add(SessionDuration).Format(time.RFC3339)

	s.appendEventLocked("security.password_set", map[string]any{
		"source": "initial_setup",
	})

	if s.doc.Recovery.Active && s.doc.Recovery.Stage == RecoveryStageActive {
		s.doc.Recovery.Active = false
		s.doc.Recovery.Stage = RecoveryStageIdle
		s.doc.Recovery.PressCount = 0
		s.doc.Recovery.ActivatedAt = ""
		s.doc.Recovery.RecoveryExpiresAt = ""
		if s.doc.Recovery.PreviousBootstrapState != "" {
			prev := s.doc.Recovery.PreviousBootstrapState
			s.doc.Recovery.PreviousBootstrapState = ""
			s.transitionBootstrapStateLocked(prev, "password reset completed")
		}
	}

	if err := s.saveLocked(); err != nil {
		return err
	}
	return nil
}

func (s *Store) RecoverPassword(password string) error {
	if !s.doc.Recovery.Active || s.doc.Recovery.Stage != RecoveryStageActive {
		return errors.New("recovery is not active")
	}
	return s.SetAdminPassword(password)
}

func (s *Store) VerifyAndCreateSession(password string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.doc.AdminAuth.PasswordHash == "" {
		return "", errors.New("no password set")
	}

	parts := strings.SplitN(s.doc.AdminAuth.PasswordHash, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("corrupt password hash")
	}
	storedHash, salt := parts[0], parts[1]

	now := time.Now().UTC()

	if s.doc.AdminAuth.FailedLoginCount >= MaxFailedLogins {
		if s.doc.AdminAuth.LastFailedLoginAt != "" {
			lastFailed, err := time.Parse(time.RFC3339, s.doc.AdminAuth.LastFailedLoginAt)
			if err == nil && now.Before(lastFailed.Add(LoginBackoff)) {
				return "", fmt.Errorf("too many failed attempts; try again after %s", lastFailed.Add(LoginBackoff).Format(time.RFC3339))
			}
		}
		s.doc.AdminAuth.FailedLoginCount = 0
	}

	computedHash := deriveKey(password, salt)
	if !hmac.Equal([]byte(computedHash), []byte(storedHash)) {
		s.doc.AdminAuth.FailedLoginCount++
		s.doc.AdminAuth.LastFailedLoginAt = now.Format(time.RFC3339)
		if err := s.saveLocked(); err != nil {
			return "", err
		}
		return "", errors.New("incorrect password")
	}

	s.doc.AdminAuth.FailedLoginCount = 0
	s.doc.AdminAuth.LastFailedLoginAt = ""

	token, err := randomSessionToken()
	if err != nil {
		return "", err
	}
	tokenHash := hashToken(token)
	s.doc.AdminAuth.SessionTokenHash = tokenHash
	s.doc.AdminAuth.SessionCreatedAt = now.Format(time.RFC3339)
	s.doc.AdminAuth.SessionExpiresAt = now.Add(SessionDuration).Format(time.RFC3339)

	s.appendEventLocked("security.login", map[string]any{
		"source": "local_ui",
	})

	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) ValidateSession(token string) bool {
	if token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.doc.AdminAuth.SessionTokenHash == "" || s.doc.AdminAuth.SessionExpiresAt == "" {
		return false
	}

	expiresAt, err := time.Parse(time.RFC3339, s.doc.AdminAuth.SessionExpiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		s.doc.AdminAuth.SessionTokenHash = ""
		s.doc.AdminAuth.SessionCreatedAt = ""
		s.doc.AdminAuth.SessionExpiresAt = ""
		s.saveLocked()
		return false
	}

	tokenHash := hashToken(token)
	return hmac.Equal([]byte(tokenHash), []byte(s.doc.AdminAuth.SessionTokenHash))
}

func (s *Store) ClearSession() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.doc.AdminAuth.SessionTokenHash != "" {
		s.doc.AdminAuth.SessionTokenHash = ""
		s.doc.AdminAuth.SessionCreatedAt = ""
		s.doc.AdminAuth.SessionExpiresAt = ""
		s.appendEventLocked("security.logout", map[string]any{})
		s.saveLocked()
	}
}

func (s *Store) RecordRecoveryPress() RecoveryState {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	r := &s.doc.Recovery

	if r.Stage == RecoveryStageActive && r.RecoveryExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, r.RecoveryExpiresAt)
		if err == nil && now.Before(expiresAt) {
			return *r
		}
		if err == nil {
			r.Stage = RecoveryStageIdle
			r.Active = false
			r.PressCount = 0
			if r.PreviousBootstrapState != "" {
				s.transitionBootstrapStateLocked(r.PreviousBootstrapState, "recovery window expired")
				r.PreviousBootstrapState = ""
			}
			s.appendEventLocked("security.recovery_window_expired", map[string]any{})
			s.saveLocked()
		}
	}

	if r.Stage != RecoveryStageListening || r.PressWindowExpiresAt == "" {
		r.Stage = RecoveryStageListening
		r.PressCount = 1
		r.PressWindowStartedAt = now.Format(time.RFC3339)
		r.PressWindowExpiresAt = now.Add(time.Duration(TapWindowSeconds) * time.Second).Format(time.RFC3339)
		r.LastPressAt = now.Format(time.RFC3339)
		s.appendEventLocked("security.recovery_window_started", map[string]any{
			"press_window_seconds": TapWindowSeconds,
		})
		s.saveLocked()
		return *r
	}

	windowExpiresAt, err := time.Parse(time.RFC3339, r.PressWindowExpiresAt)
	if err != nil || now.After(windowExpiresAt) {
		r.Stage = RecoveryStageListening
		r.PressCount = 1
		r.PressWindowStartedAt = now.Format(time.RFC3339)
		r.PressWindowExpiresAt = now.Add(time.Duration(TapWindowSeconds) * time.Second).Format(time.RFC3339)
		r.LastPressAt = now.Format(time.RFC3339)
		s.appendEventLocked("security.recovery_window_restarted", map[string]any{
			"press_window_seconds": TapWindowSeconds,
		})
		s.saveLocked()
		return *r
	}

	r.PressCount++
	r.LastPressAt = now.Format(time.RFC3339)
	s.appendEventLocked("security.recovery_press", map[string]any{
		"count": r.PressCount,
	})

	if r.PressCount >= RequiredPresses {
		return s.activateRecoveryLocked(RecoveryTriggerPinhole)
	}

	s.saveLocked()
	return *r
}

func (s *Store) activateRecoveryLocked(source string) RecoveryState {
	now := time.Now().UTC()

	r := &s.doc.Recovery
	r.Stage = RecoveryStageActive
	r.Active = true
	r.ActivatedAt = now.Format(time.RFC3339)
	r.RecoveryExpiresAt = now.Add(time.Duration(RecoveryWindowSeconds) * time.Second).Format(time.RFC3339)
	r.TriggerSource = source
	r.PreviousBootstrapState = s.doc.BootstrapState.State

	if s.doc.BootstrapState.State != BootstrapRecovery {
		s.transitionBootstrapStateLocked(BootstrapRecovery, "pinhole button tap sequence completed")
	}

	s.appendEventLocked("security.recovery_activated", map[string]any{
		"source":                source,
		"recovery_window_seconds": RecoveryWindowSeconds,
	})

	s.saveLocked()
	return *r
}

func (s *Store) transitionBootstrapStateLocked(state, reason string) {
	if s.doc.BootstrapState.State == state {
		return
	}
	now := nowISO()
	current := s.doc.BootstrapState
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
	s.doc.Events = append(s.doc.Events, Event{
		Kind:      "bootstrap.state_changed",
		Payload:   mustJSON(map[string]any{"from_state": current.State, "to_state": state, "reason": reason}),
		CreatedAt: now,
	})
}

func (s *Store) RecoveryStatus() RecoveryState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.Recovery
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

package app

import "encoding/json"

// Oper states published under /noports/state/oper-state.
const (
	operDisabled      = "disabled"
	operNotConfigured = "not-configured"
	operAwaitingKeys  = "awaiting-onboarding"
	operRunning       = "running"
	operRetrying      = "retrying"
)

// State mirrors the config-false /noports/state container.
type State struct {
	OperState     string `json:"oper-state,omitempty"`
	Message       string `json:"message,omitempty"`
	PID           uint32 `json:"pid,omitempty"`
	SshnpdVersion string `json:"sshnpd-version,omitempty"`
}

type stateEnvelope struct {
	State *State `json:"state"`
}

// setState publishes operational state into the SR Linux state tree.
// Safe to call from the supervisor goroutine.
func (a *App) setState(operState, message string, pid uint32) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	s := &State{
		OperState:     operState,
		Message:       message,
		PID:           pid,
		SshnpdVersion: a.sshnpdVersion,
	}

	data, err := json.Marshal(stateEnvelope{State: s})
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to marshal state")
		return
	}

	if err := a.agent.UpdateState(AppRoot, string(data)); err != nil {
		a.logger.Error().Err(err).Msg("failed to update state")
	}

	a.logger.Info().
		Str("oper-state", operState).
		Str("message", message).
		Uint32("pid", pid).
		Msg("state updated")
}

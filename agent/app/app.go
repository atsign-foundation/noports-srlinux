package app

import (
	"context"
	"os/exec"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/srl-labs/bond"
)

const (
	AppName = "noports"
	AppRoot = "/" + AppName
)

// App supervises the sshnpd daemon according to the /noports configuration.
type App struct {
	logger *zerolog.Logger
	agent  *bond.Agent

	config        *Config
	version       string
	sshnpdVersion string

	stateMu sync.Mutex

	// cancels the currently running supervisor epoch, if any
	cancelSupervisor context.CancelFunc
	supervisorDone   chan struct{}
}

func New(logger *zerolog.Logger, agent *bond.Agent, version string) *App {
	return &App{
		logger:  logger,
		agent:   agent,
		config:  newConfig(),
		version: version,
	}
}

// Start runs the main loop: react to committed configuration changes.
func (a *App) Start(ctx context.Context) {
	a.sshnpdVersion = probeSshnpdVersion(a.logger)

	for {
		select {
		case <-a.agent.Notifications.FullConfigReceived:
			a.logger.Info().Msg("received full config")
			if !a.loadConfig() {
				continue
			}
			a.applyConfig(ctx)

		case <-ctx.Done():
			a.stopSupervisor()
			return
		}
	}
}

// applyConfig tears down the previous supervisor epoch and starts a new one
// when the app is enabled and fully configured.
func (a *App) applyConfig(ctx context.Context) {
	a.stopSupervisor()

	cfg := a.config

	if !cfg.enabled() {
		a.setState(operDisabled, "admin-state is disable", 0)
		return
	}

	if missing := cfg.missing(); len(missing) > 0 {
		a.setState(operNotConfigured,
			"missing required config: "+strings.Join(missing, ", "), 0)
		return
	}

	// Render NoPorts' own config file from the committed tree; sshnpd is
	// started with --config pointing at it.
	if err := a.renderConfigFile(cfg); err != nil {
		a.setState(operRetrying, "failed to render "+noportsConfigPath+": "+err.Error(), 0)
		return
	}

	epochCtx, cancel := context.WithCancel(ctx)
	a.cancelSupervisor = cancel
	a.supervisorDone = make(chan struct{})

	go a.supervise(epochCtx, cfg, a.supervisorDone)
}

// stopSupervisor cancels the running epoch (killing the sshnpd child) and
// waits for it to wind down.
func (a *App) stopSupervisor() {
	if a.cancelSupervisor == nil {
		return
	}
	a.cancelSupervisor()
	<-a.supervisorDone
	a.cancelSupervisor = nil
	a.supervisorDone = nil
}

func probeSshnpdVersion(logger *zerolog.Logger) string {
	out, err := exec.Command(sshnpdBinPath, "--version").CombinedOutput()
	if err != nil {
		logger.Warn().Err(err).Msg("could not probe sshnpd version")
		return ""
	}
	return strings.TrimSpace(string(out))
}

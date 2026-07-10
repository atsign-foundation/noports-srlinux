package app

import (
	"context"
	"os"
	"os/exec"
	"time"
)

const (
	sshnpdBinPath  = "/usr/local/bin/sshnpd"
	sshnpdHomeDir  = "/var/opt/noports"
	mgmtNetns      = "srbase-mgmt"
	keyPollPeriod  = 15 * time.Second
	restartBackoff = 10 * time.Second
)

// supervise runs sshnpd for the lifetime of one config epoch: it waits for
// APKAM keys to appear (onboarding may not have happened yet), then keeps
// the daemon running, restarting it with backoff if it exits. Cancelling the
// epoch context kills the child and returns.
func (a *App) supervise(ctx context.Context, cfg *Config, done chan struct{}) {
	defer close(done)

	for {
		if _, err := os.Stat(cfg.KeyFile); err != nil {
			a.setState(operAwaitingKeys,
				"atKeys not found at "+cfg.KeyFile+
					"; run /opt/noports/onboard-noports.sh", 0)
			if !sleepCtx(ctx, keyPollPeriod) {
				return
			}
			continue
		}

		// sshnpd must run in the srbase-mgmt namespace: the management VRF
		// (outbound to the atServers) and the local sshd both live there.
		// All settings come from the rendered NoPorts config file.
		args := []string{"netns", "exec", mgmtNetns, sshnpdBinPath,
			"--config", noportsConfigPath,
		}

		if err := os.MkdirAll(sshnpdHomeDir, 0o700); err != nil {
			a.logger.Error().Err(err).Msg("failed to create sshnpd home dir")
		}

		cmd := exec.CommandContext(ctx, "ip", args...)
		// sshnpd keeps local storage under $HOME; give it a persistent home.
		cmd.Env = append(os.Environ(), "HOME="+sshnpdHomeDir)
		// app_mgr collects our stdout/stderr; pass the child's through.
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			a.logger.Error().Err(err).Msg("failed to start sshnpd")
			a.setState(operRetrying, "failed to start sshnpd: "+err.Error(), 0)
			if !sleepCtx(ctx, restartBackoff) {
				return
			}
			continue
		}

		pid := uint32(cmd.Process.Pid)
		a.logger.Info().Uint32("pid", pid).Msg("sshnpd started")
		a.setState(operRunning, "", pid)

		err := cmd.Wait()
		if ctx.Err() != nil {
			// epoch cancelled: config change or shutdown killed the child
			return
		}

		msg := "sshnpd exited"
		if err != nil {
			msg = "sshnpd exited: " + err.Error()
		}
		a.logger.Warn().Msg(msg)
		a.setState(operRetrying, msg, 0)
		if !sleepCtx(ctx, restartBackoff) {
			return
		}
	}
}

// sleepCtx sleeps for d; returns false if ctx was cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

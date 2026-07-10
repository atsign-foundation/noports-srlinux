package app

import "encoding/json"

// Defaults mirrored from the YANG module; kept here because the NDK does
// not deliver YANG defaults in the config JSON.
const (
	defaultRootServer = "root.atsign.org"
	defaultKeyFile    = "/etc/opt/noports/keys/device.atKeys"
)

// Config mirrors the writable nodes of the /noports YANG container as
// delivered by the NDK in JSON encoding. The structure follows the sections
// of NoPorts' own sshnpd.yaml, which the agent renders from this config.
type Config struct {
	AdminState   string        `json:"admin-state,omitempty"`
	DeviceAtsign string        `json:"device-atsign,omitempty"`
	RootServer   string        `json:"root-server,omitempty"`
	KeyFile      string        `json:"key-file,omitempty"`
	Access       AccessConfig  `json:"access"`
	Device       DeviceConfig  `json:"device"`
	SSH          SSHConfig     `json:"ssh"`
	Runtime      RuntimeConfig `json:"runtime"`
}

type AccessConfig struct {
	Managers   []string `json:"managers,omitempty"`
	Policy     string   `json:"policy,omitempty"`
	PermitOpen []string `json:"permit-open,omitempty"`
}

type DeviceConfig struct {
	Name  string `json:"name,omitempty"`
	Group string `json:"group,omitempty"`
}

type SSHConfig struct {
	AddPublicKeys        *bool    `json:"add-public-keys,omitempty"`
	SSHClient            string   `json:"ssh-client,omitempty"`
	SshdPort             *uint16  `json:"sshd-port,omitempty"`
	PublicKeyPermissions []string `json:"public-key-permissions,omitempty"`
}

type RuntimeConfig struct {
	Verbose        *bool `json:"verbose,omitempty"`
	ClearCachedPks bool  `json:"clear-cached-pks,omitempty"`
}

func newConfig() *Config {
	return &Config{
		AdminState: "disable",
		RootServer: defaultRootServer,
		KeyFile:    defaultKeyFile,
	}
}

// loadConfig replaces the app config from the latest full-config
// notification. Returns false if unmarshalling failed.
func (a *App) loadConfig() bool {
	cfg := newConfig()

	if a.agent.Notifications.FullConfig != nil {
		if err := json.Unmarshal(a.agent.Notifications.FullConfig, cfg); err != nil {
			a.logger.Error().Err(err).Msg("failed to unmarshal config")
			return false
		}
	}
	if cfg.RootServer == "" {
		cfg.RootServer = defaultRootServer
	}
	if cfg.KeyFile == "" {
		cfg.KeyFile = defaultKeyFile
	}

	a.config = cfg
	return true
}

// missing returns the names of required leaves that are not configured.
func (c *Config) missing() []string {
	var m []string
	if c.DeviceAtsign == "" {
		m = append(m, "device-atsign")
	}
	if len(c.Access.Managers) == 0 && c.Access.Policy == "" {
		m = append(m, "access managers (or access policy)")
	}
	if c.Device.Name == "" {
		m = append(m, "device name")
	}
	return m
}

func (c *Config) enabled() bool {
	return c.AdminState == "enable"
}

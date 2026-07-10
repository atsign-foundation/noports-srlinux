package app

import "encoding/json"

// Defaults mirrored from the YANG module; kept here because the NDK does
// not deliver YANG defaults in the config JSON.
const (
	defaultRootServer = "root.atsign.org"
	defaultKeyFile    = "/etc/opt/noports/keys/device.atKeys"
)

// Config mirrors the writable leaves of the /noports YANG container as
// delivered by the NDK in JSON encoding.
type Config struct {
	AdminState           string   `json:"admin-state,omitempty"`
	DeviceAtsign         string   `json:"device-atsign,omitempty"`
	ManagerAtsigns       []string `json:"manager-atsigns,omitempty"`
	DeviceName           string   `json:"device-name,omitempty"`
	RootServer           string   `json:"root-server,omitempty"`
	KeyFile              string   `json:"key-file,omitempty"`
	PermitOpen           string   `json:"permit-open,omitempty"`
	ManageAuthorizedKeys *bool    `json:"manage-authorized-keys,omitempty"`
	Verbose              bool     `json:"verbose,omitempty"`
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
	if len(c.ManagerAtsigns) == 0 {
		m = append(m, "manager-atsigns")
	}
	if c.DeviceName == "" {
		m = append(m, "device-name")
	}
	return m
}

func (c *Config) enabled() bool {
	return c.AdminState == "enable"
}

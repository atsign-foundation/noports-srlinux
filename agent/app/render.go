package app

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// noportsConfigPath is where the agent renders NoPorts' own config file.
// sshnpd is started with --config pointing here; operators never edit it —
// the SR Linux config tree (/noports) is the source of truth.
const noportsConfigPath = "/etc/opt/noports/sshnpd.yaml"

// sshnpdYaml mirrors the schema of NoPorts' sshnpd.yaml config file.
type sshnpdYaml struct {
	Atsign struct {
		Atsign string `yaml:"atsign"`
		Keys   string `yaml:"keys"`
		Root   string `yaml:"root"`
	} `yaml:"atsign"`
	Access struct {
		Policy     string   `yaml:"policy,omitempty"`
		Managers   []string `yaml:"managers,omitempty"`
		Permitopen []string `yaml:"permitopen,omitempty"`
	} `yaml:"access"`
	Device struct {
		Name  string `yaml:"name"`
		Group string `yaml:"group,omitempty"`
	} `yaml:"device"`
	SSH struct {
		AddPublickeys        bool     `yaml:"add-publickeys"`
		SSHClient            string   `yaml:"ssh-client"`
		SshdPort             uint16   `yaml:"sshd-port"`
		PublickeyPermissions []string `yaml:"publickey-permissions,omitempty"`
	} `yaml:"ssh"`
	Runtime struct {
		StoragePath    string `yaml:"storage-path"`
		ClearCachedPks bool   `yaml:"clear-cached-pks"`
		Verbose        bool   `yaml:"verbose"`
	} `yaml:"runtime"`
}

// renderConfigFile writes the sshnpd.yaml derived from the committed
// /noports configuration. Rendered on every commit (even before onboarding)
// so the file on disk always reflects the config tree.
func (a *App) renderConfigFile(cfg *Config) error {
	var y sshnpdYaml

	y.Atsign.Atsign = cfg.DeviceAtsign
	y.Atsign.Keys = cfg.KeyFile
	y.Atsign.Root = cfg.RootServer

	y.Access.Policy = cfg.Access.Policy
	y.Access.Managers = cfg.Access.Managers
	y.Access.Permitopen = cfg.Access.PermitOpen

	y.Device.Name = cfg.Device.Name
	y.Device.Group = cfg.Device.Group

	// YANG defaults are not delivered in the config JSON; apply them here.
	y.SSH.AddPublickeys = cfg.SSH.AddPublicKeys == nil || *cfg.SSH.AddPublicKeys
	y.SSH.SSHClient = cfg.SSH.SSHClient
	if y.SSH.SSHClient == "" {
		y.SSH.SSHClient = "openssh"
	}
	y.SSH.SshdPort = 22
	if cfg.SSH.SshdPort != nil {
		y.SSH.SshdPort = *cfg.SSH.SshdPort
	}
	y.SSH.PublickeyPermissions = cfg.SSH.PublicKeyPermissions

	y.Runtime.StoragePath = sshnpdHomeDir + "/storage"
	y.Runtime.ClearCachedPks = cfg.Runtime.ClearCachedPks
	y.Runtime.Verbose = cfg.Runtime.Verbose == nil || *cfg.Runtime.Verbose

	data, err := yaml.Marshal(&y)
	if err != nil {
		return err
	}

	header := []byte(
		"# Rendered by noports-agent from the SR Linux config tree (/noports).\n" +
			"# Do not edit: changes are overwritten on every commit.\n")

	if err := os.MkdirAll(filepath.Dir(noportsConfigPath), 0o755); err != nil {
		return err
	}
	tmp := noportsConfigPath + ".tmp"
	if err := os.WriteFile(tmp, append(header, data...), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, noportsConfigPath)
}

# SR Linux CLI reference for NoPorts

Every command this integration adds to or uses on the SR Linux CLI. All
configuration lives under `/ noports` (modeled by
[yang/noports.yang](../yang/noports.yang)); on every `commit` the agent
renders it to `/etc/opt/noports/sshnpd.yaml` and (re)starts `sshnpd` with
the new settings. Everything here works identically over gNMI and JSON-RPC.

Configuration changes follow the normal SR Linux pattern:

```text
enter candidate
set / noports <path> <value>
commit now          # or: commit stay / commit confirmed
save startup        # persist across reboot and app_mgr reload
```

## Configuration: top level

| Command | Type / values | Default | Effect |
|---|---|---|---|
| `set / noports admin-state <enable\|disable>` | enumeration | `disable` | Master switch. `enable` requires `device-atsign`, `device name`, and at least one of `access managers` / `access policy`. `disable` stops sshnpd. |
| `set / noports device-atsign @<atsign>` | string, must start `@` | — | The Atsign identifying this router (its identity on the atProtocol). |
| `set / noports root-server <host[:port]>` | string | `root.atsign.org` | atDirectory server. Use `proxy:<host>:443` to send **all** atProtocol control-plane traffic to one reverse proxy on one port (restricted egress; honored by onboarding too). |
| `set / noports key-file <path>` | string | `/etc/opt/noports/keys/device.atKeys` | Where the APKAM atKeys live. The onboarding script writes here; the agent polls for this file when enabled. |

## Configuration: `access` — who may connect, and to what

| Command | Type / values | Default | Effect |
|---|---|---|---|
| `set / noports access managers [ @a @b ]` | leaf-list of Atsigns | — | Atsigns with direct access. When `policy` is also set, managers bypass the policy check (break-glass list). |
| `set / noports access policy @<atsign>` | Atsign | — | Delegate access decisions to a NoPorts Policy Service — the fleet-scale alternative to per-router manager lists. |
| `set / noports access permit-open [ host:port ... ]` | leaf-list | NoPorts defaults¹ | host:port pairs `npt` clients may reach, e.g. `localhost:22 localhost:57400` for SSH + gNMI. |

¹ Empty means NoPorts' own defaults: `localhost:22,localhost:3389` without
a policy; `*:*` (defer to policy) when `access policy` is set.

## Configuration: `device` — identity seen by clients

| Command | Type / values | Default | Effect |
|---|---|---|---|
| `set / noports device name <name>` | `[a-zA-Z0-9_]{1,36}` | — | The name clients use: `sshnp ... -d <name>`. |
| `set / noports device group <name>` | string | — | Sent to the policy service with each request so rules can target groups (e.g. `core-routers`) instead of devices. |

## Configuration: `ssh` — how sessions reach the local sshd

| Command | Type / values | Default | Effect |
|---|---|---|---|
| `set / noports ssh add-public-keys <true\|false>` | boolean | `true` | Add manager-sent public keys to the daemon user's `authorized_keys`. NoPorts recommends `false` in enterprise/multi-admin settings. |
| `set / noports ssh ssh-client <openssh\|dart>` | enumeration | `openssh` | Client used for outbound ssh connections. |
| `set / noports ssh sshd-port <port>` | uint16 | `22` | Port the local sshd listens on. |
| `set / noports ssh public-key-permissions [ ... ]` | leaf-list | — | Options prefixed to `authorized_keys` entries added via `add-public-keys` (see sshd AUTHORIZED_KEYS FILE FORMAT). |

## Configuration: `runtime`

| Command | Type / values | Default | Effect |
|---|---|---|---|
| `set / noports runtime verbose <true\|false>` | boolean | `true` | INFO-level sshnpd logging. |
| `set / noports runtime clear-cached-pks <true\|false>` | boolean | `false` | Clear cached public keys on next start — use after resetting an Atsign. |

## Inspecting configuration and state

| Command | Shows |
|---|---|
| `info / noports` | Committed configuration as a tree (running mode). |
| `info flat / noports` | Same, as `set` commands — copy-paste to another router. |
| `info from state / noports` | Configuration **plus** operational state. |
| `info from state / noports state` | Operational state only (see below). |
| `info from running / noports \| as json` | JSON encoding (also available via gNMI Get). |

The `state` container (config false, published by the agent, streamable
over gNMI telemetry):

| Leaf | Meaning |
|---|---|
| `oper-state` | `disabled` \| `not-configured` \| `awaiting-onboarding` \| `running` \| `retrying` |
| `message` | Human-readable detail (e.g. which required leaves are missing, or why sshnpd exited). |
| `pid` | PID of the running sshnpd process. |
| `sshnpd-version` | Version reported by the packaged sshnpd binary. |

`oper-state` lifecycle: `disabled` → (`admin-state enable`) →
`not-configured` (missing required leaves) → `awaiting-onboarding` (no
atKeys yet; the agent polls every 15 s) → `running` → `retrying` (sshnpd
exited; restart with 10 s backoff).

## Application lifecycle (app_mgr)

The agent is a normal SR Linux application; sshnpd is its supervised child
(kill the app and the daemon dies with it).

| Command | Effect |
|---|---|
| `show system application noports` | App status: PID, state, version. |
| `tools system app-management application noports stop` | Stop agent + sshnpd. |
| `tools system app-management application noports start` | Start it. |
| `tools system app-management application noports restart` | Bounce it (config is re-fetched on start). |
| `tools system app-management application app_mgr reload` | Re-scan `/etc/opt/srlinux/appmgr/` — needed once after installing/removing the deb. **Replays the startup config**: `save startup` first. |

## Shell-side paths (from `bash`)

| Path / command | Purpose |
|---|---|
| `sudo /opt/noports/onboard-noports.sh <passcode>` | One-time APKAM enrollment; reads Atsign/device/root-server from the running config. |
| `/etc/opt/noports/sshnpd.yaml` | The NoPorts config file the agent renders on every commit — **read-only for humans**; change `/ noports` and commit instead. |
| `/etc/opt/noports/keys/` | APKAM atKeys (created by onboarding). Delete + `at_activate revoke` + re-onboard to re-enroll. |
| `/var/log/srlinux/stdout/noports.log` | Agent and sshnpd output (rotated as `noports.<timestamp>.log`). |
| `/var/opt/noports/` | Persistent daemon home: atProtocol storage cache, session `authorized_keys`. |

## Recipes

Enable with policy-based access (fleet):

```text
enter candidate
set / noports device-atsign @mydevice
set / noports access policy @policy_np
set / noports device name srl-1
set / noports device group core-routers
set / noports admin-state enable
commit now
save startup
```

Restricted egress (only 80/443 outbound — verified end-to-end):

```text
set / noports root-server proxy:proxy0001.atsign.org:443
```

client side:

```bash
sshnp -f @manager -r @rv_oc -t @mydevice -d srl-1 -u admin \
  --443 --relay-auth-mode escr \
  --root-domain "proxy:proxy0001.atsign.org:443"
```

Temporarily disable NoPorts access:

```text
set / noports admin-state disable ; commit now (two commands)
```

Rotate to a new manager Atsign:

```text
enter candidate
set / noports access managers [ @newmanager ]
commit now
```

The agent restarts sshnpd with the new access list within seconds — no
shell access needed, and the change is one `commit confirmed` away from
being safely revertible.

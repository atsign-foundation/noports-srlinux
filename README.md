<!-- pyml disable-num-lines 4 md013,md033-->
<h1><a href="https://atsign.com#gh-light-mode-only">
   <img width=250px src="https://atsign.com/wp-content/uploads/2022/05/atsign-logo-horizontal-color2022.svg#gh-light-mode-only" alt="The Atsign Foundation"></a>
<a href="https://atsign.com#gh-dark-mode-only">
   <img width=250px src="https://atsign.com/wp-content/uploads/2023/08/atsign-logo-horizontal-reverse2022-Color.svg#gh-dark-mode-only" alt="The Atsign Foundation"></a></h1>

# NoPorts for Nokia SR Linux

Open with intent - we welcome contributions - we want pull requests and to
hear about issues.

[NoPorts](https://docs.noports.com) as a **native feature** of Nokia
[SR Linux](https://learn.srlinux.dev): configured from the router's own
CLI/gNMI config tree, supervised by an
[NDK](https://learn.srlinux.dev/ndk/) agent, publishing operational state
into `info from state` — and giving operators SSH (plus, via `npt`,
gNMI/JSON-RPC) access to the router with **no inbound listening ports** on
the management plane.

```text
--{ running }--[  ]--
A:srl1# enter candidate
A:srl1# set / noports device-atsign @mydevice
A:srl1# set / noports manager-atsigns [ @manager ]
A:srl1# set / noports device-name srl-router-1
A:srl1# set / noports admin-state enable
A:srl1# commit now
A:srl1# info from state / noports state
    state {
        oper-state awaiting-onboarding
        message "atKeys not found at /etc/opt/noports/keys/device.atKeys; run /opt/noports/onboard-noports.sh"
    }
```

**New here? Start with the [Quickstart](QUICKSTART.md)** — virtual lab to
SSH session, no hardware required.

## Who is this for?

### Network operators

Install the `.deb` from the
[releases page](https://github.com/atsign-foundation/noports-srlinux/releases),
configure `/noports` from the CLI, onboard with a one-time passcode. You
will need NoPorts atSigns for your devices; start at
[noports.com](https://noports.com).

### Contributors

[CONTRIBUTING.md](CONTRIBUTING.md) has the general guidance. Everything in
this repo can be developed and tested without router hardware using the free
SR Linux container image and [containerlab](https://containerlab.dev) — see
[Development](#development).

## How it works

`noports-agent` is a Go [NDK](https://learn.srlinux.dev/ndk/) application
built on [srl-labs/bond](https://github.com/srl-labs/bond). It registers
with the NDK, receives the `/noports` configuration (modeled by
[yang/noports.yang](yang/noports.yang)) as JSON on every commit, and
supervises the stock `sshnpd` binary: starting it inside the `srbase-mgmt`
network namespace (where the management VRF and the local `sshd` live),
restarting it with backoff if it exits, and publishing
`oper-state`/`pid`/`sshnpd-version` back into the state tree.

| Piece | On-router path | Purpose |
|---|---|---|
| `noports-agent` | `/usr/local/bin/noports-agent` | NDK agent: config → sshnpd lifecycle → state |
| `sshnpd` | `/usr/local/bin/sshnpd` | Stock NoPorts release binary (x86_64) |
| `at_activate` | `/usr/local/bin/at_activate` | APKAM enrollment (cuts keys on the router) |
| [`appmgr/noports.yml`](appmgr/noports.yml) | `/etc/opt/srlinux/appmgr/noports.yml` | Registers the agent with `app_mgr` |
| [`yang/noports.yang`](yang/noports.yang) | `/opt/noports/yang/` | Models `/noports` config + state |
| [`onboard-noports.sh`](opt/noports/onboard-noports.sh) | `/opt/noports/onboard-noports.sh` | One-time APKAM device enrollment |
| APKAM atKeys | `/etc/opt/noports/keys/` | Device identity, created by enrollment |

Because the configuration lives in the router's config tree, it persists in
the startup config, replays on reboot, streams over gNMI telemetry, and
works from any management interface (CLI, gNMI, JSON-RPC) — no environment
files, no hand-managed daemons.

Since release 24.3.1 SR Linux is Debian-based, so the deliverable is a
single `.deb` built with [nFPM](https://nfpm.goreleaser.com/). The pinned
sshnpd release lives in [SSHNPD_VERSION](SSHNPD_VERSION) and is bumped
automatically by CI when NoPorts publishes a new release.

## Installation

Grab the `.deb` from the
[releases page](https://github.com/atsign-foundation/noports-srlinux/releases)
(or build it yourself: `make fetch agent deb`), then on the router:

```bash
# from the SR Linux CLI, drop to the Linux shell with `bash`
sudo dpkg -i noports-srlinux_*.deb   # postinstall reloads app_mgr
```

Configure from the SR Linux CLI:

```text
enter candidate
set / noports device-atsign @mydevice
set / noports manager-atsigns [ @manager ]
set / noports device-name srl-router-1
set / noports admin-state enable
commit now
```

### Onboard the device with APKAM (no atKeys files copied around)

Enrollment cuts new, scope-limited APKAM keys **on the router**; the full
atKeys file for the device atSign never leaves the administrator's custody.

On the admin machine (any host with an authorized key for `@mydevice`):

```bash
at_activate otp -a @mydevice
```

On the router (bash shell):

```bash
sudo /opt/noports/onboard-noports.sh <passcode>
```

While it waits, approve from the admin machine:

```bash
at_activate approve -a @mydevice --arx noports --drx srl-router-1
```

The agent detects the new keys within ~15 seconds and starts sshnpd —
`info from state / noports state` shows `oper-state running`.

### Connect from anywhere

```bash
sshnp -f @manager -t @mydevice -d srl-router-1 -u admin
# or tunnel gNMI without SSH (requires permit-open config):
npt -f @manager -t @mydevice -d srl-router-1 -r localhost -p 57400 -l 57400
```

## Restricted egress (management-plane ACLs)

By default the atProtocol dials the atDirectory on `root.atsign.org:64100`
and atServers on assorted high ports — typically blocked by management VRF
ACLs. The `proxy:` root-server form skips the directory lookup and sends
**all** atProtocol traffic to one reverse proxy on one port:

```text
set / noports root-server proxy:proxy0001.atsign.org:443
```

Both the daemon and APKAM enrollment honor it. Clients use the equivalent
flag, picking a relay with `-r`:

```bash
sshnp -f @manager -r @rv_oc -t @mydevice -d srl-router-1 \
  --root-domain "proxy:proxy0001.atsign.org:443"
```

Note: the proxy covers atProtocol (control-plane) traffic. The session data
path is a separate outbound connection from the router to the relay chosen
by the client (`-r`), so a 443-only egress policy also needs a relay
reachable on 443.

## Development

No hardware needed — SR Linux ships as a free public container image:

```bash
make fetch                       # stage pinned sshnpd + at_activate in build/
make agent                       # build noports-agent (Docker, no local Go)
make deb                         # build the .deb
make lab                         # containerlab deploy (Linux host)
ssh admin@clab-noports-srl-srl1  # password: NokiaSrl1!
```

The dev topology ([clab/noports-srl.clab.yml](clab/noports-srl.clab.yml))
bind-mounts the repo's artifacts into the node for fast iteration; CI uses
the minimal [clab/ci.clab.yml](clab/ci.clab.yml), installs the built deb,
configures `/noports` via the CLI and asserts the agent publishes state —
proving the whole NDK round-trip on every PR.

`make lint` runs shellcheck and pyang locally (matching CI); agent code is
checked with `go vet` and `gofmt`.

## Roadmap

- **Done:** NDK agent with CLI/gNMI config and state, APKAM on-router
  enrollment, proxy-mode (443-only) egress, deb packaging, containerlab
  smoke test in CI, automated upstream sshnpd bumps.
- **Next:** hardware validation (7220/7250) and an end-to-end lab guide with
  real atSigns; submission to the
  [NDK apps catalog](https://learn.srlinux.dev/ndk/apps/).
- **Later:** fleet onboarding at scale (SPP passcodes + `at_activate auto`
  approval); richer state (session counters, last-seen); arm64 package;
  relay-on-443 guidance for fully locked-down egress.

## Maintainers

Created by Atsign. Original author:
[Colin Constable](https://github.com/cconstab) ([@colin](https://atsign.com)).
Issues and pull requests are welcome — they are triaged weekly.

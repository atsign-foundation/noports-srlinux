<!-- pyml disable-num-lines 4 md013,md033-->
<h1><a href="https://atsign.com#gh-light-mode-only">
   <img width=250px src="https://atsign.com/wp-content/uploads/2022/05/atsign-logo-horizontal-color2022.svg#gh-light-mode-only" alt="The Atsign Foundation"></a>
<a href="https://atsign.com#gh-dark-mode-only">
   <img width=250px src="https://atsign.com/wp-content/uploads/2023/08/atsign-logo-horizontal-reverse2022-Color.svg#gh-dark-mode-only" alt="The Atsign Foundation"></a></h1>

# NoPorts for Nokia SR Linux

Open with intent - we welcome contributions - we want pull requests and to
hear about issues.

Run the [NoPorts](https://docs.noports.com) device daemon (`sshnpd`) on a
Nokia [SR Linux](https://learn.srlinux.dev) router as a first-class,
`app_mgr`-managed application, giving operators SSH — and, via `npt`,
gNMI/JSON-RPC — access to the router with **no inbound listening ports** on
the management plane.

**New here? Start with the [Quickstart](QUICKSTART.md)** — virtual lab to
SSH session in about ten minutes, no hardware required.

## Who is this for?

### Network operators

You want to reach your SR Linux routers without exposing SSH or gNMI on the
management VRF. Install the `.deb` from the
[releases page](https://github.com/atsign-foundation/noports-srlinux/releases)
— see [Installation](#installation) below. You will need NoPorts atSigns for
your devices; start at [noports.com](https://noports.com).

### Contributors

[CONTRIBUTING.md](CONTRIBUTING.md) has the general guidance. Everything in
this repo can be developed and tested without router hardware using the free
SR Linux container image and [containerlab](https://containerlab.dev) — see
[Development](#development).

## How it works

SR Linux's Application Manager (`app_mgr`) can supervise any Linux
executable — the full NDK gRPC machinery is only needed for deep
integrations (RIB/FIB access, custom telemetry). NoPorts needs none of that,
so the integration is deliberately thin:

| Piece | On-router path | Purpose |
|---|---|---|
| `sshnpd` binary | `/usr/local/bin/sshnpd` | Stock NoPorts release binary (x86_64) |
| `at_activate` binary | `/usr/local/bin/at_activate` | APKAM enrollment (cuts keys on the router) |
| [`appmgr/sshnpd.yml`](appmgr/sshnpd.yml) | `/etc/opt/srlinux/appmgr/sshnpd.yml` | Registers the app with `app_mgr` |
| [`opt/sshnpd/run-sshnpd.sh`](opt/sshnpd/run-sshnpd.sh) | `/opt/sshnpd/run-sshnpd.sh` | Launcher: loads env, enters mgmt netns |
| [`opt/sshnpd/onboard-sshnpd.sh`](opt/sshnpd/onboard-sshnpd.sh) | `/opt/sshnpd/onboard-sshnpd.sh` | One-time APKAM device enrollment |
| [`etc/sshnpd.env.example`](etc/sshnpd.env.example) | `/etc/opt/sshnpd/sshnpd.env` | Per-device settings (atSigns, device name) |
| APKAM atKeys | `/etc/opt/sshnpd/keys/` | Device identity, created by enrollment |

The one SR Linux-specific trick: the management VRF and the local `sshd`
live in the `srbase-mgmt` network namespace, while `app_mgr` launches apps
in the default namespace. The launcher therefore wraps the daemon in
`ip netns exec srbase-mgmt ...` so it can reach the atDirectory/atServers
outbound *and* connect to the local sshd.

Since release 24.3.1 SR Linux is Debian-based, so the deliverable is a
single `.deb` built with [nFPM](https://nfpm.goreleaser.com/). The pinned
sshnpd release lives in [SSHNPD_VERSION](SSHNPD_VERSION) and is bumped
automatically by CI when NoPorts publishes a new release.

## Installation

Grab the `.deb` from the
[releases page](https://github.com/atsign-foundation/noports-srlinux/releases)
(or build it yourself: `make deb`), then on the router:

```bash
# from the SR Linux CLI, drop to the Linux shell with `bash`
sudo dpkg -i noports-srlinux_*.deb
sudo cp /etc/opt/sshnpd/sshnpd.env.example /etc/opt/sshnpd/sshnpd.env
sudo vi /etc/opt/sshnpd/sshnpd.env    # set device/manager atSigns, device name
```

### Onboard the device with APKAM (no atKeys files copied around)

Enrollment cuts new, scope-limited APKAM keys **on the router**; the full
atKeys file for the device atSign never leaves the administrator's custody.

On the admin machine (any host with an authorized key for `@mydevice`),
generate a one-time passcode:

```bash
at_activate otp -a @mydevice
```

On the router:

```bash
sudo /opt/sshnpd/onboard-sshnpd.sh <passcode>
```

While it waits, approve the request from the admin machine:

```bash
at_activate approve -a @mydevice --arx sshnpd --drx srl-router-1
```

The APKAM keys land in `/etc/opt/sshnpd/keys/` and the daemon can start.

### Register and start from the SR Linux CLI

```text
tools system app-management application app_mgr reload
show system application sshnpd
tools system app-management application sshnpd start
```

### Connect from anywhere

```bash
sshnp -f @manager -t @mydevice -d srl-router-1 -u admin
# or tunnel gNMI without SSH:
npt -f @manager -t @mydevice -d srl-router-1 -r localhost -p 57400 -l 57400
```

## Restricted egress (management-plane ACLs)

By default the atProtocol dials the atDirectory on `root.atsign.org:64100`
and atServers on assorted high ports — typically blocked by management VRF
ACLs. Both `sshnpd` and `at_activate` accept a `proxy:` root server, which
skips the directory lookup and sends **all** atProtocol traffic to one
reverse proxy on one port. In `/etc/opt/sshnpd/sshnpd.env`:

```bash
ROOT_SERVER="proxy:proxy0001.atsign.org:443"
```

Both the launcher and the onboarding script honor it. Clients use the
equivalent flag, picking a relay with `-r`:

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
make fetch                       # stage the pinned sshnpd binary in build/
make deb                         # build the .deb (Docker + nFPM)
mkdir -p clab/secrets            # put sshnpd.env + atKeys here (gitignored)
make lab                         # containerlab deploy (Linux host)
ssh admin@clab-noports-srl-srl1  # password: NokiaSrl1!
```

The dev topology ([clab/noports-srl.clab.yml](clab/noports-srl.clab.yml))
bind-mounts this repo's artifacts into the node for fast iteration; CI uses
the minimal [clab/ci.clab.yml](clab/ci.clab.yml) and installs the built deb
instead.

`make lint` runs shellcheck and pyang locally (matching CI).

## Roadmap

- **Phase 1 (current):** sshnpd as an `app_mgr`-managed app, env-file
  configured, packaged as a deb, containerlab smoke test in CI, APKAM
  on-router enrollment, proxy-mode (443-only) egress support.
- **Phase 2:** native CLI/gNMI configuration. The
  [yang/noports-sshnpd.yang](yang/noports-sshnpd.yang) module ships in the
  deb but is **not yet active** — today all configuration is via the env
  file, and the SR Linux CLI only provides app lifecycle commands
  (`tools system app-management ...`). Activating it means uncommenting the
  `yang-modules` section of the app_mgr yml **and** teaching the app to
  consume config delivered by app_mgr (`wait-for-config` plus a config shim
  or a thin [Python NDK agent](https://github.com/nokia/srlinux-ndk-py)).
  Done, it makes NoPorts part of the router config tree
  (`set / sshnpd device-atsign @mydevice`, persisted in startup config,
  streamable over telemetry) and publishes `oper-state` into `show` output.
  Then: submit to the [NDK apps catalog](https://learn.srlinux.dev/ndk/apps/).
- **Phase 3:** fleet onboarding at scale (SPP passcodes + `at_activate auto`
  approval); `npt` port policy for gNMI/JSON-RPC tunneling; arm64 package;
  relay-on-443 guidance for fully locked-down egress.

## Maintainers

Created by Atsign. Original author:
[Colin Constable](https://github.com/cconstab) ([@colin](https://atsign.com)).
Issues and pull requests are welcome — they are triaged weekly.

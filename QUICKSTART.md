# Quickstart

Zero to an SSH session with no open ports — two paths: a free virtual lab
(no hardware) or a real SR Linux router.

## What you need first

- Two atSigns: one for the router (e.g. `@mydevice`) and one for you as the
  manager (e.g. `@manager`) — get them at [noports.com](https://noports.com)
- The manager atSign activated on your own machine, with the NoPorts client
  installed ([client install guide](https://docs.noports.com))
- For the lab path: a Linux host with Docker and
  [containerlab](https://containerlab.dev/install/)

## Path A: virtual lab (containerlab)

```bash
git clone https://github.com/atsign-foundation/noports-srlinux.git
cd noports-srlinux
make fetch agent                  # stage sshnpd, at_activate, noports-agent
mkdir -p clab/secrets
make lab                          # deploys the SR Linux node
```

Configure NoPorts from the SR Linux CLI:

```bash
ssh admin@clab-noports-srl-srl1   # password: NokiaSrl1!
```

```text
enter candidate
set / noports device-atsign @mydevice
set / noports access managers [ @manager ]
set / noports device name srl-router-1
set / noports admin-state enable
commit now
info from state / noports state   # expect: oper-state awaiting-onboarding
```

Onboard the (virtual) router with APKAM:

```bash
# on your machine: generate a one-time passcode for the device atSign
at_activate otp -a @mydevice

# on the SR Linux node: drop to the shell with `bash`, then
sudo /opt/noports/onboard-noports.sh <passcode>

# back on your machine, while the router waits: approve the enrollment
at_activate approve -a @mydevice --arx noports --drx srl-router-1
```

The agent detects the keys within ~15 seconds and starts sshnpd:

```text
info from state / noports state   # expect: oper-state running, pid, version
```

Connect — from your machine, anywhere on the internet:

```bash
sshnp -f @manager -t @mydevice -d srl-router-1 -u admin
```

Bonus — tunnel gNMI without SSH (add `localhost:57400` to permit-open
first: `set / noports access permit-open [ localhost:22 localhost:57400 ]`):

```bash
npt -f @manager -t @mydevice -d srl-router-1 -r localhost -p 57400 -l 57400
gnmic -a localhost:57400 -u admin --skip-verify capabilities
```

## Path B: real router

Download the `.deb` from the
[releases page](https://github.com/atsign-foundation/noports-srlinux/releases),
then:

```bash
scp noports-srlinux_*.deb admin@<router>:/tmp/
ssh admin@<router>
bash
sudo dpkg -i /tmp/noports-srlinux_*.deb    # postinstall reloads app_mgr
```

Then configure `/noports`, onboard, and connect exactly as in Path A.

### Management-plane ACLs block outbound?

Set proxy mode **before** onboarding, so enrollment traffic uses it too:

```text
set / noports root-server proxy:proxy0001.atsign.org:443
```

and connect with the matching client flags — `--443` keeps the session
data path (relay) on port 443 too:

```bash
sshnp -f @manager -r @rv_oc -t @mydevice -d srl-router-1 -u admin \
  --443 --relay-auth-mode escr \
  --root-domain "proxy:proxy0001.atsign.org:443"
```

## Path C: standalone SR Linux in plain Docker (no containerlab)

A single SR Linux node in plain Docker is enough to exercise the whole
integration. Two things containerlab normally does for you must happen
another way:

1. **Config must be present at boot** — SR Linux only adopts the
   container's `eth0` as `mgmt0` during startup, so runtime commits can't
   bootstrap management networking. The repo ships
   [docker/standalone-config.json](docker/standalone-config.json), which
   enables `mgmt0` (DHCP), DNS, the **NDK server** and the
   **`insecure-mgmt` gRPC server with its unix socket** — the two services
   the noports agent depends on (`sr_sdk_service_manager` for NDK
   registration, `sr_grpc_server_insecure-mgmt` for config retrieval).
2. **The host must run real Linux.** On macOS, Docker Desktop's LinuxKit
   kernel breaks SR Linux's management-namespace plumbing
   (`net_inst_mgr` crashes creating the mgmt gateway veth — observed on
   SR Linux 25.7.1 and 26.3.3). Use [OrbStack](https://orbstack.dev) or a
   Linux VM instead; any x86_64/arm64 Linux host with Docker works.

```bash
# 1. build the deb for the host architecture (arm64 shown; default amd64)
make fetch agent deb ARCH=arm64

# 2. boot SR Linux with the bootstrap config (image is multi-arch).
# Mount a DIRECTORY over /etc/opt/srlinux (containerlab does the same):
# a single-file mount of config.json makes `save startup` fail, because
# SR Linux saves by atomically renaming a temp file over config.json.
mkdir -p /tmp/srl-standalone
cp docker/standalone-config.json /tmp/srl-standalone/config.json
docker run -t -d --rm --privileged -u 0:0 -e SRLINUX=1 \
  -v /tmp/srl-standalone:/etc/opt/srlinux \
  --name srl ghcr.io/nokia/srlinux:latest \
  sudo -E bash -c 'touch /.dockerenv && /opt/srlinux/bin/sr_linux'

# 3. install (postinstall reloads app_mgr; startup config replays cleanly)
docker cp build/noports-srlinux_*_*.deb srl:/tmp/noports.deb
docker exec -u root srl dpkg -i /tmp/noports.deb

# 4. give the mgmt namespace a default route — docker networks don't
#    provide one via DHCP, and SR Linux static routes are not exported
#    to the kernel namespace that sshnpd/at_activate route through
GW=$(docker network inspect bridge --format '{{(index .IPAM.Config 0).Gateway}}')
docker exec srl ip netns exec srbase-mgmt ip route add default via "$GW"

# 5. configure from the CLI as usual, then persist
docker exec -it srl sr_cli
#   enter candidate / set / noports ... / commit now / save startup

# 6. onboard (APKAM) and connect, as in Path A
```

Note: `save startup` after configuring — an `app_mgr reload` replays the
*startup* config, so unsaved running config is lost when apps are
(re)installed.

## Troubleshooting

| Symptom | Check |
|---|---|
| `oper-state not-configured` | The `message` leaf lists the missing leaves (device-atsign, access managers, device name). |
| `oper-state awaiting-onboarding` | Expected before enrollment — run the onboard script. If it persists after enrollment, confirm the keys landed at the configured `key-file` path. |
| `oper-state retrying` | sshnpd keeps exiting — read the app log: `cat /var/log/srlinux/stdout/noports*` from the bash shell. Common causes: no egress (try proxy mode), bad keys, clock skew. |
| App not listed in `show system application noports` | `tools system app-management application app_mgr reload`, then check `/etc/opt/srlinux/appmgr/noports.yml` exists. |
| Onboard script hangs then fails | Enrollment wasn't approved in time — check from your machine with `at_activate list -a @mydevice -s pending`, approve, and re-run. If it never reaches the atServer, test egress: `ip netns exec srbase-mgmt curl -v https://proxy0001.atsign.org:443` and use proxy mode. |
| Name resolution fails on the node | DNS in the `srbase-mgmt` namespace is separate from the default namespace — check `/etc/resolv.conf` and the mgmt network-instance DNS config. |
| Daemon runs but `sshnp` can't connect | Verify the client uses the same device name (`-d`), the manager atSign is in `access managers`, and (behind strict ACLs) that the relay chosen with `-r` is reachable outbound from the router. |
| Session times out after "Waiting for response from the device daemon" | The daemon log shows srv dialing the relay on a random high port and hitting `TimeoutException` — egress blocks it. Probe from the router: `ip netns exec srbase-mgmt bash -c 'echo > /dev/tcp/portquiz.net/34137'`. Fix: add `--443` (and `--relay-auth-mode escr`) to the client command to keep the relay data path on 443. |
| Enrollment hangs at "submitting enrollment request" | Control-plane egress is blocked (atDirectory port 64 / atServer high ports). Set `root-server proxy:proxy0001.atsign.org:443` **before** enrolling. |
| Re-enrolling a device | Delete the key file in `/etc/opt/noports/keys/`, revoke the old enrollment (`at_activate revoke`), and run the onboard script again. |
| What config is sshnpd actually running with? | `cat /etc/opt/noports/sshnpd.yaml` — rendered by the agent from `/noports` on every commit. Don't edit it; change the config tree and commit instead. |

Config changes take effect on `commit` — the agent restarts sshnpd with the
new settings automatically. `set / noports admin-state disable` + commit
stops the daemon.

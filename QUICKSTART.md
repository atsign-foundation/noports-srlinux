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
set / noports manager-atsigns [ @manager ]
set / noports device-name srl-router-1
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

and connect with the matching client flags:

```bash
sshnp -f @manager -r @rv_oc -t @mydevice -d srl-router-1 -u admin \
  --root-domain "proxy:proxy0001.atsign.org:443"
```

## Troubleshooting

| Symptom | Check |
|---|---|
| `oper-state not-configured` | The `message` leaf lists the missing leaves (device-atsign, manager-atsigns, device-name). |
| `oper-state awaiting-onboarding` | Expected before enrollment — run the onboard script. If it persists after enrollment, confirm the keys landed at the configured `key-file` path. |
| `oper-state retrying` | sshnpd keeps exiting — read the app log: `cat /var/log/srlinux/stdout/noports*` from the bash shell. Common causes: no egress (try proxy mode), bad keys, clock skew. |
| App not listed in `show system application noports` | `tools system app-management application app_mgr reload`, then check `/etc/opt/srlinux/appmgr/noports.yml` exists. |
| Onboard script hangs then fails | Enrollment wasn't approved in time — check from your machine with `at_activate list -a @mydevice -s pending`, approve, and re-run. If it never reaches the atServer, test egress: `ip netns exec srbase-mgmt curl -v https://proxy0001.atsign.org:443` and use proxy mode. |
| Name resolution fails on the node | DNS in the `srbase-mgmt` namespace is separate from the default namespace — check `/etc/resolv.conf` and the mgmt network-instance DNS config. |
| Daemon runs but `sshnp` can't connect | Verify the client uses the same device name (`-d`), the manager atSign is in `manager-atsigns`, and (behind strict ACLs) that the relay chosen with `-r` is reachable outbound from the router. |
| Re-enrolling a device | Delete the key file in `/etc/opt/noports/keys/`, revoke the old enrollment (`at_activate revoke`), and run the onboard script again. |

Config changes take effect on `commit` — the agent restarts sshnpd with the
new settings automatically. `set / noports admin-state disable` + commit
stops the daemon.

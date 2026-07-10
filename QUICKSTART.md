# Quickstart

Zero to an SSH session with no open ports — two paths: a free virtual lab
(no hardware, ~10 minutes) or a real SR Linux router.

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
make fetch                        # stage sshnpd + at_activate binaries
mkdir -p clab/secrets
cp etc/sshnpd.env.example clab/secrets/sshnpd.env
vi clab/secrets/sshnpd.env        # set DEVICE_ATSIGN, MANAGER_ATSIGN, DEVICE_NAME
make lab                          # deploys the SR Linux node
```

Onboard the (virtual) router with APKAM:

```bash
# on your machine: generate a one-time passcode for the device atSign
at_activate otp -a @mydevice

# on the SR Linux node:
ssh admin@clab-noports-srl-srl1   # password: NokiaSrl1!
bash
sudo /opt/sshnpd/onboard-sshnpd.sh <passcode>

# back on your machine, while the router waits: approve the enrollment
at_activate approve -a @mydevice --arx sshnpd --drx srl-router-1
```

Start the daemon and connect:

```bash
# on the node, from the SR Linux CLI:
tools system app-management application app_mgr reload
tools system app-management application sshnpd start

# from your machine — anywhere on the internet:
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
sudo dpkg -i /tmp/noports-srlinux_*.deb
sudo cp /etc/opt/sshnpd/sshnpd.env.example /etc/opt/sshnpd/sshnpd.env
sudo vi /etc/opt/sshnpd/sshnpd.env
sudo /opt/sshnpd/onboard-sshnpd.sh <passcode>   # OTP flow as in Path A
```

Then reload app_mgr, start the app, and connect exactly as in Path A.

### Management-plane ACLs block outbound?

Set proxy mode in `/etc/opt/sshnpd/sshnpd.env` **before** onboarding, so
enrollment traffic uses it too:

```bash
ROOT_SERVER="proxy:proxy0001.atsign.org:443"
```

and connect with the matching client flags:

```bash
sshnp -f @manager -r @rv_oc -t @mydevice -d srl-router-1 -u admin \
  --root-domain "proxy:proxy0001.atsign.org:443"
```

## Troubleshooting

| Symptom | Check |
|---|---|
| `show system application sshnpd` shows `error` | App log: `cat /var/log/srlinux/stdout/sshnpd*` from the bash shell. Most common: env file not created, or no atKeys yet (run the onboard script). |
| App not listed at all | `tools system app-management application app_mgr reload`, then check `/etc/opt/srlinux/appmgr/sshnpd.yml` exists. |
| Onboard script hangs then fails | Enrollment wasn't approved in time — check from your machine with `at_activate list -a @mydevice -s pending`, approve, and re-run. If it never reaches the atServer, test egress: `ip netns exec srbase-mgmt curl -v https://proxy0001.atsign.org:443` and consider proxy mode. |
| Name resolution fails on the node | DNS in the `srbase-mgmt` namespace is separate from the default namespace — check `/etc/resolv.conf` and the mgmt network-instance DNS config. |
| Daemon runs but `sshnp` can't connect | Verify the client uses the same device name (`-d`), the manager atSign is in `MANAGER_ATSIGN`, and (behind strict ACLs) that the relay chosen with `-r` is reachable outbound from the router. |
| Re-enrolling a device | Delete the old key file in `/etc/opt/sshnpd/keys/`, revoke the old enrollment (`at_activate revoke`), and run the onboard script again. |

Config changes in `sshnpd.env` take effect on
`tools system app-management application sshnpd restart`.

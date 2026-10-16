# Sources for the agent detection

The environment check (`internal/env/parse.go`, `knownAgents`) looks for running VPN and security agents by process name. The match is case insensitive and checks whether the process name contains one of the listed strings. A hit is only a hint. Checked on 2026-10-16.

Status: **documented** means a vendor page or a quote of the vendor's list names the process. **secondary** means a community or third party source. **unverified** means the name comes from our own experience and has no source yet.

## Tailscale

| Match | Platform | Status | Source |
|---|---|---|---|
| `tailscale` (app `Tailscale`, daemon `io.tailscale.ipn.macsys.network-extension`) | macOS standalone | observed | Process list of a Mac with the standalone app installed (bundle id `io.tailscale.ipn.macsys`), 2026-10-16. Tailscale's page [Three ways to run Tailscale on macOS](https://tailscale.com/docs/concepts/macos-variants) describes the variants but does not name the processes. |
| `tailscale` | macOS App Store | unverified | The app process `Tailscale` should match. The daemon name of this variant is not confirmed (a search summary claimed `IPNExtension`, the vendor page does not contain it), so it is not matched. |
| `tailscale` (`tailscaled`) | Linux | documented | [tailscaled daemon](https://tailscale.com/kb/1278/tailscaled): "the more privileged daemon that does all the network handling is called `tailscaled`" |

## Zscaler Client Connector

| Match | Platform | Status | Source |
|---|---|---|---|
| `zscaler` (`Zscaler`, `ZscalerService`, `ZscalerTunnel`, `ZscalerUpdater`) | macOS | secondary, the poster cites Zscaler's allowlist | [Jamf community thread](https://community.jamf.com/general-discussions-2/jamfg-firewall-management-to-allow-zscaler-processes-34380) from 2024-09-25. It also lists `UPMServiceController` and `zdpd`, which are not matched. The vendor page [Zscaler Client Connector processes allowlist](https://help.zscaler.com/unified/zscaler-client-connector-processes-allowlist) did not load, neither for the tool nor in a browser. |
| `zscaler` | Linux | unverified | No source found for Linux process names. Zscaler's own pages only describe the package (`zscaler-client_*.deb`, `/opt/zscaler`). Check `ps` on an installed host. |

Windows names (`ZSATray.exe`, `ZSATunnel.exe`, `ZSAService.exe`) are documented but not matched, because the tool does not run on Windows.

## Palo Alto Networks GlobalProtect

| Match | Platform | Status | Source |
|---|---|---|---|
| `pangps`, `pangpa` | macOS | documented | [GlobalProtect Processes to be Whitelisted on EDR Deployments](https://docs.paloaltonetworks.com/globalprotect/administration/globalprotect-apps/globaptorect-processes-to-be-whitelisted-on-edr-deployments) lists `PanGPS` in `GlobalProtect.app` and `PanGPA` for Windows |
| `globalprotect` (app binary) | macOS | documented | same page |
| `pangps`, `pangpa` | Linux | unverified | The Linux agent is expected to use the same names. No source checked. |

## Netskope Client

| Match | Platform | Status | Source |
|---|---|---|---|
| `stagentsvc`, `stagentui`, `stagentapp` | Linux | documented | [Exceptions for Anti Virus Applications](https://docs.netskope.com/en/exceptions-for-anti-virus-applications/) lists `stAgentSvc`, `stAgentUI`, `stAgentApp`, `stAgentCli` under `/opt/netskope/stagent/` |
| `nsauxiliarysvc` | macOS | documented | same page, XPC service in `Netskope Client.app` |
| `netskope` | macOS | documented | same page, the app is `Netskope Client.app` |

`stagentd` was in the earlier list. No source names it, so it was removed.

## FortiClient

| Match | Platform | Status | Source |
|---|---|---|---|
| `fctservctl` (also matches `fctservctl2`), `fctctld`, `sslvpnd` | macOS | documented | [FortiClient (macOS) processes](https://docs.fortinet.com/document/forticlient/6.2.0/administration-guide/234949) |
| `forticlient` | macOS | documented | same page, app and agent paths contain `FortiClient` |
| `fortitray` | any | unverified | Not found for macOS or Linux. The macOS tray controller is `FortiClientAgent` (newer builds `FortiGuardAgent`), which `forticlient` does not cover for `FortiGuardAgent`. |
| any | Linux | unverified | No Linux process list found. |

## Cloudflare WARP

| Match | Platform | Status | Source |
|---|---|---|---|
| `warp-svc` | Linux | documented | [Cloudflare One Client](https://developers.cloudflare.com/cloudflare-one/team-and-resources/devices/warp/) lists `/bin/warp-svc` and `/bin/warp-taskbar` |
| `cloudflarewarp` | macOS | documented | same docs: daemon `Cloudflare WARP.app/Contents/Resources/CloudflareWARP` |
| `cloudflare warp` | macOS | documented | same docs: GUI `Cloudflare WARP.app/Contents/MacOS/Cloudflare WARP` |

## Cisco Secure Client

| Match | Platform | Status | Source |
|---|---|---|---|
| `vpnagentd`, `cisco secure client`, `acwebsecagent` | macOS, Linux | unverified | Search found `vpnagentd` only on third party pages. Version 5.1 renamed launch labels (for example `com.cisco.secureclient.vpnagentd` to `com.cisco.secureclient.vpn.service.agent`), so process names may differ by version. Check with `ps` on an installed host. |

## WireGuard

| Match | Platform | Status | Source |
|---|---|---|---|
| `wireguard` | macOS | indirect | The [wireguard-apple](https://git.zx2c4.com/wireguard-apple/) source has a `WireGuardNetworkExtension` folder. The running process name was not confirmed. |
| `wireguard` | Linux | n/a | The kernel module has no process. `wg-quick` interfaces are found by the tunnel check, not by this list. |

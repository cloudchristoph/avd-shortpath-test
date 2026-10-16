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
| `zscaler` (`ZscalerService`, `ZscalerTunnel`, `ZscalerUpdater`) | macOS | secondary, quotes the vendor's allowlist | [Jamf community thread](https://community.jamf.com/general-discussions-2/jamfg-firewall-management-to-allow-zscaler-processes-34380). The vendor page [Zscaler Client Connector processes allowlist](https://help.zscaler.com/unified/zscaler-client-connector-processes-allowlist) is the primary source, but it could not be read automatically. Check it by hand. |
| `zscaler` | Linux | unverified | No source found for Linux process names. Zscaler's own pages only describe the package (`zscaler-client_*.deb`, `/opt/zscaler`). Check `ps` on an installed host. |

Windows names (`ZSATray.exe`, `ZSATunnel.exe`, `ZSAService.exe`) are documented but not matched, because the tool does not run on Windows.

## Palo Alto Networks GlobalProtect

| Match | Platform | Status | Source |
|---|---|---|---|
| `pangps`, `pangpa` | macOS | documented | [GlobalProtect Processes to be Whitelisted on EDR Deployments](https://docs.paloaltonetworks.com/globalprotect/administration/globalprotect-apps/globaptorect-processes-to-be-whitelisted-on-edr-deployments) lists `PanGPS` in `GlobalProtect.app` and `PanGPA` for Windows |
| `globalprotect` (app binary) | macOS | documented | same page |
| `pangps`, `pangpa` | Linux | unverified | The Linux agent is expected to use the same names. No source checked. |

## Not yet checked

Cisco Secure Client (`vpnagentd`, `cisco secure client`, `acwebsecagent`), Netskope (`netskope`, `nsauxiliarysvc`, `stagentd`), FortiClient (`forticlient`, `fortitray`), Cloudflare WARP (`cloudflare warp`, `warp-svc`) and WireGuard (`wireguard`) are unverified. They need a source before the README states that they are detected.

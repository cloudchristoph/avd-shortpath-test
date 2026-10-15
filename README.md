# avd-shortpath-test

Checks from macOS or Linux whether RDP Shortpath for public networks (UDP via STUN or TURN) is likely to work for Azure Virtual Desktop and Windows 365, similar to what `avdnettest.exe` does on Windows. It also looks for VPNs and proxies in the path, the most common reason for falling back to a relay or to TCP.

> This is an unofficial tool and is not affiliated with or endorsed by Microsoft.

**Status:** early development, nothing usable yet.

## Platforms

- **macOS** (universal binary, arm64 and amd64): network checks plus client environment checks (VPN, proxy, Windows App settings).
- **Linux** (amd64, arm64): the same network checks, but no client specific checks. There is no Windows App for Linux. The Linux build is meant for checking the network at a thin client site (IGEL, Dell ThinOS, HP and others) from any Linux device such as a laptop or a Raspberry Pi.

## Usage

```
avd-shortpath-test [--json] [--verbose] [--no-color] [--timeout 10s]
                   [--avd-turn-server world.turn.wvd.microsoft.com] [--servers file.json]
```

The output starts with the same check lines and verdict sentences as `avdnettest.exe`, so results from Windows and macOS can be compared line by line. A details section follows with response times, mapped addresses and the TURN response.

TURN is checked the way the original checks it: an unauthenticated Allocate request counts as successful if the relay answers at all (Microsoft's relays answer `300 Try Alternate`). No credentials are used, so the tool cannot test an actual relay allocation.

## Environment checks

After the network checks the tool looks at the things that most often push RDP Shortpath to a relay or to TCP. They only produce hints and never change the exit code. Skip them with `--no-env`.

| Check | macOS | Linux |
|---|---|---|
| Route to every TURN relay that was tested, flagged when it goes through a tunnel (missing split tunnel) | `route -n get` | `ip -4 route get` |
| VPN services and their state | `scutil --nc list` | tunnel interfaces only (`tun*`, `wg*`, `tailscale*`, ...) |
| Proxy (HTTP, HTTPS, SOCKS, PAC URL, auto discovery). PAC files are shown, not evaluated | `scutil --proxy`, then proxy variables | `http_proxy`, `https_proxy`, `all_proxy`, `no_proxy` |
| Carrier grade NAT (default gateway or outgoing address in 100.64.0.0/10, tunnels such as Tailscale excluded) | yes | yes |
| Known agents (Zscaler, GlobalProtect, Cisco Secure Client, Netskope, Tailscale, FortiClient, Cloudflare WARP, WireGuard) | process names | process names |
| No IPv4 address at all (the relays are IPv4 only) | yes | yes |
| Windows App version | `/Applications/Windows App.app` | n/a |

The Windows App's own settings, such as "Use UDP where possible", are stored in its protected app container. Reading them would trigger a macOS privacy prompt, so the tool does not read them.

All system commands run with fixed arguments. Nothing is sent anywhere except the STUN and TURN requests to the relays.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Direct UDP path via STUN likely (cone shaped NAT) |
| 1 | UDP via TURN relay only (symmetric or unknown NAT, relay reachable) |
| 2 | No usable UDP path, RDP falls back to TCP reverse connect |
| 3 | The checks could not run (DNS failure, no UDP socket, bad arguments) |

These differ on purpose from `avdnettest.exe`, which exits 0 for every verdict and 1 when any STUN request gets no answer. `--verbose` and `--json` also show the exit code the original would return.

## License

MIT, see [LICENSE](LICENSE).

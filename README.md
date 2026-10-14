# avd-shortpath-test

Checks from macOS or Linux whether RDP Shortpath for public networks (UDP via STUN or TURN) is likely to work for Azure Virtual Desktop and Windows 365, similar to what `avdnettest.exe` does on Windows. It also looks for VPNs and proxies in the path, the most common reason for falling back to a relay or to TCP.

> This is an unofficial tool and is not affiliated with or endorsed by Microsoft.

**Status:** early development, nothing usable yet.

## Platforms

- **macOS** (universal binary, arm64 and amd64): network checks plus client environment checks (VPN, proxy, Windows App settings).
- **Linux** (amd64, arm64): the same network checks, but no client specific checks. There is no Windows App for Linux. The Linux build is meant for checking the network at a thin client site (IGEL, Dell ThinOS, HP and others) from any Linux device such as a laptop or a Raspberry Pi.

## License

MIT, see [LICENSE](LICENSE).

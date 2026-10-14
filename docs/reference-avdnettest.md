# Reference: avdnettest.exe

Observed behaviour of Microsoft's `avdnettest.exe` (from `Azure/RDS-Templates/AVD-TestShortpath`), recorded by watching it run: `strings`, the tool's own debug log (`--log-level debug`) and a `pktmon` capture. No decompilation. Own IP addresses are replaced by `<LAN-IP>` and `<PUBLIC-IP>`.

Measured on 2026-10-14.

## Binary

| Field | Value |
|---|---|
| Source | `https://raw.githubusercontent.com/Azure/RDS-Templates/master/AVD-TestShortpath/avdnettest.exe` |
| Last change in repo | 2026-07-01, commit `c29bf5aa` "Update avdnettest tool to remove deprecated TURN server IPs" |
| Version | `1.2606.25001.acf0a4e` |
| Size | 3,796,280 bytes |
| SHA-256 | `f36b325d08e7c57f4413bf0e46c4d8cd47e40fde00e512d0d53e506729a09e3b` |
| Format | PE32+ console, x86-64 (ran under emulation on Windows 11 ARM64) |
| Signature | Valid, `CN=Microsoft Corporation`, timestamped by Microsoft Time-Stamp Service |
| Libraries seen in strings | Microsoft Basix (`libbasix-network`, the RDP client's network stack, `Dct::ICE::STUNMessage`), Boost program_options, Boost JSON |

The repo also contains the older `Test-Shortpath.ps1` (STUN only, against `stun.azure.com` and `13.107.17.41`). It is superseded by the exe and only matters for its wording (see below).

## Command line

```
AVD Network Test Version 1.2606.25001.acf0a4e

General options:
  --help                                produce help message
  --version                             print version
  --pause arg (=auto)                   Pause at end of program (auto, on, off)
  --color arg (=auto)                   Color output (auto, on, off). If you
                                        see weird characters in your output and
                                        no colors, turn this off.
  --log-level arg (=none)               Log level to use (none, critical,
                                        error, warning, normal, debug)
  --log-file arg (=avdnettest.log)      Log file to use

Server options:
  --avd-turn-server arg (=world.turn.wvd.microsoft.com)
                                        AVD TURN server DNS to use.
```

`--pause auto` waits for a key press at the end, also when stdout is not a terminal. Use `--pause off` in scripts.

## Sequence

All UDP traffic goes from **one local socket** (one source port for the whole run).

1. **DNS:** resolve `world.turn.wvd.microsoft.com` (one query, IPv4). The answer is a single address in `51.5.0.0/16` and varies between runs (seen: `51.5.68.1`, `51.5.68.23`). Only the first address is used.
2. **TURN Allocate** to `<resolved>:3478`, no credentials. Attributes:
   | Attribute | Value |
   |---|---|
   | SOFTWARE | `libnano` |
   | REQUESTED-TRANSPORT | `0x11` (UDP) |
   | LIFETIME | 3600 |
   | DONT-FRAGMENT | (empty) |
   | REQUESTED-ADDRESS-FAMILY | `0x01` (IPv4) |
3. **Allocate error response** (`0x0113`). The server does **not** answer 401, it answers **300 Try Alternate**:
   | Attribute | Value |
   |---|---|
   | ERROR-CODE | 300 "Try Alternate" |
   | NONCE | 16 bytes |
   | REALM | `rtcmedia` |
   | SOFTWARE | build string, for example `20260721.07-301a13` |
   | ALTERNATE-SERVER | another `51.5.x.y:3478` (seen: `.7`, `.10`, `.36` in the same /24 as the first server) |

   Any valid TURN error response counts as "TURN support OK". The ALTERNATE-SERVER becomes the second STUN server.
4. **STUN Binding Request** (`0x0001`, no attributes) to the resolved server.
5. **STUN Binding Request** to the ALTERNATE-SERVER address, same local socket.
6. Each Binding Response (`0x0101`) carries only XOR-MAPPED-ADDRESS.
7. **NAT evaluation:** both mapped addresses equal means "cone shaped", different means "symmetric". The log prints `Final list of addresses:` with the distinct mapped endpoints.

No TCP 443 traffic was seen. The tool fetches no credentials over HTTPS.

Typical RTTs from the test network (Germany): Allocate 34 ms, Binding to first server 22 ms, Binding to alternate 40 ms. The whole run takes about 0.2 s when UDP works.

### Excerpt (normal run, own addresses removed)

```
UDP <LAN-IP>:57833 -> <DNS>:53            dns query
UDP <DNS>:53 -> <LAN-IP>:57833            dns answer
UDP <LAN-IP>:57834 -> 51.5.68.1:3478      stun type=0x0003 len=40   Allocate Request
UDP 51.5.68.1:3478 -> <LAN-IP>:57834      stun type=0x0113 len=92   Allocate Error (300, ALTERNATE-SERVER 51.5.68.7)
UDP <LAN-IP>:57834 -> 51.5.68.1:3478      stun type=0x0001 len=0    Binding Request
UDP 51.5.68.1:3478 -> <LAN-IP>:57834      stun type=0x0101 len=12   Binding Response
UDP <LAN-IP>:57834 -> 51.5.68.7:3478      stun type=0x0001 len=0    Binding Request
UDP 51.5.68.7:3478 -> <LAN-IP>:57834      stun type=0x0101 len=12   Binding Response
```

## Timeouts and retries

- Each request waits **10 seconds** for a response (`Failed to get STUN response.: timeout` 10.0 s after the request in the log).
- With UDP 3478 blocked the run takes about 20 s: Allocate times out (10 s), then a single Binding to the first server times out (10 s). No Binding to a second server, because there is no ALTERNATE-SERVER.
- Retransmissions within the 10 s window could not be observed: the outbound firewall block drops packets before the capture point, and an inbound block has no effect because Windows Firewall is stateful. Open point, measure later with a block outside the VM.

## Output texts

Normal run (bridged network, home connection):

```
AVD Network Test Version 1.2606.25001.acf0a4e
Checking Windows cloud global TURN server(world.turn.wvd.microsoft.com) resolved to : 51.5.68.1:3478 (Unicast(0)) ... OK
Checking TURN support 51.5.68.1:3478 ... OK
Checking STUN support 51.5.68.1:3478 ... OK
Checking STUN support 51.5.68.7:3478 ... OK

Your have access to TURN servers and your NAT type appears to be 'cone shaped'.
Shortpath for public networks is very likely to work on this host.

See https://go.microsoft.com/fwlink/?linkid=2204021 for more information.
```

UDP 3478 blocked outbound:

```
AVD Network Test Version 1.2606.25001.acf0a4e
Checking Windows cloud global TURN server(world.turn.wvd.microsoft.com) resolved to : 51.5.68.1:3478 (Unicast(0)) ... OK
Checking TURN support 51.5.68.1:3478 ... FAIL
Did not receive a response from TURN server. Trying with STUN only.
Checking STUN support 51.5.68.1:3478 ... FAIL
Did not receive a response on UDP. Check if firewall allows UDP access to 51.5.0.0/16:3478.


See https://go.microsoft.com/fwlink/?linkid=2204021 for more information.
```

Note: the original prints no verdict sentence when UDP is blocked. The typo "Your have access" is in the original.

### All verdict sentences in the binary

First line (TURN and NAT):

- `Your have access to TURN servers and your NAT type appears to be 'cone shaped'.`
- `Your NAT type appears to be 'symmetric', but you have access to TURN servers.`
- `You do not have access to TURN servers, but your NAT appears to be 'cone shaped'.`
- `Your NAT type appears to be 'symmetric' and you do not have access to TURN servers.`
- `We were unable to determine your NAT type, but you have access to TURN servers.`
- `We were unable to determine your NAT type, and you do not have access to TURN servers.`

Second line (likelihood):

- `Shortpath for public networks is very likely to work on this host.`
- `Shortpath for public networks is likely to work on this host.`
- `Shortpath for public networks is very unlikely to work on this host.`
- `We are unable to determine if shortpath for public networks will work on this host.`

Footer: `See https://go.microsoft.com/fwlink/?linkid=2204021 for more information.`

Which second line goes with which first line is not visible in the strings. Only "cone + TURN = very likely" is observed so far. The other pairs need field tests (symmetric NAT, TURN blocked but STUN open).

### Other messages

- `Did not receive a response from TURN server. Trying with STUN only.`
- `UDP works, but the response from the TURN server is invalid. Trying with STUN only.`
- `Did not receive a response on UDP. Check if firewall allows UDP access to 51.5.0.0/16:3478.`
- `Failed to open local UDP port. Your computer might have UDP disabled by a policy or firewall. This is very unusual.`
- `Failed to resolve '<name>'` and ` failed. This might be a temporary failure, or your firewall might restrict DNS queries.`
- `Windows cloud TURN lookup resulted in zero addresses`

## Exit codes

| Situation | Exit code |
|---|---|
| UDP works, cone, TURN OK | 0 |
| UDP 3478 blocked | 1 |

Other cases not measured yet.

## Consequences for avd-shortpath-test

- Default target: `world.turn.wvd.microsoft.com:3478`, overridable like `--avd-turn-server`. No hard coded IP list needed, the second server comes from ALTERNATE-SERVER.
- TURN reachable means: any well formed TURN error response to an unauthenticated Allocate. In practice that is **300 Try Alternate** with REALM `rtcmedia` and NONCE. Also accept 401 with REALM and NONCE, in case the server behaviour changes.
- Without ALTERNATE-SERVER (TURN failed) the original tests STUN against one server only and therefore cannot determine the NAT type. Our tool may add a fallback second server, but must report that this differs from the original.
- Per request timeout of 10 s matches the original; our default can be shorter with `--timeout`.
- Keep the original wording for the check lines and verdicts so outputs can be compared, including the "Checking ... OK/FAIL" lines.

## Test setup

- VM: VMPal "Windows 11 Tests", Windows 11 24H2 ARM64 (build 26100), network bridged to `en0` (Ethernet). Home connection in Germany, one NAT layer (router).
- Capture: `pktmon` with filters on port 53, UDP 3478 and TCP 443, converted with `pktmon etl2pcap`. The resulting pcapng has a snaplen of 0 that `tcpdump` rejects; read with a small parser instead.
- Runs: two normal runs, one with outbound UDP 3478 blocked (Windows Firewall rule, removed afterwards), one with inbound blocked (no effect).
- Comparison run with VM NAT: not done yet.

## Open points

- Retransmission behaviour within the 10 s timeout.
- Pairing of first and second verdict lines for symmetric NAT and for TURN blocked with STUN open.
- Exit codes for the other outcomes.
- Run with VM NAT (double NAT) for comparison.
- Whether the web client uses RDP Shortpath (decides how useful the Linux build is).

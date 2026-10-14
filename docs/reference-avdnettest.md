# Reference: avdnettest.exe

Behaviour of Microsoft's `avdnettest.exe` (from `Azure/RDS-Templates/AVD-TestShortpath`), recorded for interoperability so that avd-shortpath-test can match it. Sources:

- **Observation:** `strings`, the tool's own debug log (`--log-level debug`), `pktmon` captures including drop events, and controlled runs with different targets and network modes.
- **Static analysis:** disassembly of `main` (capstone, function bounds from `.pdata`, C++ exception tables) to confirm the decision logic. Addresses below refer to this exact build.

Every statement marked **[measured]** was observed in a run; **[code]** comes from static analysis only; **[both]** was confirmed both ways. Own IP addresses are replaced by `<LAN-IP>` and `<PUBLIC-IP>`.

No public source code exists. The binary points to Microsoft internal sources (`tools/avdnettest/main.cpp`, `src/libbasix-network`, the RDP client's network stack). The only public material is the repo above, `Test-Shortpath.ps1` and sample output on Microsoft Learn.

Measured and analysed on 2026-10-14.

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
| Libraries | Microsoft Basix (`Dct::ICE::STUNMessage`), Boost program_options, Boost JSON |

The repo also contains the older `Test-Shortpath.ps1` (STUN only, against `stun.azure.com` and `13.107.17.41`). It is superseded by the exe.

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

- `--avd-turn-server` also takes an IP literal **[measured]**.
- `--pause auto` pauses when the console has fewer than 2 processes, i.e. when started by double click **[code]**. When run from a non-console parent (for example a remote exec) it pauses too **[measured]**. Use `--pause off` in scripts.

## Sequence

All UDP traffic uses **one local socket**, one ephemeral source port for the whole run **[both]**.

1. **DNS:** `getaddrinfo` with `AF_INET`, so **A records only** **[both]**. `world.turn.wvd.microsoft.com` has no AAAA record anyway **[measured]**. The name resolves to one address in `51.5.0.0/16` that varies between runs (seen `51.5.68.1`, `.22`, `.23`). **All** returned addresses are used: a name with two A records (`stun.sipgate.net`) got a TURN and a STUN check per address **[both]**. There is no fallback IP and no hard coded server list anywhere in the binary **[code]**.
2. **TURN Allocate** to every resolved address, one after another, no credentials **[both]**:
   | Attribute | Value |
   |---|---|
   | SOFTWARE | `libnano` |
   | REQUESTED-TRANSPORT | `0x11` (UDP) |
   | LIFETIME | 3600 |
   | DONT-FRAGMENT | (empty) |
   | REQUESTED-ADDRESS-FAMILY | `0x01` (IPv4) |
3. **Allocate response.** Microsoft's servers answer with an error response (`0x0113`) **300 Try Alternate**, not 401 **[measured]**:
   | Attribute | Value |
   |---|---|
   | ERROR-CODE | 300 "Try Alternate" |
   | NONCE | 16 bytes |
   | REALM | `rtcmedia` |
   | SOFTWARE | build string, for example `20260721.07-301a13` |
   | ALTERNATE-SERVER | another `51.5.x.y:3478` in the same /24 (seen `.3`, `.7`, `.10`, `.34`, `.35`, `.36`, `.43`) |
4. **Server list for STUN:** the resolved addresses, followed by every ALTERNATE-SERVER received, not de-duplicated **[code]**.
5. **STUN Binding Request** (`0x0001`, no attributes) to every server in that list, one after another **[both]**.
6. **Binding Response** (`0x0101`) carries only XOR-MAPPED-ADDRESS **[measured]**. The tool reads XOR-MAPPED-ADDRESS, falling back to MAPPED-ADDRESS **[code]**.
7. **NAT evaluation** (see below). The log prints `Final list of addresses:` with the distinct mapped endpoints.

No TCP 443 traffic was seen and there is no code path for fetching TURN credentials **[both]**.

Typical RTTs from the test network (Germany): Allocate 34 ms, Binding to first server 22 ms, Binding to alternate 40 ms. The whole run takes about 0.2 s when UDP works.

### What counts as "TURN support OK"

The wait routine (`0x14006b7e8`) accepts the first datagram that parses as STUN and carries the request's transaction ID. It does **not** check message class or error code **[code]**.

| Response to Allocate | Result |
|---|---|
| Any well formed STUN message with matching transaction ID (300, 401, success, any error) | `TURN support ... OK`, TURN access flag set. Measured with 300 (Microsoft), 401 and an Allocate success response (test server) |
| Same, but without ALTERNATE-SERVER | still OK, no second server, so NAT type unknown **[measured]** |
| No response within 10 s | `FAIL` + `Did not receive a response from TURN server. Trying with STUN only.` **[measured]** |
| Datagram that fails STUN parsing | `FAIL` + `UDP works, but the response from the TURN server is invalid. Trying with STUN only.` **[measured]** |

Observed: `stun.cloudflare.com` and `stun.sipgate.net` silently ignore the Allocate, which gives the timeout case **[measured]**.

### NAT evaluation

Counting the mapped addresses (`0x1400810c2`) **[code]**, confirmed by runs **[measured]**:

- 0 addresses: no verdict (only when the server list is empty, i.e. DNS failed).
- 1 address: NAT type unknown. Happens when there is only one server (one A record, no ALTERNATE-SERVER).
- 2 or more: adjacent duplicates removed (`std::unique`, no sort). One left is "cone shaped", more is "symmetric".

The comparison is a `memcmp` of the whole sockaddr, so **IP and port** count **[code]**. Confirmed: behind VMPal's NAT the public IP stayed the same and only the port changed, and the verdict was "symmetric" **[measured]**.

A STUN Binding that gets no answer prints `FAIL` and `Did not receive a response on UDP. Check if firewall allows UDP access to 51.5.0.0/16:3478.`, sets exit code 1 and the run goes on. The verdict is then computed from the addresses collected so far **[measured]**: if only the alternate server is silent, the run prints "unable to determine your NAT type, but you have access to TURN servers" / "very likely" and still exits 1. With no Binding answer at all there is no address and therefore no verdict. (The static analysis assumed the run aborts; the test server run `f4` shows it does not.)

### Excerpt (normal run, own addresses removed)

```
UDP <LAN-IP>:57833 -> <DNS>:53            dns query A world.turn.wvd.microsoft.com
UDP <DNS>:53 -> <LAN-IP>:57833            dns answer
UDP <LAN-IP>:57834 -> 51.5.68.1:3478      stun type=0x0003 len=40   Allocate Request
UDP 51.5.68.1:3478 -> <LAN-IP>:57834      stun type=0x0113 len=92   Allocate Error (300, ALTERNATE-SERVER 51.5.68.7)
UDP <LAN-IP>:57834 -> 51.5.68.1:3478      stun type=0x0001 len=0    Binding Request
UDP 51.5.68.1:3478 -> <LAN-IP>:57834      stun type=0x0101 len=12   Binding Response
UDP <LAN-IP>:57834 -> 51.5.68.7:3478      stun type=0x0001 len=0    Binding Request
UDP 51.5.68.7:3478 -> <LAN-IP>:57834      stun type=0x0101 len=12   Binding Response
```

## Timeouts and retries

- One send per request, then up to **10 s** for the matching response. **No retransmissions**, neither for Allocate nor for Binding **[both]**. With UDP 3478 blocked, pktmon logged exactly two dropped packets from the same source port: the Allocate at t=0 and the Binding at t=10 s.
- A separate wait of up to 10 s for the local UDP socket to become ready **[code]**.
- With UDP 3478 blocked a run takes about 20 s **[measured]**.

## Verdicts

Each verdict is one fixed two line string, so the second line always follows from the first **[code]**.

| NAT | TURN access | First line | Second line | Colour | Seen in run |
|---|---|---|---|---|---|
| cone | yes | `Your have access to TURN servers and your NAT type appears to be 'cone shaped'.` | `Shortpath for public networks is very likely to work on this host.` | green | bridged home network |
| cone | no | `You do not have access to TURN servers, but your NAT appears to be 'cone shaped'.` | `Shortpath for public networks is likely to work on this host.` | green | `--avd-turn-server stun.sipgate.net` |
| symmetric | yes | `Your NAT type appears to be 'symmetric', but you have access to TURN servers.` | `Shortpath for public networks is likely to work on this host.` | green | VMPal shared (NAT) network |
| symmetric | no | `Your NAT type appears to be 'symmetric' and you do not have access to TURN servers.` | `Shortpath for public networks is very unlikely to work on this host.` | red | VMPal NAT + `--avd-turn-server stun.sipgate.net` |
| unknown | yes | `We were unable to determine your NAT type, but you have access to TURN servers.` | `Shortpath for public networks is very likely to work on this host.` | green | test server answering 401, no ALTERNATE-SERVER |
| unknown | no | `We were unable to determine your NAT type, and you do not have access to TURN servers.` | `We are unable to determine if shortpath for public networks will work on this host.` | yellow | `--avd-turn-server stun.cloudflare.com` |

The typo "Your have access" is in the original. Footer after every run: `See https://go.microsoft.com/fwlink/?linkid=2204021 for more information.`

### Runs without a verdict

UDP 3478 blocked, or a target that never answers (`192.0.2.1`) **[measured]**, exit 1:

```
AVD Network Test Version 1.2606.25001.acf0a4e
Checking Windows cloud global TURN server(world.turn.wvd.microsoft.com) resolved to : 51.5.68.1:3478 (Unicast(0)) ... OK
Checking TURN support 51.5.68.1:3478 ... FAIL
Did not receive a response from TURN server. Trying with STUN only.
Checking STUN support 51.5.68.1:3478 ... FAIL
Did not receive a response on UDP. Check if firewall allows UDP access to 51.5.0.0/16:3478.


See https://go.microsoft.com/fwlink/?linkid=2204021 for more information.
```

DNS failure (`does-not-exist.invalid`) **[measured]**, exit **0**:

```
AVD Network Test Version 1.2606.25001.acf0a4e
FAIL
DNS resolution of does-not-exist.invalid failed. This might be a temporary failure, or your firewall might restrict DNS queries.



See https://go.microsoft.com/fwlink/?linkid=2204021 for more information.
```

With several resolved addresses, each one gets its own `Checking TURN support` line first, then each server its own `Checking STUN support` line **[both]**.

### Other messages

- `UDP works, but the response from the server is invalid. This is very unusual.` (invalid Binding response, exit 1) **[measured]**
- `Failed to open local UDP port. Your computer might have UDP disabled by a policy or firewall. This is very unusual.` (exit 1) **[code]**
- `Windows cloud TURN lookup resulted in zero addresses` **[code]**

## Exit codes

| Code | When | Source |
|---|---|---|
| 0 | Run reaches the end: all six verdicts, TURN failed but STUN worked, and also **DNS failure** | [both] |
| 0 | `--version` | [both] |
| 1 | At least one STUN Binding without answer (UDP blocked, target dead, alternate silent). A verdict may still be printed | [both] |
| 1 | Invalid Binding response (`UDP works, but the response from the server is invalid. This is very unusual.`) | [both] |
| 1 | Local UDP socket not ready | [code] |
| 1 | `--help` | [both] |
| 99 | Invalid command line (`--bogus-option`) | [both] |

The exit code says nothing about the verdict. Scripts around the original cannot tell "very likely" from "very unlikely" by exit code.

## Not used

- IPv6: socket family and DNS are IPv4 only **[code]**. The test network has global IPv6, the tool still used IPv4 only **[measured]**.
- IPv6 only network (IPv4 unbound from the VM adapter, IPv6 DNS still working and still returning the A record): the tool reports `DNS resolution of world.turn.wvd.microsoft.com failed. ...` and exits 0 **[measured]**. The message is misleading, the real cause is the missing IPv4 address.
- `EcnCheckStun`, `CiscoStunFlowdata`, `MS_MultiplexedTurnSessionId` and similar strings are only in the library's attribute name table for logging; `main` never sends them **[code]**.

## Consequences for avd-shortpath-test

- Default target `world.turn.wvd.microsoft.com:3478`, overridable like `--avd-turn-server` (name or IP). Query A records, use every address. No hard coded IP list; the further servers come from ALTERNATE-SERVER.
- TURN reachable means any well formed STUN response with matching transaction ID to an unauthenticated Allocate. In practice that is 300 Try Alternate. Report the error code we got in `--verbose` and JSON.
- One socket, Allocate to all resolved addresses first, then Binding to resolved plus alternate servers. Compare mapped IP and port. Same "unknown" rule (exactly one mapped address).
- Same verdict table and wording, so outputs can be compared. Keep the `Checking ... OK/FAIL` lines.
- Differences we make on purpose, to be documented in the README:
  - Exit codes follow our contract (`0` direct UDP likely, `1` relay only, `2` UDP blocked, `3` tool error) instead of the original's 0/1/99.
  - When a Binding answer is missing we keep the original's behaviour (verdict from what was collected), but the exit code reflects the verdict, not the missing answer.
  - In an IPv6 only network we say so instead of reporting a DNS failure.
  - Shorter default timeout is possible via `--timeout`; default 10 s like the original.

## Test setup

- VM: VMPal "Windows 11 Tests", Windows 11 24H2 ARM64 (build 26100), network bridged to `en0` (Ethernet). Home connection in Germany, one NAT layer (router), dual stack.
- Comparison network: VMPal shared mode (VM NAT behind the Mac, then the router): symmetric.
- Capture: `pktmon` with filters on port 53, UDP 3478 and TCP 443, `--comp all --type all` to include drop events, converted with `pktmon etl2pcap` and `etl2txt`. The pcapng from pktmon has a snaplen of 0 that `tcpdump` rejects; read with a small parser instead.
- Blocking: Windows Firewall outbound rule on UDP remote port 3478, removed after each run.
- Static analysis: Python with capstone and pefile on the Mac, binary never executed there.

## Test server runs

To reach code paths that Microsoft's servers don't trigger, a small STUN server (Go, `pion/stun`) ran on the Mac in the same LAN, and the tool was pointed at it with `--avd-turn-server <mac-ip>`. Private targets show as `(Private(3))` instead of `(Unicast(0))` in the check line.

| Run | Allocate answer | Binding answer | Result | Exit |
|---|---|---|---|---|
| f1 | 401 + REALM + NONCE | ok | TURN OK, one mapped address: "unable to determine your NAT type, but you have access to TURN servers" / "very likely" | 0 |
| f2 | Allocate success | ok | same as f1 | 0 |
| f3 | not STUN | ok | TURN FAIL "response ... is invalid", then "unable to determine ... and you do not have access" / "unable to determine if shortpath" | 0 |
| f4 | 300 + ALTERNATE-SERVER to a silent port | ok on first server | STUN FAIL on the alternate, verdict still printed ("unable to determine NAT, but TURN" / "very likely") | 1 |
| f5 | 401 | not STUN | STUN FAIL "UDP works, but the response from the server is invalid. This is very unusual.", no verdict | 1 |
| f6 | 300 + ALTERNATE-SERVER pointing to itself | ok | two identical mapped addresses: "cone shaped" + TURN / "very likely" | 0 |

## Client support (Microsoft Learn, Compare Windows App features, Network)

- Web browser: no RDP Shortpath at all (neither managed nor public networks). So the Linux build is only useful for checking the network at a thin client site, not for browser users.
- macOS: RDP Shortpath for public networks (STUN and TURN) needs Windows App for macOS **beta 11.3.8 (3048)** according to Learn. Shortpath for managed networks works with the release version.
- Windows, Android/Chrome OS and Meta Quest support STUN and TURN; iOS/iPadOS does not.

## Open points

None for the reference itself. Field tests (`docs/field-tests.md`) will repeat the matrix in real networks (hotspot with CGNAT, Azure VM behind NAT Gateway, VPN without split tunnel).

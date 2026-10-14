# avd-shortpath-test

Open CLI for macOS and Linux that checks what Microsoft's `avdnettest.exe` checks on Windows (DNS, STUN, TURN, NAT type) and additionally detects VPNs and proxies in the path. Not affiliated with Microsoft.

## Working with Christoph

- Talk to Christoph in German. Code, comments, commit messages and README stay in English.
- No dashes as punctuation (no em or en dashes, no " - " as a sentence break), neither in chat nor in docs.
- Commit and push only when Christoph says so.
- No secrets in the repo. Tokens (for example for the Homebrew tap) live only as GitHub secrets, Christoph creates them himself.

## Project facts

- Go module: `github.com/cloudchristoph/avd-shortpath-test` (lower case on purpose).
- Repo: `github.com/CloudChristoph/avd-shortpath-test`, MIT, public.
- Targets: macOS universal binary (arm64 + amd64) is the main target, Linux amd64/arm64 as a by-product. No Windows build.
- No cgo. Platform code only via build tags or `_darwin.go` / `_linux.go` file suffixes.
- Dependencies pinned to exact versions in `go.mod`.
- System commands (`scutil`, `route`, `ip`) are called with fixed arguments only, output is parsed, user input is never passed through.
- No telemetry. The tool only connects to the test targets.
- Default server list is embedded from `internal/config/servers.json`, never scattered through the code.

## Layout

```
cmd/avd-shortpath-test/   main package, flags
internal/probe/           DNS, STUN, TURN, NAT evaluation
internal/env/             VPN, proxy, CGNAT, agents (per OS)
internal/report/          text and JSON output, verdict, exit code
internal/config/          embedded servers.json
docs/                     reference-avdnettest.md, field-tests.md
```

## Behaviour contract

- Verdict wording matches the original avdnettest.exe so Windows and Mac outputs can be compared side by side. The exact sentences are recorded in `docs/reference-avdnettest.md`.
- Exit codes: `0` direct UDP path (STUN) likely, `1` relay only (TURN), `2` UDP blocked (TCP only), `3` tool error.
- TURN is checked for reachability, like the original: any well formed STUN response with the matching transaction ID to an unauthenticated Allocate counts as reachable. Microsoft's servers answer `300 Try Alternate` with REALM, NONCE and ALTERNATE-SERVER. The error code we got is shown in `--verbose` and JSON. Every ALTERNATE-SERVER is added as a further STUN server, as in the original.
- Unlike the original (exit 1 when UDP is blocked), our exit codes follow the contract above. The README must say so.
- NAT filtering behaviour (RFC 5780) is not promised.

## Commands

```
go build ./...
go vet ./...
go test ./...
```

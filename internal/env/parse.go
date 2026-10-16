package env

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// Parsers are platform neutral so they can be tested anywhere.

// parseBSDRouteGet reads the output of `route -n get <dest>` (macOS).
func parseBSDRouteGet(out string) (Route, bool) {
	var r Route
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch k {
		case "destination":
			r.Destination = v
		case "gateway":
			r.Gateway = v
		case "interface":
			r.Interface = v
		}
	}
	return r, r.Interface != ""
}

// parseIPRoute reads the first line of `ip -4 route get <ip>` or
// `ip -4 route show default` (Linux).
func parseIPRoute(out string) (Route, bool) {
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	f := strings.Fields(line)
	if len(f) == 0 {
		return Route{}, false
	}
	r := Route{Destination: f[0]}
	for i := 1; i+1 < len(f); i++ {
		switch f[i] {
		case "via":
			r.Gateway = f[i+1]
		case "dev":
			r.Interface = f[i+1]
		case "src":
			r.Source = f[i+1]
		}
	}
	return r, r.Interface != ""
}

// `scutil --nc list` line, for example:
// * (Connected)      66175C33-... VPN (io.tailscale.ipn.macsys) "Tailscale"   [VPN:io.tailscale.ipn.macsys]
var ncLine = regexp.MustCompile(`^(\*?)\s*\(([^)]+)\)\s+[0-9A-Fa-f-]{36}\s+(.*?)\s+"([^"]*)"`)

// parseNCList reads `scutil --nc list` (macOS).
func parseNCList(out string) []VPNService {
	var svcs []VPNService
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		m := ncLine.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil {
			continue
		}
		svcs = append(svcs, VPNService{
			Name:    m[4],
			Kind:    strings.TrimSpace(m[3]),
			State:   m[2],
			Enabled: m[1] == "*",
		})
	}
	return svcs
}

// parseScutilProxy reads `scutil --proxy` (macOS). Only the keys that matter
// are read; a PAC file is shown, not evaluated.
func parseScutilProxy(out string) Proxy {
	kv := map[string]string{}
	var exceptions []string
	inExceptions := false
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if inExceptions {
			if line == "}" {
				inExceptions = false
				continue
			}
			if _, v, ok := strings.Cut(line, " : "); ok {
				exceptions = append(exceptions, v)
			}
			continue
		}
		k, v, ok := strings.Cut(line, " : ")
		if !ok {
			continue
		}
		if k == "ExceptionsList" {
			inExceptions = true
			continue
		}
		kv[k] = v
	}
	hostPort := func(prefix string) string {
		if kv[prefix+"Enable"] != "1" || kv[prefix+"Proxy"] == "" {
			return ""
		}
		if p := kv[prefix+"Port"]; p != "" {
			return kv[prefix+"Proxy"] + ":" + p
		}
		return kv[prefix+"Proxy"]
	}
	p := Proxy{
		Source:        "system",
		HTTP:          hostPort("HTTP"),
		HTTPS:         hostPort("HTTPS"),
		SOCKS:         hostPort("SOCKS"),
		AutoDiscovery: kv["ProxyAutoDiscoveryEnable"] == "1",
	}
	if kv["ProxyAutoConfigEnable"] == "1" {
		p.PACURL = kv["ProxyAutoConfigURLString"]
	}
	if p.Configured() {
		p.Exceptions = exceptions
	}
	return p
}

// proxyFromEnv reads the usual proxy variables (Linux, and shells on macOS).
func proxyFromEnv(getenv func(string) string) Proxy {
	get := func(names ...string) string {
		for _, n := range names {
			if v := getenv(n); v != "" {
				return v
			}
		}
		return ""
	}
	p := Proxy{
		Source: "environment",
		HTTP:   get("http_proxy", "HTTP_PROXY"),
		HTTPS:  get("https_proxy", "HTTPS_PROXY"),
		SOCKS:  get("all_proxy", "ALL_PROXY"),
	}
	if no := get("no_proxy", "NO_PROXY"); no != "" && p.Configured() {
		p.Exceptions = strings.Split(no, ",")
	}
	return p
}

// parseProcessList reads `ps -axco comm` / `ps -eo comm` output and returns
// the known VPN and security agents that run.
func parseProcessList(out string) []string {
	seen := map[string]bool{}
	var found []string
	for _, line := range strings.Split(out, "\n") {
		name := strings.ToLower(strings.TrimSpace(line))
		if name == "" {
			continue
		}
		for _, a := range knownAgents {
			if seen[a.name] {
				continue
			}
			for _, p := range a.processes {
				if strings.Contains(name, p) {
					seen[a.name] = true
					found = append(found, a.name)
					break
				}
			}
		}
	}
	return found
}

// knownAgents are matched case insensitively against process names. Only a
// hint: a running agent does not mean it captures UDP 3478. Where the names
// come from is recorded in docs/agent-detection-sources.md.
var knownAgents = []struct {
	name      string
	processes []string
}{
	{"Zscaler", []string{"zscaler"}},
	{"GlobalProtect", []string{"pangps", "pangpa", "globalprotect"}},
	{"Cisco Secure Client", []string{"vpnagentd", "cisco secure client", "acwebsecagent"}},
	{"Netskope", []string{"netskope", "nsauxiliarysvc", "stagentsvc", "stagentui", "stagentapp"}},
	{"Tailscale", []string{"tailscale"}},
	{"FortiClient", []string{"forticlient", "fortitray", "fctservctl", "fctctld", "sslvpnd"}},
	{"Cloudflare WARP", []string{"cloudflare warp", "cloudflarewarp", "warp-svc"}},
	{"WireGuard", []string{"wireguard"}},
}

// versionAtLeast compares dotted numeric versions like "11.4.3".
func versionAtLeast(v, min string) bool {
	a, b := strings.Split(v, "."), strings.Split(min, ".")
	for i := 0; i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x, _ = strconv.Atoi(a[i])
		}
		y, _ = strconv.Atoi(b[i])
		if x != y {
			return x > y
		}
	}
	return true
}

package reload

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"email-oauth2-proxy-ui/internal/config"
)

// PortSpec describes a proxy host port to publish and document in Compose YAML.
type PortSpec struct {
	Port          int    `json:"port"`
	Section       string `json:"section,omitempty"`
	ServerAddress string `json:"server_address,omitempty"`
}

// PortStatus describes availability and Docker publication status for a single port.
type PortStatus struct {
	Port              int    `json:"port"`
	Section           string `json:"section,omitempty"`
	Protocol          string `json:"protocol,omitempty"`
	ServerAddress     string `json:"server_address,omitempty"`
	PublishedOnProxy  bool   `json:"published_on_proxy"`
	Protected         bool   `json:"protected,omitempty"`
	Status            string `json:"status"` // "open_on_proxy", "available", "conflict_docker", "conflict_host", "conflict_config", "unknown"
	Message           string `json:"message"`
	ConflictContainer string `json:"conflict_container,omitempty"`
}

// DockerPortsOverview describes current Docker & Compose port status.
type DockerPortsOverview struct {
	DockerAvailable  bool         `json:"docker_available"`
	SelfPidShared    bool         `json:"self_pid_shared"`
	ProxyFound       bool         `json:"proxy_found"`
	ProxyRunning     bool         `json:"proxy_running"`
	PublishedPorts   []int        `json:"published_ports"`
	ProtectedPorts   []int        `json:"protected_ports"`
	ComposeAvailable bool         `json:"compose_available"`
	ComposePath      string       `json:"compose_path,omitempty"`
	ComposeService   string       `json:"compose_service,omitempty"`
	ComposeHostHint  string       `json:"compose_host_hint,omitempty"`
	PortsYAML        string       `json:"ports_yaml"`
	Statuses         []PortStatus `json:"statuses"`
}

// ApplyPortsResult describes the outcome of applying port changes.
type ApplyPortsResult struct {
	OK               bool   `json:"ok"`
	Mode             string `json:"mode"`
	DockerRecreated  bool   `json:"docker_recreated"`
	ComposeUpdated   bool   `json:"compose_updated"`
	ComposePath      string `json:"compose_path,omitempty"`
	RolledBack       bool   `json:"rolled_back,omitempty"`
	Message          string `json:"message"`
}

// ResolveComposeFile inspects composePath (either a file or a directory containing docker-compose.yml / compose.yml)
// and verifies whether it exists as a regular writable file.
func ResolveComposeFile(composePath string) (resolved string, writable bool) {
	composePath = strings.TrimSpace(composePath)
	if composePath == "" {
		return "", false
	}

	info, err := os.Stat(composePath)
	if err != nil {
		return "", false
	}

	candidate := composePath
	if info.IsDir() {
		found := ""
		for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
			p := filepath.Join(composePath, name)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				found = p
				break
			}
		}
		if found == "" {
			return "", false
		}
		candidate = found
	}

	f, err := os.OpenFile(candidate, os.O_RDWR, 0)
	if err != nil {
		return candidate, false
	}
	_ = f.Close()
	return candidate, true
}

// RenderPortsSnippet builds a clean YAML `ports:` snippet for manual copy or preview.
func RenderPortsSnippet(specs []PortSpec) string {
	sorted := DeduplicateAndSortSpecs(specs)
	if len(sorted) == 0 {
		return "    ports: []"
	}
	var b strings.Builder
	b.WriteString("    ports:\n")
	for _, s := range sorted {
		b.WriteString(formatComposePortLine("      ", s))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// DeduplicateAndSortSpecs normalizes and sorts PortSpecs ascending by port number.
func DeduplicateAndSortSpecs(specs []PortSpec) []PortSpec {
	seen := make(map[int]PortSpec)
	for _, s := range specs {
		if s.Port <= 0 || s.Port > 65535 {
			continue
		}
		if _, ok := seen[s.Port]; !ok {
			seen[s.Port] = s
		}
	}
	out := make([]PortSpec, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Port < out[j].Port
	})
	return out
}

func formatComposePortLine(indent string, s PortSpec) string {
	comment := ""
	if s.Section != "" && s.ServerAddress != "" {
		comment = fmt.Sprintf(" # %s (%s)", s.Section, s.ServerAddress)
	} else if s.Section != "" {
		comment = fmt.Sprintf(" # %s", s.Section)
	}
	return fmt.Sprintf(`%s- "%d:%d"%s`, indent, s.Port, s.Port, comment)
}

var portMappingLineRe = regexp.MustCompile(`^\s*-\s*['"]?(?:[^:'"]+:)?(\d+|\$\{[^}]+\}):(\d+)(?:/(tcp|udp))?['"]?`)

// UpdateComposePortsYAML surgically updates or inserts the `ports:` block of serviceName inside a Compose YAML file
// while preserving comments, formatting, environment variables, and all other services intact.
func UpdateComposePortsYAML(yamlText, serviceName string, specs []PortSpec) (string, error) {
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "email-oauth2-proxy"
	}
	sorted := DeduplicateAndSortSpecs(specs)

	useCRLF := strings.Contains(yamlText, "\r\n")
	norm := strings.ReplaceAll(yamlText, "\r\n", "\n")
	hadTrailingNewline := strings.HasSuffix(norm, "\n")
	lines := strings.Split(strings.TrimSuffix(norm, "\n"), "\n")

	servicesLine := -1
	for i, l := range lines {
		if leadingSpaces(l) == 0 && strings.TrimSpace(stripInlineComment(l)) == "services:" {
			servicesLine = i
			break
		}
	}
	if servicesLine < 0 {
		return "", fmt.Errorf("seção 'services:' não encontrada no arquivo docker-compose.yml")
	}

	svcStart := -1
	svcIndent := -1
	for i := servicesLine + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		ind := leadingSpaces(lines[i])
		if ind == 0 {
			break // exited services:
		}
		if svcIndent < 0 {
			svcIndent = ind
		}
		if ind == svcIndent && strings.TrimSpace(stripInlineComment(lines[i])) == serviceName+":" {
			svcStart = i
			break
		}
	}
	if svcStart < 0 {
		return "", fmt.Errorf("serviço '%s:' não encontrado dentro de 'services:' no docker-compose.yml", serviceName)
	}

	// Find end of service block (exclusive)
	svcEnd := len(lines)
	propIndent := -1
	portsKeyIdx := -1
	for i := svcStart + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		ind := leadingSpaces(lines[i])
		if ind <= svcIndent {
			svcEnd = i
			break
		}
		if propIndent < 0 || ind < propIndent {
			propIndent = ind
		}
		if ind == propIndent && strings.HasPrefix(trimmed, "ports:") {
			portsKeyIdx = i
		}
	}

	if propIndent < 0 {
		propIndent = svcIndent + 2
	}
	propIndentStr := strings.Repeat(" ", propIndent)
	itemIndentStr := strings.Repeat(" ", propIndent+2)

	if portsKeyIdx >= 0 {
		// Find range of existing list items under ports:
		portsEnd := portsKeyIdx + 1
		var preserved []string
		for i := portsKeyIdx + 1; i < svcEnd; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				// Keep looking only if followed by more list items of ports:
				if hasFollowingListItem(lines, i+1, svcEnd, propIndent) {
					portsEnd = i + 1
					continue
				}
				break
			}
			ind := leadingSpaces(lines[i])
			if ind > propIndent || (ind == propIndent && strings.HasPrefix(trimmed, "-")) {
				portsEnd = i + 1
				if strings.HasPrefix(trimmed, "-") && shouldPreservePortLine(lines[i]) {
					preserved = append(preserved, lines[i])
				}
				continue
			}
			break
		}

		newBlock := buildPortsBlock(propIndentStr, itemIndentStr, preserved, sorted)
		outLines := make([]string, 0, len(lines)-(portsEnd-portsKeyIdx)+len(newBlock))
		outLines = append(outLines, lines[:portsKeyIdx]...)
		outLines = append(outLines, newBlock...)
		outLines = append(outLines, lines[portsEnd:]...)
		lines = outLines
	} else {
		// Insert new ports: block at the end of non-empty lines inside service block
		insertAt := svcEnd
		for insertAt > svcStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
			insertAt--
		}
		newBlock := buildPortsBlock(propIndentStr, itemIndentStr, nil, sorted)
		outLines := make([]string, 0, len(lines)+len(newBlock))
		outLines = append(outLines, lines[:insertAt]...)
		outLines = append(outLines, newBlock...)
		outLines = append(outLines, lines[insertAt:]...)
		lines = outLines
	}

	res := strings.Join(lines, "\n")
	if hadTrailingNewline {
		res += "\n"
	}
	if useCRLF {
		res = strings.ReplaceAll(res, "\n", "\r\n")
	}
	return res, nil
}

func buildPortsBlock(propIndentStr, itemIndentStr string, preserved []string, sorted []PortSpec) []string {
	if len(preserved) == 0 && len(sorted) == 0 {
		return []string{propIndentStr + "ports: []"}
	}
	out := []string{propIndentStr + "ports:"}
	out = append(out, preserved...)
	for _, s := range sorted {
		out = append(out, formatComposePortLine(itemIndentStr, s))
	}
	return out
}

func shouldPreservePortLine(line string) bool {
	m := portMappingLineRe.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	containerPort := m[2]
	// Always protect Runtipi's port 80 callback mapping (e.g. ${APP_PORT}:80)
	return containerPort == "80"
}

func hasFollowingListItem(lines []string, start, limit, propIndent int) bool {
	for i := start; i < limit; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		ind := leadingSpaces(lines[i])
		return ind > propIndent || (ind == propIndent && strings.HasPrefix(trimmed, "-"))
	}
	return false
}

func leadingSpaces(s string) int {
	n := 0
	for _, ch := range s {
		if ch == ' ' {
			n++
		} else if ch == '\t' {
			n += 2
		} else {
			break
		}
	}
	return n
}

func stripInlineComment(s string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return s[:i]
			}
		}
	}
	return s
}

// WriteComposeFile updates composePath safely after creating a .bak backup.
// Falls back to in-place write if atomic rename fails across single-file bind mount boundaries.
func WriteComposeFile(composePath, content string) error {
	bakPath := composePath + ".bak"
	if existing, err := os.ReadFile(composePath); err == nil && len(existing) > 0 {
		_ = os.WriteFile(bakPath, existing, 0o644)
	}

	dir := filepath.Dir(composePath)
	tmpPath := filepath.Join(dir, ".compose.tmp."+strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err == nil {
		if err := os.Rename(tmpPath, composePath); err == nil {
			return nil
		}
		_ = os.Remove(tmpPath)
	}

	// Single-file bind mounts in Docker return EBUSY/EXDEV on rename; overwrite in place cleanly.
	return os.WriteFile(composePath, []byte(content), 0o644)
}

// SpecsFromServers converts ServerEntry items into deduplicated, sorted PortSpecs.
func SpecsFromServers(servers []config.ServerEntry) []PortSpec {
	out := make([]PortSpec, 0, len(servers))
	for _, s := range servers {
		out = append(out, PortSpec{
			Port:          s.LocalPort,
			Section:       s.Section,
			ServerAddress: s.ServerAddress,
		})
	}
	return DeduplicateAndSortSpecs(out)
}

func evaluatePortsStatic(servers []config.ServerEntry, proxyPorts map[int]bool, otherPorts map[int]string, gatewayIP string) []PortStatus {
	portCount := make(map[int]int)
	for _, s := range servers {
		portCount[s.LocalPort]++
	}

	statuses := make([]PortStatus, 0, len(servers))
	for _, s := range servers {
		st := PortStatus{
			Port:             s.LocalPort,
			Section:          s.Section,
			Protocol:         s.Protocol,
			ServerAddress:    s.ServerAddress,
			PublishedOnProxy: proxyPorts[s.LocalPort],
		}

		if portCount[s.LocalPort] > 1 {
			st.Status = "conflict_config"
			st.Message = fmt.Sprintf("Conflito: porta %d repetida no emailproxy.config", s.LocalPort)
			statuses = append(statuses, st)
			continue
		}

		if proxyPorts != nil && proxyPorts[s.LocalPort] {
			st.Status = "open_on_proxy"
			st.Message = "Aberta no Docker (publicada no container do proxy)"
			statuses = append(statuses, st)
			continue
		}

		if otherPorts != nil {
			if owner, busy := otherPorts[s.LocalPort]; busy {
				st.Status = "conflict_docker"
				st.ConflictContainer = owner
				st.Message = fmt.Sprintf("Ocupada pelo container Docker '%s'", owner)
				statuses = append(statuses, st)
				continue
			}
		}

		if gatewayIP != "" && canDialTCP(gatewayIP, s.LocalPort) {
			st.Status = "conflict_host"
			st.Message = fmt.Sprintf("Ocupada no servidor host (%s:%d respondendo)", gatewayIP, s.LocalPort)
			statuses = append(statuses, st)
			continue
		}

		st.Status = "available"
		if proxyPorts == nil {
			st.Message = "Disponível na configuração (Docker socket indisponível para checagem no host)"
		} else {
			st.Message = "Livre para abrir no Docker (só interna no momento)"
		}
		statuses = append(statuses, st)
	}
	return statuses
}

func canDialTCP(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

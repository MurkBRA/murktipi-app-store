//go:build !linux

package reload

import (
	"fmt"
	"os"

	"email-oauth2-proxy-ui/internal/config"
)

// InspectDockerPortsOverview returns static port/compose overview on non-Linux hosts.
func InspectDockerPortsOverview(_, _, composePath, composeService string, servers []config.ServerEntry) DockerPortsOverview {
	specs := SpecsFromServers(servers)
	resolvedCompose, composeWritable := ResolveComposeFile(composePath)
	if composeService == "" {
		composeService = "email-oauth2-proxy"
	}

	return DockerPortsOverview{
		DockerAvailable:  false,
		ComposeAvailable: composeWritable,
		ComposePath:      resolvedCompose,
		ComposeService:   composeService,
		PortsYAML:        RenderPortsSnippet(specs),
		PublishedPorts:   []int{},
		ProtectedPorts:   []int{},
		Statuses:         evaluatePortsStatic(servers, nil, nil, ""),
	}
}

// CheckPortsWithHostProbe verifies config conflicts on non-Linux hosts.
func CheckPortsWithHostProbe(_, _ string, servers []config.ServerEntry, candidatePorts []int) ([]PortStatus, error) {
	ov := InspectDockerPortsOverview("", "", "", "", servers)
	seen := make(map[int]bool)
	for _, s := range ov.Statuses {
		seen[s.Port] = true
	}
	for _, p := range candidatePorts {
		if p > 0 && !seen[p] {
			ov.Statuses = append(ov.Statuses, PortStatus{
				Port:    p,
				Section: fmt.Sprintf("PORT-%d", p),
				Status:  "available",
				Message: "Docker socket indisponível neste sistema",
			})
		}
	}
	return ov.Statuses, nil
}

// ApplyPorts allows compose_only updates on non-Linux hosts and returns a clear message for Docker modes.
func ApplyPorts(_, _, composePath, composeService, mode string, specs []PortSpec) ApplyPortsResult {
	specs = DeduplicateAndSortSpecs(specs)
	resolvedCompose, composeWritable := ResolveComposeFile(composePath)

	if mode == "compose_only" {
		if !composeWritable {
			return ApplyPortsResult{Mode: mode, Message: "Arquivo docker-compose.yml não encontrado ou sem permissão de escrita."}
		}
		if composeService == "" {
			composeService = "email-oauth2-proxy"
		}
		raw, err := os.ReadFile(resolvedCompose)
		if err != nil {
			return ApplyPortsResult{Mode: mode, Message: err.Error()}
		}
		updated, err := UpdateComposePortsYAML(string(raw), composeService, specs)
		if err != nil {
			return ApplyPortsResult{Mode: mode, Message: err.Error()}
		}
		if err := WriteComposeFile(resolvedCompose, updated); err != nil {
			return ApplyPortsResult{Mode: mode, Message: err.Error()}
		}
		return ApplyPortsResult{
			OK:             true,
			Mode:           mode,
			ComposeUpdated: true,
			ComposePath:    resolvedCompose,
			Message:        fmt.Sprintf("Arquivo %s atualizado!", resolvedCompose),
		}
	}

	return ApplyPortsResult{
		Mode:    mode,
		Message: "Abertura automática via Docker socket só está disponível no servidor Linux/Docker.",
	}
}

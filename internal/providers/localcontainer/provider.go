package localcontainer

import (
	"flag"
	"strings"

	core "github.com/openclaw/crabbox/internal/cli"
)

func init() {
	core.RegisterProvider(Provider{})
}

type Provider struct{}

func (Provider) Name() string { return providerName }

func (Provider) Aliases() []string {
	return []string{"docker", "container", "local-docker"}
}

func (Provider) Spec() core.ProviderSpec {
	return core.ProviderSpec{
		Name:        providerName,
		Family:      "container",
		Kind:        core.ProviderKindSSHLease,
		Targets:     []core.TargetSpec{{OS: core.TargetLinux}},
		Features:    core.FeatureSet{core.FeatureSSH, core.FeatureCrabboxSync, core.FeatureCleanup, core.FeatureDesktop, core.FeatureBrowser, core.FeatureCacheVolume, core.FeatureCheckpoint, core.FeatureFork},
		Coordinator: core.CoordinatorNever,
	}
}

func (Provider) RegisterFlags(fs *flag.FlagSet, defaults core.Config) any {
	return registerFlags(fs, defaults)
}

func (Provider) ApplyFlags(cfg *core.Config, fs *flag.FlagSet, values any) error {
	return applyFlags(cfg, fs, values)
}

func (p Provider) Configure(cfg core.Config, rt core.Runtime) (core.Backend, error) {
	if cfg.TargetOS != "" && cfg.TargetOS != core.TargetLinux {
		return nil, core.Exit(2, "provider=%s supports target=linux only", providerName)
	}
	if cfg.Tailscale.Enabled || string(cfg.Network) == "tailscale" {
		return nil, core.Exit(2, "--tailscale is not supported for provider=%s; use a remote SSH provider when tailnet reachability is required", providerName)
	}
	return newBackend(p.Spec(), cfg, rt), nil
}

func (Provider) NativeCheckpointCapability(req core.NativeCheckpointRequest) (core.NativeCheckpointCapability, bool) {
	if req.Server.CloudID == "" {
		return core.NativeCheckpointCapability{}, false
	}
	if req.Config.LocalContainer.DockerSocket || leaseHasDockerSocket(req.Server) {
		return core.NativeCheckpointCapability{}, false
	}
	// docker-commit is a native/auto-mode checkpoint, not an image-mode one;
	// reject the capability when image strategy is explicitly requested.
	if core.IsImageCheckpointStrategy(req.Strategy) {
		return core.NativeCheckpointCapability{}, false
	}
	return core.NativeCheckpointCapability{Kind: core.CheckpointKindDockerCommit, Direct: true}, true
}

// leaseHasDockerSocket reports whether a resolved lease was created with
// docker-socket mode (recorded on its labels). docker-commit checkpoints are
// skipped for those leases because the host work-root mount masks the committed
// workspace; the config flag alone misses leases whose mode is on the labels.
func leaseHasDockerSocket(server core.Server) bool {
	switch server.Labels["docker_socket"] {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

// ApplyNativeCheckpointForkConfig points a new lease at a docker-commit
// checkpoint image so `crabbox checkpoint fork` launches the box from the
// committed image instead of the default base image.
func (Provider) ApplyNativeCheckpointForkConfig(req core.NativeCheckpointForkRequest) error {
	if req.Record.Kind != core.CheckpointKindDockerCommit {
		return core.Exit(2, "provider=%s does not support checkpoint kind=%s", providerName, req.Record.Kind)
	}
	image := req.Record.Resource
	if image == "" {
		image = req.Record.ImageID
	}
	if image == "" {
		return core.Exit(2, "local-container checkpoint fork requires a committed image reference")
	}
	req.Config.LocalContainer.Image = image
	// docker-socket mode mounts a host work-root over the container work-root,
	// which would mask the committed image's saved workspace; disable it for
	// docker-commit forks so the checkpointed workspace is preserved.
	req.Config.LocalContainer.DockerSocket = false
	// Replay the daemon scope the checkpoint was created against so the fork
	// launches on the daemon the committed image lives on, not whatever Docker
	// context/host is ambient at fork time.
	if rt := strings.TrimSpace(req.Record.Runtime); rt != "" {
		req.Config.LocalContainer.Runtime = rt
	}
	req.Config.LocalContainer.DockerHost = strings.TrimSpace(req.Record.DockerHost)
	req.Config.LocalContainer.DockerContext = strings.TrimSpace(req.Record.DockerContext)
	return nil
}

func (p Provider) ConfigureDoctor(cfg core.Config, rt core.Runtime) (core.DoctorBackend, error) {
	backend, err := p.Configure(cfg, rt)
	if err != nil {
		return nil, err
	}
	doctor, ok := backend.(core.DoctorBackend)
	if !ok {
		return nil, core.Exit(2, "%s doctor backend unavailable", providerName)
	}
	return doctor, nil
}

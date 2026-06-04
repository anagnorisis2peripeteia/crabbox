package localcontainer

import (
	"testing"

	core "github.com/openclaw/crabbox/internal/cli"
)

func TestSpecAdvertisesForkFeature(t *testing.T) {
	if !(Provider{}).Spec().Features.Has(core.FeatureFork) {
		t.Fatal("expected provider spec to advertise FeatureFork now that fork is implemented")
	}
}

func TestNativeCheckpointCapabilityReturnsDockerCommit(t *testing.T) {
	cap, ok := Provider{}.NativeCheckpointCapability(core.NativeCheckpointRequest{
		Server: core.Server{CloudID: "abc123"},
	})
	if !ok {
		t.Fatal("expected capability to be supported")
	}
	if cap.Kind != core.CheckpointKindDockerCommit {
		t.Fatalf("Kind=%q, want %q", cap.Kind, core.CheckpointKindDockerCommit)
	}
	if !cap.Direct {
		t.Fatal("Direct=false, want true")
	}
}

func TestNativeCheckpointCapabilityRequiresCloudID(t *testing.T) {
	_, ok := Provider{}.NativeCheckpointCapability(core.NativeCheckpointRequest{
		Server: core.Server{},
	})
	if ok {
		t.Fatal("expected capability to be unsupported without CloudID")
	}
}

func TestNativeCheckpointCapabilitySkipsImageStrategy(t *testing.T) {
	_, ok := Provider{}.NativeCheckpointCapability(core.NativeCheckpointRequest{
		Server:   core.Server{CloudID: "abc123"},
		Strategy: "image",
	})
	if ok {
		t.Fatal("expected capability to be unsupported with strategy=image (docker-commit is native/auto-mode)")
	}
}

func TestNativeCheckpointCapabilitySkipsDockerSocket(t *testing.T) {
	_, ok := Provider{}.NativeCheckpointCapability(core.NativeCheckpointRequest{
		Server: core.Server{CloudID: "abc123"},
		Config: core.Config{LocalContainer: core.LocalContainerConfig{DockerSocket: true}},
	})
	if ok {
		t.Fatal("expected capability to be unsupported with docker-socket")
	}
}

func TestNativeCheckpointCapabilitySkipsDockerSocketLabel(t *testing.T) {
	_, ok := Provider{}.NativeCheckpointCapability(core.NativeCheckpointRequest{
		Server: core.Server{CloudID: "abc123", Labels: map[string]string{"docker_socket": "1"}},
	})
	if ok {
		t.Fatal("expected capability to be unsupported when the lease label marks docker-socket mode")
	}
}

func TestApplyNativeCheckpointForkConfigSetsImage(t *testing.T) {
	cfg := core.Config{}
	if err := (Provider{}).ApplyNativeCheckpointForkConfig(core.NativeCheckpointForkRequest{
		Config: &cfg,
		Record: core.NativeCheckpointForkRecord{
			Kind:    core.CheckpointKindDockerCommit,
			ImageID: "crabbox-checkpoint-demo",
		},
	}); err != nil {
		t.Fatalf("ApplyNativeCheckpointForkConfig: %v", err)
	}
	if cfg.LocalContainer.Image != "crabbox-checkpoint-demo" {
		t.Fatalf("Image=%q, want crabbox-checkpoint-demo", cfg.LocalContainer.Image)
	}
}

func TestApplyNativeCheckpointForkConfigPrefersResource(t *testing.T) {
	cfg := core.Config{}
	if err := (Provider{}).ApplyNativeCheckpointForkConfig(core.NativeCheckpointForkRequest{
		Config: &cfg,
		Record: core.NativeCheckpointForkRecord{
			Kind:     core.CheckpointKindDockerCommit,
			Resource: "crabbox-checkpoint-res",
			ImageID:  "sha256:deadbeef",
		},
	}); err != nil {
		t.Fatalf("ApplyNativeCheckpointForkConfig: %v", err)
	}
	if cfg.LocalContainer.Image != "crabbox-checkpoint-res" {
		t.Fatalf("Image=%q, want crabbox-checkpoint-res", cfg.LocalContainer.Image)
	}
}

func TestApplyNativeCheckpointForkConfigRejectsOtherKinds(t *testing.T) {
	cfg := core.Config{}
	if err := (Provider{}).ApplyNativeCheckpointForkConfig(core.NativeCheckpointForkRequest{
		Config: &cfg,
		Record: core.NativeCheckpointForkRecord{Kind: "workspace-archive", ImageID: "x"},
	}); err == nil {
		t.Fatal("expected error for non docker-commit checkpoint kind")
	}
}

func TestApplyNativeCheckpointForkConfigDisablesDockerSocket(t *testing.T) {
	cfg := core.Config{}
	cfg.LocalContainer.DockerSocket = true
	if err := (Provider{}).ApplyNativeCheckpointForkConfig(core.NativeCheckpointForkRequest{
		Config: &cfg,
		Record: core.NativeCheckpointForkRecord{Kind: core.CheckpointKindDockerCommit, ImageID: "crabbox-checkpoint-x"},
	}); err != nil {
		t.Fatalf("ApplyNativeCheckpointForkConfig: %v", err)
	}
	if cfg.LocalContainer.DockerSocket {
		t.Fatal("DockerSocket should be cleared for a docker-commit fork to avoid masking the committed workdir")
	}
}

func TestApplyNativeCheckpointForkConfigReplaysDaemonScope(t *testing.T) {
	cfg := core.Config{}
	if err := (Provider{}).ApplyNativeCheckpointForkConfig(core.NativeCheckpointForkRequest{
		Config: &cfg,
		Record: core.NativeCheckpointForkRecord{
			Kind:          core.CheckpointKindDockerCommit,
			ImageID:       "crabbox-checkpoint-scoped",
			Runtime:       "podman",
			DockerHost:    "tcp://10.0.0.5:2376",
			DockerContext: "remote-ctx",
		},
	}); err != nil {
		t.Fatalf("ApplyNativeCheckpointForkConfig: %v", err)
	}
	if cfg.LocalContainer.Runtime != "podman" {
		t.Fatalf("Runtime=%q, want podman (fork must replay the checkpoint runtime)", cfg.LocalContainer.Runtime)
	}
	if cfg.LocalContainer.DockerHost != "tcp://10.0.0.5:2376" {
		t.Fatalf("DockerHost=%q, want the recorded host so the fork targets the checkpoint daemon", cfg.LocalContainer.DockerHost)
	}
	if cfg.LocalContainer.DockerContext != "remote-ctx" {
		t.Fatalf("DockerContext=%q, want remote-ctx", cfg.LocalContainer.DockerContext)
	}
}

package localcontainer

import (
	"testing"

	core "github.com/openclaw/crabbox/internal/cli"
)

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

func TestNativeCheckpointCapabilitySkipsDockerSocket(t *testing.T) {
	_, ok := Provider{}.NativeCheckpointCapability(core.NativeCheckpointRequest{
		Server: core.Server{CloudID: "abc123"},
		Config: core.Config{DockerSocket: true},
	})
	if ok {
		t.Fatal("expected capability to be unsupported with docker-socket")
	}
}

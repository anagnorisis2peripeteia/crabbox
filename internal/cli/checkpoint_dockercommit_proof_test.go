//go:build dockerproof

package cli

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// TestDockerCommitCheckpointLifecycleProof exercises the real local-container
// docker-commit native checkpoint lifecycle against a live Docker container:
//
//	create  -> directDockerCommitCheckpointDriver.Create (docker commit)
//	verify  -> docker image inspect (the same command checkpoint verify runs)
//	delete  -> docker rmi
//
// It is gated behind the `dockerproof` build tag so it never runs in the
// default unit suite; it needs a working Docker daemon.
//
//	go test -tags dockerproof -run TestDockerCommitCheckpointLifecycleProof -v ./internal/cli/
func TestDockerCommitCheckpointLifecycleProof(t *testing.T) {
	const runtime = "docker"
	if _, err := exec.LookPath(runtime); err != nil {
		t.Skipf("%s not on PATH: %v", runtime, err)
	}

	// A throwaway running container stands in for a local-container lease.
	out, err := exec.Command(runtime, "run", "-d", "alpine:3", "sleep", "600").CombinedOutput()
	if err != nil {
		t.Fatalf("docker run: %v: %s", err, out)
	}
	containerID := lastLine(string(out))
	t.Cleanup(func() { _ = exec.Command(runtime, "rm", "-f", containerID).Run() })
	t.Logf("source container: %s", short(containerID))

	// CREATE — real driver: docker commit <container> crabbox-checkpoint-proof
	img, err := directDockerCommitCheckpointDriver{}.Create(context.Background(), checkpointNativeCreateRequest{
		Server: Server{CloudID: containerID},
		Name:   "proof",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if img.State != "available" || img.Kind != checkpointKindDockerCommit || !img.Direct {
		t.Fatalf("unexpected checkpoint image: %+v", img)
	}
	t.Cleanup(func() { _ = exec.Command(runtime, "rmi", "-f", img.Name).Run() })
	t.Logf("CREATE: image=%s id=%s state=%s kind=%s direct=%v", img.Name, short(img.ID), img.State, img.Kind, img.Direct)

	// DUP-NAME coverage — a second checkpoint with the same --name must not
	// retag the first; identity is the per-image digest.
	img2, err := directDockerCommitCheckpointDriver{}.Create(context.Background(), checkpointNativeCreateRequest{
		Server: Server{CloudID: containerID},
		Name:   "proof",
	})
	if err != nil {
		t.Fatalf("Create #2: %v", err)
	}
	t.Cleanup(func() { _ = exec.Command(runtime, "rmi", "-f", img2.Name).Run() })
	if img2.ID == img.ID || img2.Name == img.Name {
		t.Fatalf("duplicate --name reused identity: #1=%s/%s #2=%s/%s", img.Name, short(img.ID), img2.Name, short(img2.ID))
	}
	t.Logf("DUP-NAME: second checkpoint is distinct -> image=%s id=%s", img2.Name, short(img2.ID))

	// VERIFY — same command checkpoint verify uses for docker-commit records.
	insp, err := exec.Command(runtime, "image", "inspect", img.Name, "--format", "{{.Id}}").CombinedOutput()
	if err != nil {
		t.Fatalf("verify (image inspect): %v: %s", err, insp)
	}
	t.Logf("VERIFY: image present (next_action would be delete_local), id=%s", short(strings.TrimSpace(string(insp))))

	// DELETE — docker rmi.
	rmiOut, err := exec.Command(runtime, "rmi", img.Name).CombinedOutput()
	if err != nil {
		t.Fatalf("delete (rmi): %v: %s", err, rmiOut)
	}
	t.Logf("DELETE: %s", strings.TrimSpace(string(rmiOut)))

	// CONFIRM removed.
	if err := exec.Command(runtime, "image", "inspect", img.Name).Run(); err == nil {
		t.Fatalf("image %s still present after delete", img.Name)
	}
	t.Logf("CONFIRMED: %s removed", img.Name)
}

func short(s string) string {
	s = strings.TrimPrefix(s, "sha256:")
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// lastLine returns the final non-empty line, so `docker run -d` output is read
// as the container id even when image-pull progress precedes it.
func lastLine(s string) string {
	parts := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(parts[len(parts)-1])
}

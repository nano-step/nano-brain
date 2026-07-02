package main

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// dockerCall records a single invocation made through the fake runDocker.
type dockerCall struct {
	args []string
}

// dockerResponse is one scripted response for the fake runDocker to return.
type dockerResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

// withDockerRunner overrides the package-level runDocker var with a fake that
// pops scripted responses in order while recording each invocation's args,
// restoring the original via t.Cleanup — mirrors withRecoveryHooks
// (commands_test.go:500-525) and the runServeDaemonFn hook-var idiom
// (client.go:18-19).
func withDockerRunner(t *testing.T, script []dockerResponse) *[]dockerCall {
	t.Helper()
	orig := runDocker
	calls := &[]dockerCall{}
	idx := 0

	runDocker = func(ctx context.Context, args ...string) (string, string, int, error) {
		*calls = append(*calls, dockerCall{args: append([]string{}, args...)})
		if idx >= len(script) {
			t.Fatalf("withDockerRunner: no more scripted responses (call %d: docker %s)", idx, strings.Join(args, " "))
		}
		resp := script[idx]
		idx++
		return resp.stdout, resp.stderr, resp.exitCode, resp.err
	}

	t.Cleanup(func() {
		runDocker = orig
	})

	return calls
}

func TestDockerStatus(t *testing.T) {
	cases := []struct {
		name   string
		script []dockerResponse
		want   dockerStatusType
	}{
		{
			name: "not installed - exec.Error wrapping ErrNotFound",
			script: []dockerResponse{
				{stdout: "", stderr: "", exitCode: -1, err: &exec.Error{Name: "docker", Err: exec.ErrNotFound}},
			},
			want: dockerStatusNotInstalled,
		},
		{
			name: "daemon not running",
			script: []dockerResponse{
				{stdout: "", stderr: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?", exitCode: 1, err: nil},
			},
			want: dockerStatusDaemonNotRunning,
		},
		{
			name: "available",
			script: []dockerResponse{
				{stdout: "Server Version: 29.5.3", stderr: "", exitCode: 0, err: nil},
			},
			want: dockerStatusAvailable,
		},
		{
			name: "unknown error",
			script: []dockerResponse{
				{stdout: "", stderr: "some other docker error", exitCode: 1, err: nil},
			},
			want: dockerStatusUnknownError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := withDockerRunner(t, tc.script)
			got := dockerStatus(context.Background())
			if got != tc.want {
				t.Errorf("dockerStatus() = %q, want %q", got, tc.want)
			}
			if len(*calls) != 1 || (*calls)[0].args[0] != "info" {
				t.Errorf("expected single 'docker info' call, got %+v", *calls)
			}
		})
	}
}

func TestProvisionPostgres(t *testing.T) {
	t.Run("success path - single run, no start/rm calls", func(t *testing.T) {
		script := []dockerResponse{
			{stdout: "abc123", stderr: "", exitCode: 0, err: nil},
		}
		calls := withDockerRunner(t, script)

		url, err := provisionPostgres(context.Background())
		if err != nil {
			t.Fatalf("provisionPostgres() error = %v, want nil", err)
		}
		if url != "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev?sslmode=disable" {
			t.Errorf("url = %q, want :5432 URL", url)
		}
		if len(*calls) != 1 {
			t.Fatalf("expected exactly 1 docker call (run only), got %d: %+v", len(*calls), *calls)
		}
		if (*calls)[0].args[0] != "run" {
			t.Errorf("expected first call to be 'run', got %q", (*calls)[0].args[0])
		}
	})

	t.Run("name-conflict path - docker start recovers", func(t *testing.T) {
		script := []dockerResponse{
			{stdout: "", stderr: `docker: Error response from daemon: Conflict. The container name "/nanobrain-pg" is already in use by container "abc". You have to remove (or rename) that container to be able to reuse that name.`, exitCode: 125, err: nil},
			{stdout: "nanobrain-pg", stderr: "", exitCode: 0, err: nil},
		}
		calls := withDockerRunner(t, script)

		url, err := provisionPostgres(context.Background())
		if err != nil {
			t.Fatalf("provisionPostgres() error = %v, want nil", err)
		}
		if url != "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev?sslmode=disable" {
			t.Errorf("url = %q, want :5432 URL", url)
		}
		if len(*calls) != 2 {
			t.Fatalf("expected exactly 2 docker calls (run, start), got %d: %+v", len(*calls), *calls)
		}
		if (*calls)[0].args[0] != "run" {
			t.Errorf("call 0 = %q, want run", (*calls)[0].args[0])
		}
		if (*calls)[1].args[0] != "start" {
			t.Errorf("call 1 = %q, want start", (*calls)[1].args[0])
		}
	})

	t.Run("port-conflict path - rm stray then retry on 5433", func(t *testing.T) {
		script := []dockerResponse{
			{stdout: "", stderr: `docker: Error response from daemon: failed to set up container networking: driver failed programming external connectivity on endpoint nanobrain-pg2 (abc): Bind for 0.0.0.0:5432 failed: port is already allocated`, exitCode: 125, err: nil},
			{stdout: "nanobrain-pg", stderr: "", exitCode: 0, err: nil}, // docker rm
			{stdout: "def456", stderr: "", exitCode: 0, err: nil},       // retry run on 5433
		}
		calls := withDockerRunner(t, script)

		url, err := provisionPostgres(context.Background())
		if err != nil {
			t.Fatalf("provisionPostgres() error = %v, want nil", err)
		}
		if url != "postgres://nanobrain:nanobrain@localhost:5433/nanobrain_dev?sslmode=disable" {
			t.Errorf("url = %q, want :5433 URL", url)
		}
		if len(*calls) != 3 {
			t.Fatalf("expected exactly 3 docker calls (run, rm, run), got %d: %+v", len(*calls), *calls)
		}
		if (*calls)[0].args[0] != "run" {
			t.Errorf("call 0 = %q, want run", (*calls)[0].args[0])
		}
		if (*calls)[1].args[0] != "rm" {
			t.Errorf("call 1 = %q, want rm (stray cleanup between the two run invocations)", (*calls)[1].args[0])
		}
		if (*calls)[2].args[0] != "run" {
			t.Errorf("call 2 = %q, want run (retry)", (*calls)[2].args[0])
		}
		// Assert the retry targets port 5433, not 5432.
		found5433 := false
		for _, a := range (*calls)[2].args {
			if a == "5433:5433" {
				found5433 = true
			}
		}
		if !found5433 {
			t.Errorf("retry run args %v do not contain -p 5433:5433", (*calls)[2].args)
		}
	})

	t.Run("unrecovered exit returns descriptive error", func(t *testing.T) {
		script := []dockerResponse{
			{stdout: "", stderr: "some unrelated docker run failure", exitCode: 1, err: nil},
		}
		withDockerRunner(t, script)

		_, err := provisionPostgres(context.Background())
		if err == nil {
			t.Fatal("provisionPostgres() error = nil, want non-nil")
		}
	})

	t.Run("execution failure (binary missing) returns error", func(t *testing.T) {
		script := []dockerResponse{
			{stdout: "", stderr: "", exitCode: -1, err: &exec.Error{Name: "docker", Err: exec.ErrNotFound}},
		}
		withDockerRunner(t, script)

		_, err := provisionPostgres(context.Background())
		if err == nil {
			t.Fatal("provisionPostgres() error = nil, want non-nil")
		}
		if !errors.Is(err, exec.ErrNotFound) {
			t.Errorf("expected wrapped exec.ErrNotFound, got %v", err)
		}
	})
}

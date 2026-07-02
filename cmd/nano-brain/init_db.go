package main

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
)

// pingPostgres is the test seam over defaultPingPostgres. Tests override it
// to avoid a real Postgres connection.
var pingPostgres = defaultPingPostgres

// provisionPostgresFn is the test seam over provisionPostgres (declared in
// docker_provision.go, Plan 01). Declared here (not there) so this file owns
// the seam it depends on without modifying Plan 01's file.
var provisionPostgresFn = provisionPostgres

// defaultPingPostgres connects to dbURL and pings it, mirroring
// doctor.CheckPostgreSQL's connect+ping shape (internal/health/doctor/doctor.go:61-86)
// with a 3s per-attempt timeout. Returns nil when the connection is ready,
// or an error describing why it is not (connection refused, auth failure,
// ping failure, etc).
func defaultPingPostgres(ctx context.Context, dbURL string) error {
	connCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := pgx.Connect(connCtx, dbURL)
	if err != nil {
		return err
	}
	defer conn.Close(connCtx)

	return conn.Ping(connCtx)
}

// waitForPostgresReady polls pingPostgres every interval until it succeeds
// or timeout elapses, using the exact deadline/remaining/clamp structure of
// waitForServerHealthy (client_helpers.go:88-118). Any pingPostgres error is
// treated as transient until the deadline — this is what lets the poll
// survive Postgres's own restart-once-during-first-init behavior (Pitfall 2)
// rather than failing on the first connection-refused attempt.
func waitForPostgresReady(ctx context.Context, dbURL string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if err := pingPostgres(ctx, dbURL); err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("postgres did not become ready within %s", timeout)
		}

		remaining := time.Until(deadline)
		sleep := interval
		if remaining < sleep {
			sleep = remaining
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}
}

// hostOnly extracts just the host:port from a postgres:// URL for
// user-facing display, mirroring doctor.go:69-73 — never prints the full
// credentialed URL (which may contain a password).
func hostOnly(dbURL string) string {
	parsed, err := url.Parse(dbURL)
	if err != nil || parsed == nil {
		return "unknown"
	}
	return parsed.Host
}

// stepDatabase implements the D-05/D-08/D-09 database wizard step. It
// returns the dbURL to write to config and whether the step completed.
// ok=false whenever the user's stdin closes mid-prompt, per the
// promptConsequential EOF-means-decline convention (CR-01) — a closed
// stream is never treated as consent to save.
//
// Detection order (D-05):
//  1. pingPostgres(defaultURL) succeeds → return defaultURL, zero prompts.
//  2. Unreachable → dockerStatus(ctx):
//     a. Available → ask to provision via Docker; on accept, provision +
//        poll (D-08) and return the provisioned URL.
//     b. NotInstalled/DaemonNotRunning, or user declined the Docker
//        prompt → print install guidance and prompt for a remote URL,
//        live-validating (D-09) with a "save anyway" escape hatch.
func stepDatabase(scanner *bufio.Scanner, defaultURL string) (dbURL string, ok bool) {
	ctx := context.Background()

	if err := pingPostgres(ctx, defaultURL); err == nil {
		return defaultURL, true
	}

	fmt.Print("\n── Database ──\n")
	fmt.Printf("  Could not reach PostgreSQL at %s\n", hostOnly(defaultURL))

	status := dockerStatus(ctx)
	if status == dockerStatusAvailable {
		answer, promptOK := promptConsequential(scanner, "PostgreSQL not found. Start one via Docker with default settings?", "Y")
		if promptOK && isAffirmative(answer) {
			fmt.Println("  Pulling pgvector/pgvector:pg17 image (if needed) and starting container...")
			provisionedURL, err := provisionPostgresFn(ctx)
			if err != nil {
				fmt.Printf("  Docker provisioning failed: %v\n", err)
				return promptRemoteURL(scanner)
			}
			fmt.Println("  Waiting for PostgreSQL to become ready...")
			if err := waitForPostgresReady(ctx, provisionedURL, 30*time.Second, 500*time.Millisecond); err != nil {
				fmt.Printf("  %v\n", err)
				return promptRemoteURL(scanner)
			}
			fmt.Printf("  ✓ PostgreSQL ready at %s\n", hostOnly(provisionedURL))
			return provisionedURL, true
		}
	} else {
		fmt.Println("  Docker not available.")
		fmt.Println("  Install Docker: https://docs.docker.com/get-docker/")
		fmt.Println("  (nano-brain requires the pgvector extension — the pgvector/pgvector:pg17 image includes it)")
	}

	return promptRemoteURL(scanner)
}

// promptRemoteURL implements D-09: prompt for a Postgres URL, live-validate
// it via pingPostgres before accepting. On validation failure, it re-prompts
// for a URL; the "save anyway" escape hatch is a DELIBERATE blank answer at
// that re-prompt (Enter on an open TTY), which returns the last-entered
// (unvalidated) URL with a printed warning, so an intentionally-offline
// setup isn't blocked. A closed stdin (promptOK=false) is a decline per
// CR-01, never an implicit save-anyway.
func promptRemoteURL(scanner *bufio.Scanner) (dbURL string, ok bool) {
	ctx := context.Background()
	lastEntered := ""

	for {
		answer, promptOK := promptConsequential(scanner, "PostgreSQL URL (postgres://user:pass@host:port/db)", "")
		if !promptOK {
			// EOF/closed stdin is a decline (CR-01) — never consent to save.
			return "", false
		}
		if answer == "" {
			if lastEntered != "" {
				// Deliberate blank after at least one prior entry is the
				// "save anyway" escape hatch (D-09).
				fmt.Println("  Saving without validation (save anyway).")
				return lastEntered, true
			}
			// No prior attempt to fall back to and the user gave no URL —
			// re-prompt rather than saving an empty database.url.
			continue
		}

		// T-13-05 V5: only postgres:// / postgresql:// URLs reach pgx.
		if parsed, err := url.Parse(answer); err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
			fmt.Println("  Not a postgres:// URL — expected postgres://user:pass@host:port/db")
			continue
		}

		lastEntered = answer

		if err := pingPostgres(ctx, answer); err != nil {
			fmt.Printf("  Could not connect to %s: %v\n", hostOnly(answer), err)
			fmt.Println("  Enter a different URL, or press Enter to save this one anyway.")
			continue
		}

		fmt.Printf("  ✓ PostgreSQL reachable at %s\n", hostOnly(answer))
		return answer, true
	}
}

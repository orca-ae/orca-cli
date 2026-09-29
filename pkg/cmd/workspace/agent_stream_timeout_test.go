// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

type streamDeadlineTransport func(*http.Request) (*http.Response, error)

func (f streamDeadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestManagedAgentStreamClientPreservesCallerDeadline(t *testing.T) {
	originalTransport, originalOptions := http.DefaultTransport, currentOptions
	t.Cleanup(func() { http.DefaultTransport, currentOptions = originalTransport, originalOptions })
	for _, tokenType := range []string{"bearer", "api-key"} {
		for _, timeout := range []time.Duration{0, 5 * time.Minute} {
			t.Run(fmt.Sprintf("%s/%s", tokenType, timeout), func(t *testing.T) {
				currentOptions = &Options{RegistryURL: "https://registry.example.com"}
				if tokenType == "bearer" {
					currentOptions.AccessToken = "test-token"
				} else {
					currentOptions.APIKey = "test-key"
				}
				ctx := context.Background()
				if timeout != 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, timeout)
					defer cancel()
				}
				wantDeadline, wantSet := ctx.Deadline()
				called := false
				http.DefaultTransport = streamDeadlineTransport(func(req *http.Request) (*http.Response, error) {
					called = true
					deadline, set := req.Context().Deadline()
					if set != wantSet || !deadline.Equal(wantDeadline) {
						t.Errorf("request deadline = %v, %v; want caller deadline %v, %v", deadline, set, wantDeadline, wantSet)
					}
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
						Body: io.NopCloser(strings.NewReader(": heartbeat\n\n")), Request: req}, nil
				})
				client, err := newWorkspaceManagedAgentsStreamClient()
				if err != nil {
					t.Fatal(err)
				}
				if err := streamManagedAgentSSE(client, ctx, "/v1/sessions/test/events/stream", io.Discard); err != nil {
					t.Fatal(err)
				}
				if !called {
					t.Fatal("stream did not issue a request")
				}
			})
		}
	}
}

func TestSessionStreamsOnlySuppressTheirOwnTimeout(t *testing.T) {
	original := newWorkspaceManagedAgentsStreamClient
	t.Cleanup(func() { newWorkspaceManagedAgentsStreamClient = original })
	o := &agentOptions{ioStreams: IOStreams{Out: io.Discard, ErrOut: io.Discard}}
	for _, thread := range []bool{false, true} {
		for _, ownTimeout := range []bool{false, true} {
			t.Run(fmt.Sprintf("thread=%v/own-timeout=%v", thread, ownTimeout), func(t *testing.T) {
				newWorkspaceManagedAgentsStreamClient = func() (workspaceManagedAgentsClient, error) {
					return &workspaceManagedAgentsClientMock{getStreamFn: func(ctx context.Context, _ string, _ string, _ func(io.Reader) error) error {
						if ownTimeout {
							<-ctx.Done()
						}
						return fmt.Errorf("request timed out: %w", context.DeadlineExceeded)
					}}, nil
				}
				var cmd *cobra.Command
				args := []string{"--session", "test", "--timeout", "10ms"}
				if thread {
					cmd = o.newSessionThreadEventStreamCommand()
					args = append(args, "--thread", "test-thread")
				} else {
					cmd = o.newSessionEventStreamCommand()
				}
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				cmd.SetArgs(args)
				err := cmd.Execute()
				if ownTimeout && err != nil {
					t.Fatalf("configured stream timeout should stop cleanly: %v", err)
				}
				if !ownTimeout && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("upstream timeout was swallowed: %v", err)
				}
			})
		}
	}
}

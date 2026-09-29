// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHealthCommandsUseExpectedEndpoints(t *testing.T) {
	for _, tc := range []struct {
		args []string
		path string
	}{
		{path: "/apis/cloud.sn.io/v1/health"},
		{args: []string{"ready"}, path: "/apis/cloud.sn.io/v1/health/ready"},
		{args: []string{"live"}, path: "/apis/cloud.sn.io/v1/health/live"},
	} {
		tc := tc
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/apis" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"cloud.sn.io"}]}`))
					return
				}
				if r.URL.Path != tc.path {
					t.Fatalf("path = %q, want %q", r.URL.Path, tc.path)
				}
				_, _ = w.Write([]byte(`true`))
			}))
			defer server.Close()

			var output bytes.Buffer
			opts := &Options{
				IOStreams: IOStreams{Out: &output, ErrOut: io.Discard},
				ResolveRuntime: func(context.Context, *cobra.Command, *Options) (RegistryRuntime, error) {
					return RegistryRuntime{BaseURL: server.URL, AccessToken: "token", HTTPClient: server.Client()}, nil
				},
			}
			cmd := NewCmdHealth(opts)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if output.String() != "true\n" {
				t.Fatalf("output = %q", output.String())
			}
		})
	}
}

// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

// IOStreams carries the input/output streams used by workspace commands.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// RegistryRuntime is the resolved registry connection information for a command.
type RegistryRuntime struct {
	BaseURL     string
	AccessToken string
	APIKey      string
	HTTPClient  *http.Client
}

// Options configures reusable workspace resource commands.
type Options struct {
	IOStreams IOStreams

	// Standalone flags. Embedders may ignore these by providing ResolveRuntime.
	RegistryURL string
	AccessToken string
	APIKey      string

	ResolveRuntime   func(context.Context, *cobra.Command, *Options) (RegistryRuntime, error)
	SelectConnection func(context.Context) (string, error)

	discoveryOnce   sync.Once
	discoveryGroups *registry.APIGroupList
	discoveryErr    error

	warningMu            sync.RWMutex
	warningOut           io.Writer
	legacyURLWarningOnce sync.Once
}

type onceWriter struct {
	once   *sync.Once
	writer io.Writer
}

func (w onceWriter) Write(p []byte) (int, error) {
	written := len(p)
	var writeErr error
	w.once.Do(func() {
		written, writeErr = w.writer.Write(p)
	})
	return written, writeErr
}

var currentOptions *Options

func setCurrentOptions(opts *Options) *Options {
	if opts == nil {
		opts = &Options{}
	}
	currentOptions = opts
	return opts
}

func (o *Options) resolveRuntime(ctx context.Context, cmd *cobra.Command) (RegistryRuntime, error) {
	runtime, err := o.resolveRuntimeWithoutAuth(ctx, cmd)
	if err != nil {
		return RegistryRuntime{}, err
	}
	hasAccessToken := strings.TrimSpace(runtime.AccessToken) != ""
	hasAPIKey := strings.TrimSpace(runtime.APIKey) != ""
	if hasAccessToken && hasAPIKey {
		return RegistryRuntime{}, fmt.Errorf("--access-token and --api-key cannot be used together")
	}
	if !hasAccessToken && !hasAPIKey {
		return RegistryRuntime{}, fmt.Errorf("one of --access-token or --api-key is required")
	}
	return runtime, nil
}

func (o *Options) resolveRuntimeWithoutAuth(ctx context.Context, cmd *cobra.Command) (RegistryRuntime, error) {
	if o == nil {
		return RegistryRuntime{}, fmt.Errorf("workspace options are not configured")
	}
	if o.ResolveRuntime != nil {
		return o.ResolveRuntime(ctx, cmd, o)
	}
	if strings.TrimSpace(o.RegistryURL) == "" {
		return RegistryRuntime{}, fmt.Errorf("--registry-url is required")
	}
	return RegistryRuntime{BaseURL: o.RegistryURL, AccessToken: o.AccessToken, APIKey: o.APIKey}, nil
}

func (o *Options) newRegistryClient(ctx context.Context, cmd *cobra.Command, httpClient *http.Client) (*registry.Client, error) {
	rt, err := o.resolveRuntime(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if httpClient != nil {
		rt.HTTPClient = httpClient
	}
	if strings.TrimSpace(rt.APIKey) != "" {
		return registry.NewAPIKeyClientWithWarningWriter(rt.BaseURL, rt.APIKey, rt.HTTPClient, o.legacyURLWarningWriter(cmd))
	}
	return registry.NewClientWithWarningWriter(rt.BaseURL, rt.AccessToken, rt.HTTPClient, o.legacyURLWarningWriter(cmd))
}

func (o *Options) newUnauthenticatedRegistryClient(ctx context.Context, cmd *cobra.Command, httpClient *http.Client) (*registry.Client, error) {
	rt, err := o.resolveRuntimeWithoutAuth(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if httpClient != nil {
		rt.HTTPClient = httpClient
	}
	return registry.NewUnauthenticatedClientWithWarningWriter(rt.BaseURL, rt.HTTPClient, o.legacyURLWarningWriter(cmd))
}

func (o *Options) discoverAPIGroups(ctx context.Context, cmd *cobra.Command) (*registry.APIGroupList, error) {
	o.discoveryOnce.Do(func() {
		client, err := o.newRegistryClient(ctx, cmd, nil)
		if err != nil {
			o.discoveryErr = err
			return
		}
		o.discoveryGroups, o.discoveryErr = client.GetAPIGroups(ctx)
	})
	return o.discoveryGroups, o.discoveryErr
}

func (o *Options) commandErrorWriter(cmd *cobra.Command) io.Writer {
	if cmd != nil {
		writer := cmd.ErrOrStderr()
		if file, ok := writer.(*os.File); ok && file == os.Stderr && o.IOStreams.ErrOut != nil {
			writer = o.IOStreams.ErrOut
		}
		o.warningMu.Lock()
		o.warningOut = writer
		o.warningMu.Unlock()
		return writer
	}

	o.warningMu.RLock()
	writer := o.warningOut
	o.warningMu.RUnlock()
	if writer != nil {
		return writer
	}
	if o.IOStreams.ErrOut != nil {
		return o.IOStreams.ErrOut
	}
	return os.Stderr
}

func (o *Options) legacyURLWarningWriter(cmd *cobra.Command) io.Writer {
	return onceWriter{once: &o.legacyURLWarningOnce, writer: o.commandErrorWriter(cmd)}
}

// NewConnectionsClient creates a registry connections client for embedders that need helper hooks.
func (o *Options) NewConnectionsClient(ctx context.Context, cmd *cobra.Command) (workspaceConnectionsClient, error) {
	client, err := o.newRegistryClient(ctx, cmd, nil)
	if err != nil {
		return nil, err
	}
	return registry.NewConnectionsClient(client), nil
}

func (o *Options) resolveConnectionSelection(ctx context.Context, explicit string, useInteractive bool) (string, error) {
	if explicit != "" && useInteractive {
		return "", fmt.Errorf("--connection and --use-connection cannot be used together")
	}
	if explicit != "" {
		return explicit, nil
	}
	if useInteractive {
		if o == nil || o.SelectConnection == nil {
			return "", fmt.Errorf("--use-connection is not supported; pass --connection")
		}
		return o.SelectConnection(ctx)
	}
	return "", nil
}

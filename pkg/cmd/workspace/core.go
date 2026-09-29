// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"fmt"
	"io"
	"strings"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

// NewCmdAPIVersions creates the core API version discovery command.
func NewCmdAPIVersions(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	output := "text"
	cmd := &cobra.Command{
		Use:    "api-versions",
		Short:  "List the core API versions this deployment serves",
		Args:   cobra.NoArgs,
		PreRun: configureWorkspaceWarningWriter,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := opts.newRegistryClient(cmd.Context(), cmd, nil)
			if err != nil {
				return err
			}
			versions, err := client.GetAPIVersions(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list core API versions: %w", err)
			}
			return renderAPIVersions(opts.IOStreams.Out, output, versions)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

// NewCmdHealthz creates the unauthenticated core liveness probe command.
func NewCmdHealthz(opts *Options) *cobra.Command {
	return newCoreProbeCommand(opts, "healthz", "Check core service liveness", func(ctx context.Context, client *registry.Client) (*registry.CoreProbeStatus, error) {
		return client.GetHealthz(ctx)
	})
}

// NewCmdReadyz creates the unauthenticated core readiness probe command.
func NewCmdReadyz(opts *Options) *cobra.Command {
	return newCoreProbeCommand(opts, "readyz", "Check core service readiness", func(ctx context.Context, client *registry.Client) (*registry.CoreProbeStatus, error) {
		return client.GetReadyz(ctx)
	})
}

func newCoreProbeCommand(
	opts *Options,
	use string,
	short string,
	probe func(context.Context, *registry.Client) (*registry.CoreProbeStatus, error),
) *cobra.Command {
	opts = setCurrentOptions(opts)
	output := "text"
	cmd := &cobra.Command{
		Use:    use,
		Short:  short,
		Args:   cobra.NoArgs,
		PreRun: configureWorkspaceWarningWriter,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := opts.newUnauthenticatedRegistryClient(cmd.Context(), cmd, nil)
			if err != nil {
				return err
			}
			status, err := probe(cmd.Context(), client)
			if err != nil {
				return fmt.Errorf("core %s probe failed: %w", use, err)
			}
			return renderCoreProbeStatus(opts.IOStreams.Out, output, status)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func renderAPIVersions(writer io.Writer, output string, versions *registry.APIVersions) error {
	return renderWorkspaceResource(writer, output, versions, func(writer io.Writer) error {
		if versions == nil {
			return nil
		}
		_, err := fmt.Fprintf(writer, "Preferred Version: %s\nVersions: %s\n",
			versions.PreferredVersion, strings.Join(versions.Versions, ", "))
		return err
	})
}

func renderCoreProbeStatus(writer io.Writer, output string, status *registry.CoreProbeStatus) error {
	return renderWorkspaceResource(writer, output, status, func(writer io.Writer) error {
		if status == nil {
			return nil
		}
		_, err := fmt.Fprintf(writer, "Status: %s\nService: %s\n", status.Status, status.Service)
		return err
	})
}

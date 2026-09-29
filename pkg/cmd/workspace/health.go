// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"fmt"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

type healthOptions struct {
	ioStreams IOStreams
}

// NewCmdHealth creates registry service health commands.
func NewCmdHealth(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &healthOptions{ioStreams: opts.IOStreams}
	cmd := o.newProbeCommand("health", "Check workspace registry health", func(ctx context.Context, client *registry.HealthClient) (bool, error) {
		return client.Health(ctx)
	})
	cmd.PreRunE = requireCloudExtensionOperation
	cmd.PersistentPreRunE = requireCloudExtension
	cmd.AddCommand(
		o.newProbeCommand("ready", "Check workspace registry readiness", func(ctx context.Context, client *registry.HealthClient) (bool, error) {
			return client.Ready(ctx)
		}),
		o.newProbeCommand("live", "Check workspace registry liveness", func(ctx context.Context, client *registry.HealthClient) (bool, error) {
			return client.Live(ctx)
		}),
	)
	return cmd
}

func (o *healthOptions) newProbeCommand(
	use string,
	short string,
	probe func(context.Context, *registry.HealthClient) (bool, error),
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			baseClient, err := newWorkspaceRegistryClient()
			if err != nil {
				return err
			}
			healthy, err := probe(cmd.Context(), registry.NewHealthClient(baseClient))
			if err != nil {
				return fmt.Errorf("workspace registry %s probe failed: %w", use, err)
			}
			_, err = fmt.Fprintln(o.ioStreams.Out, healthy)
			return err
		},
	}
}

// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/olekukonko/tablewriter"
	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

type apiGroupsOptions struct {
	opts      *Options
	ioStreams IOStreams
}

// NewCmdAPIGroups creates the api-groups discovery command.
func NewCmdAPIGroups(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &apiGroupsOptions{
		opts:      opts,
		ioStreams: opts.IOStreams,
	}

	output := "text"
	cmd := &cobra.Command{
		Use:   "api-groups",
		Short: "List the extension API groups this deployment advertises",
		Long: "Call authenticated GET /apis at the deployment host root. The response may include " +
			"installed groups such as policy.runorca.ai, pricing.runorca.ai, or cloud.sn.io; " +
			"an empty list means no extensions are installed.",
		Args:   cobra.NoArgs,
		PreRun: configureWorkspaceWarningWriter,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceDiscoveryClient()
			if err != nil {
				return err
			}
			groups, err := client.GetAPIGroups(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list API groups: %w", err)
			}
			return renderAPIGroups(o.ioStreams.Out, output, groups)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func renderAPIGroups(writer io.Writer, output string, groups *registry.APIGroupList) error {
	return renderWorkspaceResource(writer, output, groups, func(writer io.Writer) error {
		if groups == nil || len(groups.Groups) == 0 {
			_, _ = fmt.Fprintln(writer, "No extension groups advertised by this deployment.")
			return nil
		}
		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{"Name", "Preferred Version"})
		for _, group := range groups.Groups {
			table.Append([]string{group.Name, group.PreferredVersion.GroupVersion})
		}
		table.Render()
		return nil
	})
}

// NewCmdAPIResources creates the extension resource discovery command.
func NewCmdAPIResources(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	output := "text"
	group := registry.CloudExtensionGroup
	cmd := &cobra.Command{
		Use:   "api-resources",
		Short: "List resources exposed by an extension API group",
		Long: "List resources for an advertised extension API group. The default is cloud.sn.io; " +
			"use --group for policy.runorca.ai or pricing.runorca.ai.",
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			configureWorkspaceWarningWriter(cmd, args)
			group = strings.TrimSpace(group)
			if group == "" {
				return fmt.Errorf("--group cannot be empty")
			}
			return requireExtensionGroupOperation(cmd, group)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			resources, err := client.Get(cmd.Context(), extensionAPIResourcesPath(group))
			if err != nil {
				return fmt.Errorf("failed to list API resources for %q: %w", group, err)
			}
			return renderDiscoveredAPIResources(opts.IOStreams.Out, output, resources)
		},
	}
	cmd.Flags().StringVar(&group, "group", group, "Advertised extension API group")
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func extensionAPIResourcesPath(group string) string {
	path := "/apis/" + url.PathEscape(group) + "/v1"
	if group == registry.CloudExtensionGroup {
		return path + "/"
	}
	return path
}

func renderDiscoveredAPIResources(writer io.Writer, output string, value interface{}) error {
	if output != "text" {
		return renderJSONOrText(writer, output, value)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to encode API resources: %w", err)
	}
	var resources registry.APIResourceList
	if err := json.Unmarshal(raw, &resources); err != nil {
		return fmt.Errorf("failed to decode API resources: %w", err)
	}
	return renderAPIResources(writer, output, &resources)
}

func renderAPIResources(writer io.Writer, output string, resources *registry.APIResourceList) error {
	return renderWorkspaceResource(writer, output, resources, func(writer io.Writer) error {
		if resources == nil || len(resources.Resources) == 0 {
			_, _ = fmt.Fprintln(writer, "No API resources advertised by this extension group.")
			return nil
		}
		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{"Name", "Kind", "Namespaced"})
		for _, resource := range resources.Resources {
			table.Append([]string{resource.Name, resource.Kind, fmt.Sprintf("%t", resource.Namespaced)})
		}
		table.Render()
		return nil
	})
}

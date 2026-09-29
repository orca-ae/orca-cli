// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	policyExtensionGroup  = "policy.runorca.ai"
	pricingExtensionGroup = "pricing.runorca.ai"

	guardrailsPath     = "/apis/" + policyExtensionGroup + "/v1/guardrails"
	guardrailTypesPath = "/apis/" + policyExtensionGroup + "/v1/guardrailtypes"
	modelPricesPath    = "/apis/" + pricingExtensionGroup + "/v1/modelprices"
)

type extensionCommandOptions struct {
	ioStreams IOStreams
}

type extensionListOptions struct {
	output          string
	limit           int
	page            string
	includeArchived bool
}

type extensionPayloadOptions struct {
	output     string
	configJSON string
	configFile string
}

// NewCmdGuardrails creates commands for the policy.runorca.ai extension.
func NewCmdGuardrails(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &extensionCommandOptions{ioStreams: opts.IOStreams}

	cmd := &cobra.Command{
		Use:     "guardrails",
		Aliases: []string{"guardrail"},
		Short:   "Manage policy guardrails",
		Long: "Manage policy guardrails through the policy.runorca.ai extension API. " +
			"The command checks GET /apis before calling an extension endpoint.",
		PersistentPreRunE: requireExtensionGroup(policyExtensionGroup),
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newGuardrailCreateCommand(),
		o.newGuardrailListCommand(),
		o.newGuardrailGetCommand(),
		o.newGuardrailUpdateCommand(),
		o.newGuardrailArchiveCommand(),
		o.newGuardrailDeleteCommand(),
		o.newGuardrailListTypesCommand(),
	)
	return cmd
}

// NewCmdModelPrices creates commands for the pricing.runorca.ai extension.
func NewCmdModelPrices(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &extensionCommandOptions{ioStreams: opts.IOStreams}

	cmd := &cobra.Command{
		Use:     "model-prices",
		Aliases: []string{"model-price", "modelprices", "modelprice"},
		Short:   "Inspect effective model prices",
		Long: "Inspect effective model prices through the pricing.runorca.ai extension API. " +
			"The command checks GET /apis before calling an extension endpoint.",
		PersistentPreRunE: requireExtensionGroup(pricingExtensionGroup),
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newModelPriceListCommand(),
		o.newModelPriceGetCommand(),
	)
	return cmd
}

func (o *extensionCommandOptions) newGuardrailCreateCommand() *cobra.Command {
	opts := extensionPayloadOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "create (--config-json <json> | --file <path>)",
		Short: "Create a policy guardrail",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildExtensionPayload(opts)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.Create(cmd.Context(), guardrailsPath, payload)
			if err != nil {
				return fmt.Errorf("failed to create guardrail: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addExtensionPayloadFlags(cmd, &opts)
	return cmd
}

func (o *extensionCommandOptions) newGuardrailListCommand() *cobra.Command {
	opts := extensionListOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List policy guardrails",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			path, err := buildExtensionListPath(guardrailsPath, opts)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list guardrails: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addExtensionListFlags(cmd, &opts, true)
	return cmd
}

func (o *extensionCommandOptions) newGuardrailGetCommand() *cobra.Command {
	return o.newExtensionGetCommand(
		"get [guardrail-id]",
		"Get a policy guardrail",
		"guardrail",
		guardrailsPath,
		nil,
	)
}

func (o *extensionCommandOptions) newGuardrailUpdateCommand() *cobra.Command {
	opts := extensionPayloadOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "update [guardrail-id] (--config-json <json> | --file <path>)",
		Short: "Update a policy guardrail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			path, err := extensionItemPath(guardrailsPath, args[0], "guardrail-id")
			if err != nil {
				return err
			}
			payload, err := buildExtensionPayload(opts)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.Update(cmd.Context(), http.MethodPost, path, payload)
			if err != nil {
				return fmt.Errorf("failed to update guardrail %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addExtensionPayloadFlags(cmd, &opts)
	return cmd
}

func (o *extensionCommandOptions) newGuardrailArchiveCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "archive [guardrail-id]",
		Short: "Archive a policy guardrail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			path, err := extensionItemPath(guardrailsPath, args[0], "guardrail-id")
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.Archive(cmd.Context(), path+"/archive")
			if err != nil {
				return fmt.Errorf("failed to archive guardrail %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *extensionCommandOptions) newGuardrailDeleteCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "delete [guardrail-id]",
		Short: "Permanently delete an unreferenced policy guardrail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			path, err := extensionItemPath(guardrailsPath, args[0], "guardrail-id")
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.Delete(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to delete guardrail %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *extensionCommandOptions) newGuardrailListTypesCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "list-types",
		Short: "List builtin guardrail types and parameter schemas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), guardrailTypesPath)
			if err != nil {
				return fmt.Errorf("failed to list guardrail types: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *extensionCommandOptions) newModelPriceListCommand() *cobra.Command {
	opts := extensionListOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List effective model prices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			path, err := buildExtensionListPath(modelPricesPath, opts)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list model prices: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addExtensionListFlags(cmd, &opts, false)
	return cmd
}

func (o *extensionCommandOptions) newModelPriceGetCommand() *cobra.Command {
	provider := ""
	cmd := o.newExtensionGetCommand(
		"get [model-id]",
		"Get the effective price for a model",
		"model price",
		modelPricesPath,
		func(values url.Values) {
			if value := strings.TrimSpace(provider); value != "" {
				values.Set("provider", value)
			}
		},
	)
	cmd.Flags().StringVar(&provider, "provider", "", "Model provider used to disambiguate the model id")
	return cmd
}

func (o *extensionCommandOptions) newExtensionGetCommand(
	use string,
	short string,
	resourceName string,
	basePath string,
	addQuery func(url.Values),
) *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			path, err := extensionItemPath(basePath, args[0], resourceName+" id")
			if err != nil {
				return err
			}
			values := url.Values{}
			if addQuery != nil {
				addQuery(values)
			}
			if len(values) > 0 {
				path += "?" + values.Encode()
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get %s %q: %w", resourceName, args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func addExtensionPayloadFlags(cmd *cobra.Command, opts *extensionPayloadOptions) {
	cmd.Flags().StringVar(&opts.configJSON, "config-json", "", "Full guardrail request JSON object")
	cmd.Flags().StringVarP(&opts.configFile, "file", "f", "", "Path to a JSON file containing the full guardrail request")
	cmd.MarkFlagsMutuallyExclusive("config-json", "file")
	cmd.MarkFlagsOneRequired("config-json", "file")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func addExtensionListFlags(cmd *cobra.Command, opts *extensionListOptions, includeArchived bool) {
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	if includeArchived {
		cmd.Flags().BoolVar(&opts.includeArchived, "include-archived", false, "Include archived guardrails")
	}
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func buildExtensionPayload(opts extensionPayloadOptions) (map[string]interface{}, error) {
	if strings.TrimSpace(opts.configJSON) != "" && strings.TrimSpace(opts.configFile) != "" {
		return nil, fmt.Errorf("--config-json and --file cannot be used together")
	}
	if strings.TrimSpace(opts.configJSON) != "" {
		return parseJSONObject(opts.configJSON, "config-json")
	}
	if strings.TrimSpace(opts.configFile) != "" {
		data, err := os.ReadFile(opts.configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read --file %q: %w", opts.configFile, err)
		}
		return parseJSONObject(string(data), "file")
	}
	return nil, fmt.Errorf("one of --config-json or --file is required")
}

func buildExtensionListPath(basePath string, opts extensionListOptions) (string, error) {
	if opts.limit < 0 {
		return "", fmt.Errorf("--limit cannot be negative")
	}
	values := url.Values{}
	if opts.limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", opts.limit))
	}
	if page := strings.TrimSpace(opts.page); page != "" {
		values.Set("page", page)
	}
	if opts.includeArchived {
		values.Set("include_archived", "true")
	}
	if len(values) == 0 {
		return basePath, nil
	}
	return basePath + "?" + values.Encode(), nil
}

func extensionItemPath(basePath string, id string, fieldName string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%s cannot be empty", fieldName)
	}
	return basePath + "/" + url.PathEscape(id), nil
}

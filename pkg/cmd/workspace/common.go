// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/spf13/cobra"
)

var workspaceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

var selectConnectionNameForApply = func(ctx context.Context) (string, error) {
	return currentOptions.resolveConnectionSelection(ctx, "", true)
}

type workspaceConnectionsClient interface {
	List(ctx context.Context) ([]registry.ConnectionConfig, error)
	Get(ctx context.Context, name string) (*registry.ConnectionConfig, error)
	Create(ctx context.Context, cfg registry.ConnectionConfig) error
	Validate(ctx context.Context, cfg registry.ConnectionConfig) error
	Update(ctx context.Context, name string, cfg registry.ConnectionConfig) error
	Delete(ctx context.Context, name string) error
	Test(ctx context.Context, name string) (*registry.ConnectionHealthStatus, error)
}

type workspaceManagedAgentsClient interface {
	Get(ctx context.Context, path string) (interface{}, error)
	Create(ctx context.Context, path string, payload interface{}) (interface{}, error)
	Update(ctx context.Context, method string, path string, payload interface{}) (interface{}, error)
	Delete(ctx context.Context, path string) (interface{}, error)
	Archive(ctx context.Context, path string) (interface{}, error)
	DoMultipart(ctx context.Context, method string, path string, payload registry.MultipartRequest) (interface{}, error)
	GetToWriter(ctx context.Context, path string, writer io.Writer) error
	GetStream(ctx context.Context, path string, accept string, handle func(io.Reader) error) error
}

type workspaceCatalogClient interface {
	ListSources(ctx context.Context) ([]registry.ConnectorDefinition, error)
	GetSourceConfigDefinition(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error)
	ListSinks(ctx context.Context) ([]registry.ConnectorDefinition, error)
	GetSinkConfigDefinition(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error)
	ListKafkaConnectors(ctx context.Context) ([]registry.ConnectorDefinition, error)
	GetKafkaConfigDefinition(ctx context.Context, name string) ([]registry.ConfigFieldDefinition, error)
}

type workspaceProvidersClient interface {
	List(ctx context.Context) ([]registry.AgentProviderInfo, error)
	Get(ctx context.Context, name string) (*registry.AgentProviderInfo, error)
}

func newWorkspaceRegistryClient() (*registry.Client, error) {
	return newWorkspaceRegistryClientWithHTTPClient(nil)
}

func newWorkspaceRegistryClientWithHTTPClient(httpClient *http.Client) (*registry.Client, error) {
	return currentOptions.newRegistryClient(context.Background(), nil, httpClient)
}

var newWorkspaceConnectionsClient = func() (workspaceConnectionsClient, error) {
	return currentOptions.NewConnectionsClient(context.Background(), nil)
}

var newWorkspaceManagedAgentsClient = func() (workspaceManagedAgentsClient, error) {
	client, err := newWorkspaceRegistryClient()
	if err != nil {
		return nil, err
	}

	return registry.NewManagedAgentsClient(client), nil
}

var newWorkspaceCatalogClient = func() (workspaceCatalogClient, error) {
	client, err := newWorkspaceRegistryClient()
	if err != nil {
		return nil, err
	}

	return registry.NewCatalogClient(client), nil
}

var newWorkspaceProvidersClient = func() (workspaceProvidersClient, error) {
	client, err := newWorkspaceRegistryClient()
	if err != nil {
		return nil, err
	}

	return registry.NewProvidersClient(client), nil
}

var newWorkspaceManagedAgentsStreamClient = func() (workspaceManagedAgentsClient, error) {
	client, err := newWorkspaceRegistryClientWithHTTPClient(&http.Client{})
	if err != nil {
		return nil, err
	}
	// The SDK also applies a per-request deadline independently of http.Client.
	// Streaming lifetime is controlled by the caller's context and --timeout.
	client, err = client.With(option.WithRequestTimeout(0))
	if err != nil {
		return nil, err
	}

	return registry.NewManagedAgentsClient(client), nil
}

var newWorkspaceDiscoveryClient = func() (*registry.Client, error) {
	return newWorkspaceRegistryClient()
}

// requireCloudExtension is a cobra PersistentPreRunE attached to every workspace command that only
// the hosted distribution serves, through the hosted extension group: health, connections,
// sources, sinks, functions, kafka-connect, packages, and (in agent.go) agent providers. Before a
// leaf command's own request, it probes GET /apis once per Options runtime, so a deployment that
// does not support the extension reports a clear "not available" message instead of a bare 404
// from deep inside whatever the command was about to do. Executing one of the parent commands directly still shows help without requiring a
// configured runtime.
//
// Attaching it to the parent command of each of those trees (rather than every leaf) means core
// commands never pay for the extra round trip, and cobra only invokes the nearest
// PersistentPreRunE it finds walking up from the command actually being run - so this has no
// effect on sibling core commands.
func requireCloudExtension(cmd *cobra.Command, _ []string) error {
	if cmd.HasSubCommands() {
		return nil
	}
	return requireCloudExtensionOperation(cmd, nil)
}

func requireCloudExtensionOperation(cmd *cobra.Command, _ []string) error {
	return requireExtensionGroupOperation(cmd, registry.CloudExtensionGroup)
}

func requireExtensionGroup(group string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if cmd.HasSubCommands() {
			return nil
		}
		return requireExtensionGroupOperation(cmd, group)
	}
}

func requireExtensionGroupOperation(cmd *cobra.Command, group string) error {
	warnOut := currentOptions.commandErrorWriter(cmd)
	groups, err := currentOptions.discoverAPIGroups(cmd.Context(), cmd)
	return ensureExtensionGroupAvailable(groups, err, group, warnOut)
}

func configureWorkspaceWarningWriter(cmd *cobra.Command, _ []string) {
	currentOptions.commandErrorWriter(cmd)
}

// ensureExtensionAvailable calls GET /apis on client and reports whether group is advertised.
// Diagnostic warnings go to warnOut (the caller's stderr), never stdout.
//
//   - 200 with group present: nil - the caller's real request proceeds.
//   - 200 without group present (including an empty groups list - a normal self-hosted engine
//     with no extensions installed): a "not available" error.
//   - 404: a deployment that predates extension discovery entirely. Also reported as "not
//     available", but additionally logged as a version warning on warnOut, since a 404 here is a
//     stronger signal of an outdated deployment than an empty list is - the two are different
//     diagnoses even though both mean the caller cannot use the extension today.
//   - Any other error (network failure, malformed response, ...): returned as-is, not folded into
//     "not available". The probe could not determine capability, so guessing - either blocking
//     with a claim the deployment does not support this when it might, or silently proceeding past
//     a real connectivity problem - would be worse than surfacing what actually happened. The
//     command's real request would hit the same failure anyway.
func ensureExtensionAvailable(ctx context.Context, client *registry.Client, group string, warnOut io.Writer) error {
	groups, err := client.GetAPIGroups(ctx)
	return ensureExtensionGroupAvailable(groups, err, group, warnOut)
}

func ensureExtensionGroupAvailable(groups *registry.APIGroupList, err error, group string, warnOut io.Writer) error {
	if err != nil {
		var httpErr *registry.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			fmt.Fprintf(warnOut,
				"warning: GET /apis returned 404 — this deployment predates extension discovery "+
					"and cannot serve the %q extension group. Confirm the server version if this "+
					"is unexpected.\n", group)
			return fmt.Errorf("the %q extension group is not available on this deployment", group)
		}
		return fmt.Errorf("failed to probe deployment capabilities: %w", err)
	}
	if !groups.HasGroup(group) {
		return fmt.Errorf("the %q extension group is not available on this deployment", group)
	}
	return nil
}

func validateWorkspaceOutput(output string) error {
	switch output {
	case "text", "json", "yaml":
		return nil
	default:
		return fmt.Errorf("--output must be one of: text, json, yaml")
	}
}

func validateWorkspaceName(resourceType, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s name cannot be empty", resourceType)
	}
	if !workspaceNamePattern.MatchString(name) {
		return fmt.Errorf("%s name %q is invalid: must contain only alphanumeric characters, hyphens, and underscores, and start with a letter or number", resourceType, name)
	}
	return nil
}

func lifecycleActionLabel(action string) string {
	switch action {
	case "start":
		return "Start"
	case "stop":
		return "Stop"
	case "restart":
		return "Restart"
	default:
		return action
	}
}

func renderWorkspaceResource(writer io.Writer, output string, value any, renderText func(io.Writer) error) error {
	if err := validateWorkspaceOutput(output); err != nil {
		return err
	}

	switch output {
	case "json":
		return renderJSON(writer, value)
	case "yaml":
		return renderYAML(writer, value)
	default:
		return renderText(writer)
	}
}

func renderNameList(writer io.Writer, output string, header string, items []string) error {
	if err := validateWorkspaceOutput(output); err != nil {
		return err
	}

	switch output {
	case "json":
		return renderJSON(writer, items)
	case "yaml":
		return renderYAML(writer, items)
	default:
		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{header})
		for _, item := range items {
			table.Append([]string{item})
		}
		table.Render()
		return nil
	}
}

func renderConnectorDefinitions(writer io.Writer, output string, connectorKind string, items []registry.ConnectorDefinition) error {
	return renderWorkspaceResource(writer, output, items, func(writer io.Writer) error {
		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{"Name", "Description", "Class", "Config Class"})
		for _, item := range items {
			className := item.SourceClass
			configClassName := item.SourceConfigClass
			if connectorKind == "sink" {
				className = item.SinkClass
				configClassName = item.SinkConfigClass
			} else if connectorKind == "kafka" && className == "" {
				className = item.SinkClass
				configClassName = item.SinkConfigClass
			}
			table.Append([]string{item.Name, item.Description, className, configClassName})
		}
		table.Render()
		return nil
	})
}

func renderConfigFieldDefinitions(writer io.Writer, output string, items []registry.ConfigFieldDefinition) error {
	return renderWorkspaceResource(writer, output, items, func(writer io.Writer) error {
		table := tablewriter.NewWriter(writer)
		table.SetHeader([]string{"Field Name", "Type", "Attributes"})
		for _, item := range items {
			table.Append([]string{item.FieldName, item.TypeName, formatConfigFieldAttributes(item.Attributes)})
		}
		table.Render()
		return nil
	})
}

func renderAgentProviders(writer io.Writer, output string, items []registry.AgentProviderInfo) error {
	return renderWorkspaceResource(writer, output, items, func(writer io.Writer) error {
		renderAgentProviderTable(writer, items)
		return nil
	})
}

func renderAgentProvider(writer io.Writer, output string, item *registry.AgentProviderInfo) error {
	return renderWorkspaceResource(writer, output, item, func(writer io.Writer) error {
		if item == nil {
			return nil
		}
		renderAgentProviderTable(writer, []registry.AgentProviderInfo{*item})
		return nil
	})
}

func renderAgentProviderTable(writer io.Writer, items []registry.AgentProviderInfo) {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Name", "Type", "API Version", "Beta Version", "API Key Env", "API Key Configured"})
	for _, item := range items {
		table.Append([]string{
			item.Name,
			item.Type,
			item.APIVersion,
			item.BetaVersion,
			item.APIKeyEnv,
			strconv.FormatBool(item.APIKeyConfigured),
		})
	}
	table.Render()
}

func formatConfigFieldAttributes(attributes map[string]string) string {
	if len(attributes) == 0 {
		return ""
	}

	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, attributes[key]))
	}
	return strings.Join(parts, ", ")
}

func resolveConnectionSelection(ctx context.Context, explicit string, useInteractive bool) (string, error) {
	if explicit != "" && useInteractive {
		return "", fmt.Errorf("--connection and --use-connection cannot be used together")
	}
	if explicit != "" {
		return explicit, nil
	}
	if useInteractive {
		return selectConnectionNameForApply(ctx)
	}
	return "", nil
}

// Pulsar places a function, source or sink in the public tenant's default
// namespace unless its config names others.
const (
	defaultPulsarTenant    = "public"
	defaultPulsarNamespace = "default"
)

// packageURLPrefixes mark a package location that Pulsar fetches itself rather
// than one the CLI uploads: http(s) and file URLs, and the function, sink and
// source package schemes. Matching is a plain prefix test, as in Pulsar's admin
// tools.
var packageURLPrefixes = []string{"http", "file", "function", "sink", "source"}

func splitPackageLocation(location string) (filePath string, packageURL string) {
	trimmed := strings.TrimSpace(location)
	if trimmed == "" {
		return "", ""
	}
	if strings.HasPrefix(trimmed, "builtin://") || isPackageURL(trimmed) {
		return "", trimmed
	}
	return trimmed, ""
}

func isPackageURL(location string) bool {
	for _, prefix := range packageURLPrefixes {
		if strings.HasPrefix(location, prefix) {
			return true
		}
	}
	return false
}

// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/orca-ae/orca-sdk-go/packages/param"
	"github.com/spf13/cobra"
)

// corePathPrefix is prepended to every core managed-agent resource path (agents, sessions, memory
// stores, files, skills, vaults, environments, triggers): the surface shared by the managed and
// open-source deployments, served at the deployment host root under /v1.
const corePathPrefix = "/v1"

type agentOptions struct {
	opts      *Options
	ioStreams IOStreams
}

type agentListOptions struct {
	output          string
	limit           int
	page            string
	afterID         string
	beforeID        string
	agentID         string
	includeArchived bool
	order           string
	createdAtGT     string
	createdAtGTE    string
	createdAtLT     string
	createdAtLTE    string
	eventTypes      []string
	subpath         string
}

type agentPayloadOptions struct {
	output string

	metadata []string

	version          int
	model            string
	modelJSON        string
	name             string
	description      string
	system           string
	mcpServers       []string
	tools            []string
	skills           []string
	multiagentType   string
	multiagentAgents []string

	displayName string
	scope       string

	configType       string
	networkingType   string
	configProperties []string
	configJSON       string
	packageApt       []string
	packageCargo     []string
	packageGem       []string
	packageGo        []string
	packageNPM       []string
	packagePip       []string

	environmentID string
	vaultIDs      []string
	title         string
	resources     []string
	agentJSON     string
	initialEvents []string
}

type agentSessionChildOptions struct {
	output string

	agentID   string
	sessionID string
	threadID  string
	limit     int
	page      string
	order     string
	afterID   string
	beforeID  string

	createdAtGT  string
	createdAtGTE string
	createdAtLT  string
	createdAtLTE string
	eventTypes   []string
	subpath      string
	fromCursor   string
	eventDeltas  []string

	resourceType       string
	fileID             string
	memoryStoreID      string
	access             string
	instructions       string
	url                string
	authorizationToken string
	checkoutType       string
	branch             string
	commit             string
	mountPath          string
	mountStrategy      string
	eventJSONs         []string
	eventText          string
	outcomeDescription string
	outcomeRubric      string
	maxIterations      int
	toolUseID          string
	decision           string
	denyMessage        string
	timeout            string
	outputPath         string
}

type agentMemoryEntryOptions struct {
	output          string
	memoryStoreID   string
	content         string
	contentJSON     string
	path            string
	preconditionSHA string
	view            string
	expectedSHA     string
	limit           int
	page            string
	depth           string
	pathPrefix      string
}

type agentMemoryVersionOptions struct {
	output        string
	memoryStoreID string
	memoryID      string
	apiKeyID      string
	operation     string
	createdAtGTE  string
	createdAtLTE  string
	view          string
	limit         int
	page          string
}

type agentFileContentOptions struct {
	outputPath string
}

type agentTriggerOptions struct {
	output string

	configJSON string
	configFile string

	name                    string
	agentID                 string
	agentVersion            int
	sessionMode             string
	sourceType              string
	connection              string
	topics                  []string
	topicPattern            string
	subscriptionName        string
	typeClassName           string
	typeClassDefinition     string
	typeClassDefinitionFile string
	schemaType              string
	consumerConfigs         []string
	inputSchemaConfigs      []string
	schedule                string
	timezone                string
	payload                 string
	environmentID           string
	titleTemplate           string
	sessionMetadata         []string
	vaultIDs                []string
	replicas                int
	paused                  bool
}

type agentMultipartOptions struct {
	output       string
	files        []string
	displayTitle string
	contentType  string
}

type vaultCredentialOptions struct {
	output          string
	vaultID         string
	limit           int
	page            string
	includeArchived bool
	displayName     string
	authJSON        string
	metadata        []string
}

type managedAgentResource struct {
	use          string
	aliases      []string
	singular     string
	plural       string
	idName       string
	basePath     string
	updateMethod string
	archive      bool
	multipart    bool
	multiFile    bool
	skill        bool
	file         bool
	sessions     bool
}

// NewCmdAgent creates workspace managed agent commands.
func NewCmdAgent(opts *Options) *cobra.Command {
	opts = setCurrentOptions(opts)
	o := &agentOptions{
		opts:      opts,
		ioStreams: opts.IOStreams,
	}

	cmd := &cobra.Command{
		Use:     "agent",
		Aliases: []string{"agents"},
		Short:   "Manage workspace managed agents",
		Long: "Manage managed agents and related objects through the workspace registry API. " +
			"Use subcommands such as sessions, vaults, environments, files, and skills for related objects.",
		PersistentPreRun: configureWorkspaceWarningWriter,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}

	agentResource := managedAgentResource{
		use:          "agent",
		singular:     "agent",
		plural:       "agents",
		idName:       "agent-id",
		basePath:     "/agents",
		updateMethod: http.MethodPost,
		archive:      true,
	}
	cmd.AddCommand(
		o.newManagedAgentListCommand(agentResource),
		o.newManagedAgentGetCommand(agentResource),
		o.newManagedAgentCreateCommand(agentResource),
		o.newManagedAgentUpdateCommand(agentResource),
		o.newManagedAgentArchiveCommand(agentResource),
		o.newAgentVersionsCommand(),
		o.newManagedAgentResourceGroup(managedAgentResource{
			use:          "sessions",
			singular:     "session",
			plural:       "sessions",
			idName:       "session-id",
			updateMethod: http.MethodPost,
			archive:      true,
			sessions:     true,
		}),
		o.newManagedAgentResourceGroup(managedAgentResource{
			use:          "memory-stores",
			aliases:      []string{"memory_stores", "memory-store", "memory_store"},
			singular:     "memory store",
			plural:       "memory stores",
			idName:       "memory-store-id",
			basePath:     "/memory_stores",
			updateMethod: http.MethodPost,
			archive:      true,
		}),
		o.newManagedAgentResourceGroup(managedAgentResource{
			use:          "vaults",
			singular:     "vault",
			plural:       "vaults",
			idName:       "vault-id",
			basePath:     "/vaults",
			updateMethod: http.MethodPost,
			archive:      true,
		}),
		o.newManagedAgentResourceGroup(managedAgentResource{
			use:          "environments",
			singular:     "environment",
			plural:       "environments",
			idName:       "environment-id",
			basePath:     "/environments",
			updateMethod: http.MethodPost,
			archive:      true,
		}),
		o.newManagedAgentResourceGroup(managedAgentResource{
			use:       "files",
			singular:  "file",
			plural:    "files",
			idName:    "file-id",
			basePath:  "/files",
			multipart: true,
			file:      true,
		}),
		o.newManagedAgentResourceGroup(managedAgentResource{
			use:       "skills",
			singular:  "skill",
			plural:    "skills",
			idName:    "skill-id",
			basePath:  "/skills",
			multipart: true,
			multiFile: true,
			skill:     true,
		}),
		o.newMemoryVersionsCommand(),
		o.newAgentProvidersCommand(),
		o.newAgentTriggersCommand(),
	)

	return cmd
}

func (o *agentOptions) newAgentProvidersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "providers",
		Aliases: []string{"provider"},
		Short:   "Manage workspace managed agent providers",
		Long: "Manage workspace managed agent providers. " +
			"Requires the cloud.sn.io extension group; not available on a self-hosted engine.",
		PersistentPreRunE: requireCloudExtension,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newAgentProviderListCommand(),
		o.newAgentProviderGetCommand(),
	)
	return cmd
}

func (o *agentOptions) newAgentProviderListCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace managed agent providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceProvidersClient()
			if err != nil {
				return err
			}
			items, err := client.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list agent providers: %w", err)
			}
			return renderAgentProviders(o.ioStreams.Out, output, items)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newAgentProviderGetCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "get [provider-name]",
		Short: "Get a workspace managed agent provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceProvidersClient()
			if err != nil {
				return err
			}
			item, err := client.Get(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("failed to get agent provider %q: %w", args[0], err)
			}
			return renderAgentProvider(o.ioStreams.Out, output, item)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newAgentTriggersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "triggers",
		Aliases: []string{"trigger"},
		Short:   "Manage Pulsar, Kafka, and cron agent triggers",
		Long: "Manage workspace managed agent triggers for Pulsar, Kafka, and cron sources. " +
			"Support for source types and session modes depends on the deployment.",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newAgentTriggerListCommand(),
		o.newAgentTriggerGetCommand(),
		o.newAgentTriggerCreateCommand(),
		o.newAgentTriggerUpdateCommand(),
		o.newAgentTriggerDeleteCommand(),
		o.newAgentTriggerActionCommand("pause"),
		o.newAgentTriggerActionCommand("unpause"),
		o.newAgentTriggerSessionsCommand(),
	)
	return cmd
}

func (o *agentOptions) newManagedAgentResourceGroup(resource managedAgentResource) *cobra.Command {
	cmd := &cobra.Command{
		Use:     resource.use,
		Aliases: resource.aliases,
		Short:   fmt.Sprintf("Manage workspace %s", managedAgentPluralLabel(resource)),
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(
		o.newManagedAgentListCommand(resource),
		o.newManagedAgentGetCommand(resource),
		o.newManagedAgentCreateCommand(resource),
		o.newManagedAgentDeleteCommand(resource),
	)
	if !resource.multipart {
		cmd.AddCommand(o.newManagedAgentUpdateCommand(resource))
	}
	if resource.archive {
		cmd.AddCommand(o.newManagedAgentArchiveCommand(resource))
	}
	if resource.skill {
		cmd.AddCommand(o.newSkillVersionsCommand())
	}
	if resource.use == "vaults" {
		cmd.AddCommand(o.newVaultCredentialsCommand())
	}
	if resource.use == "memory-stores" {
		cmd.AddCommand(o.newMemoryStoreMemoriesCommand())
	}
	if resource.use == "files" {
		cmd.AddCommand(o.newFileContentCommand())
	}
	if resource.sessions {
		cmd.AddCommand(
			o.newSessionOutcomeCommand(),
			o.newSessionResourcesCommand(),
			o.newSessionFilesCommand(),
			o.newSessionEventsCommand(),
			o.newSessionThreadsCommand(),
		)
	}

	return cmd
}

func (o *agentOptions) newVaultCredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credentials",
		Short: "Manage workspace managed agent vault credentials",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newVaultCredentialListCommand(),
		o.newVaultCredentialGetCommand(),
		o.newVaultCredentialCreateCommand(),
		o.newVaultCredentialUpdateCommand(),
		o.newVaultCredentialDeleteCommand(),
		o.newVaultCredentialArchiveCommand(),
		o.newVaultCredentialValidateCommand(),
	)
	return cmd
}

func (o *agentOptions) newManagedAgentListCommand(resource managedAgentResource) *cobra.Command {
	opts := agentListOptions{output: "text"}

	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List workspace %s", managedAgentPluralLabel(resource)),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceTypedClient()
			if err != nil {
				return err
			}
			var raw []byte
			if err := managedAgentOpsFor(resource).list(cmd.Context(), client, opts, &raw); err != nil {
				return fmt.Errorf("failed to list %s: %w", managedAgentPluralLabel(resource), err)
			}
			return renderRaw(o.ioStreams.Out, opts.output, raw)
		},
	}

	addManagedAgentListFlags(cmd, &opts, resource)
	return cmd
}

func (o *agentOptions) newAgentTriggerListCommand() *cobra.Command {
	opts := agentListOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace managed agent triggers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildAgentTriggerCollectionPath(cmd, opts)
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list agent triggers: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().StringVar(&opts.agentID, "agent", "", "Managed agent id")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newAgentTriggerGetCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "get [trigger-id]",
		Short: "Get a workspace managed agent trigger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildAgentTriggerItemPath(args[0])
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get agent trigger %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newAgentTriggerCreateCommand() *cobra.Command {
	opts := newAgentTriggerOptions()
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a workspace managed agent trigger",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildAgentTriggerPayload(cmd, opts, false)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildAgentTriggerCollectionPath(cmd, agentListOptions{})
			if err != nil {
				return err
			}
			result, err := client.Create(cmd.Context(), path, payload)
			if err != nil {
				return fmt.Errorf("failed to create agent trigger: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addAgentTriggerPayloadFlags(cmd, &opts, false)
	return cmd
}

func (o *agentOptions) newAgentTriggerUpdateCommand() *cobra.Command {
	opts := newAgentTriggerOptions()
	cmd := &cobra.Command{
		Use:   "update [trigger-id]",
		Short: "Update a workspace managed agent trigger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildAgentTriggerPayload(cmd, opts, true)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildAgentTriggerItemPath(args[0])
			if err != nil {
				return err
			}
			result, err := client.Update(cmd.Context(), http.MethodPost, path, payload)
			if err != nil {
				return fmt.Errorf("failed to update agent trigger %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addAgentTriggerPayloadFlags(cmd, &opts, true)
	return cmd
}

func (o *agentOptions) newAgentTriggerDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [trigger-id]",
		Short: "Delete a workspace managed agent trigger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildAgentTriggerItemPath(args[0])
			if err != nil {
				return err
			}
			result, err := client.Delete(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to delete agent trigger %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	return cmd
}

func (o *agentOptions) newAgentTriggerActionCommand(action string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   action + " [trigger-id]",
		Short: strings.ToUpper(action[:1]) + action[1:] + " a workspace managed agent trigger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildAgentTriggerActionPath(args[0], action)
			if err != nil {
				return err
			}
			result, err := client.Update(cmd.Context(), http.MethodPost, path, nil)
			if err != nil {
				return fmt.Errorf("failed to %s agent trigger %q: %w", action, args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	return cmd
}

func (o *agentOptions) newAgentTriggerSessionsCommand() *cobra.Command {
	opts := agentListOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "sessions [trigger-id]",
		Short: "List sessions created by a workspace managed agent trigger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildAgentTriggerSessionsPath(args[0], opts)
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list sessions for agent trigger %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().BoolVar(&opts.includeArchived, "include-archived", false, "Include archived sessions")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newVaultCredentialListCommand() *cobra.Command {
	opts := vaultCredentialOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list --vault <vault-id>",
		Short: "List workspace managed agent vault credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildVaultCredentialCollectionPath(opts)
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list vault credentials: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addVaultCredentialParentFlag(cmd, &opts)
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().BoolVar(&opts.includeArchived, "include-archived", false, "Include archived credentials")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newVaultCredentialGetCommand() *cobra.Command {
	opts := vaultCredentialOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "get [credential-id] --vault <vault-id>",
		Short: "Get a workspace managed agent vault credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildVaultCredentialItemPath(opts, args[0])
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get vault credential %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addVaultCredentialParentFlag(cmd, &opts)
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newVaultCredentialUpdateCommand() *cobra.Command {
	opts := vaultCredentialOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "update [credential-id] --vault <vault-id>",
		Short: "Update a workspace managed agent vault credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildVaultCredentialPayload(cmd, opts, true)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildVaultCredentialItemPath(opts, args[0])
			if err != nil {
				return err
			}
			result, err := client.Update(cmd.Context(), http.MethodPost, path, payload)
			if err != nil {
				return fmt.Errorf("failed to update vault credential %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addVaultCredentialParentFlag(cmd, &opts)
	addVaultCredentialPayloadFlags(cmd, &opts, true)
	return cmd
}

func (o *agentOptions) newVaultCredentialDeleteCommand() *cobra.Command {
	opts := vaultCredentialOptions{}
	cmd := &cobra.Command{
		Use:   "delete [credential-id] --vault <vault-id>",
		Short: "Delete a workspace managed agent vault credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildVaultCredentialItemPath(opts, args[0])
			if err != nil {
				return err
			}
			result, err := client.Delete(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to delete vault credential %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	addVaultCredentialParentFlag(cmd, &opts)
	return cmd
}

func (o *agentOptions) newVaultCredentialArchiveCommand() *cobra.Command {
	opts := vaultCredentialOptions{}
	cmd := &cobra.Command{
		Use:   "archive [credential-id] --vault <vault-id>",
		Short: "Archive a workspace managed agent vault credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildVaultCredentialActionPath(opts, args[0], "archive")
			if err != nil {
				return err
			}
			result, err := client.Archive(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to archive vault credential %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	addVaultCredentialParentFlag(cmd, &opts)
	return cmd
}

func (o *agentOptions) newVaultCredentialValidateCommand() *cobra.Command {
	opts := vaultCredentialOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "validate [credential-id] --vault <vault-id>",
		Short: "Validate a workspace managed agent vault credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildVaultCredentialActionPath(opts, args[0], "mcp_oauth_validate")
			if err != nil {
				return err
			}
			result, err := client.Archive(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to validate vault credential %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addVaultCredentialParentFlag(cmd, &opts)
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newMemoryStoreMemoriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memories",
		Aliases: []string{"memory"},
		Short:   "Manage workspace managed agent memory entries",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newMemoryEntryListCommand(),
		o.newMemoryEntryGetCommand(),
		o.newMemoryEntryCreateCommand(),
		o.newMemoryEntryUpdateCommand(),
		o.newMemoryEntryDeleteCommand(),
	)
	return cmd
}

func (o *agentOptions) newMemoryEntryListCommand() *cobra.Command {
	opts := agentMemoryEntryOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list --memory-store <memory-store-id>",
		Short: "List workspace managed agent memory entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildMemoryEntryCollectionPath(opts)
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list memory entries: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addMemoryEntryParentFlag(cmd, &opts)
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().StringVar(&opts.depth, "depth", "", "Memory hierarchy depth: 0 or 1")
	cmd.Flags().StringVar(&opts.pathPrefix, "path-prefix", "", "Filter memories by path prefix")
	cmd.Flags().StringVar(&opts.view, "view", "", "Memory view query value: basic or full")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newMemoryEntryGetCommand() *cobra.Command {
	opts := agentMemoryEntryOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "get [memory-id] --memory-store <memory-store-id>",
		Short: "Get a workspace managed agent memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildMemoryEntryItemPath(opts, args[0])
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get memory entry %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addMemoryEntryParentFlag(cmd, &opts)
	cmd.Flags().StringVar(&opts.view, "view", "", "Memory view query value")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newMemoryEntryCreateCommand() *cobra.Command {
	opts := agentMemoryEntryOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "create --memory-store <memory-store-id>",
		Short: "Create a workspace managed agent memory entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildMemoryEntryPayload(opts, false)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildMemoryEntryCollectionPath(opts)
			if err != nil {
				return err
			}
			result, err := client.Create(cmd.Context(), path, payload)
			if err != nil {
				return fmt.Errorf("failed to create memory entry: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addMemoryEntryParentFlag(cmd, &opts)
	cmd.Flags().StringVar(&opts.view, "view", "", "Memory view query value")
	addMemoryEntryPayloadFlags(cmd, &opts, false)
	return cmd
}

func (o *agentOptions) newMemoryEntryUpdateCommand() *cobra.Command {
	opts := agentMemoryEntryOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "update [memory-id] --memory-store <memory-store-id>",
		Short: "Update a workspace managed agent memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildMemoryEntryPayload(opts, true)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildMemoryEntryItemPath(opts, args[0])
			if err != nil {
				return err
			}
			result, err := client.Update(cmd.Context(), http.MethodPost, path, payload)
			if err != nil {
				return fmt.Errorf("failed to update memory entry %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addMemoryEntryParentFlag(cmd, &opts)
	cmd.Flags().StringVar(&opts.view, "view", "", "Memory view query value")
	addMemoryEntryPayloadFlags(cmd, &opts, true)
	return cmd
}

func (o *agentOptions) newMemoryEntryDeleteCommand() *cobra.Command {
	opts := agentMemoryEntryOptions{}
	cmd := &cobra.Command{
		Use:   "delete [memory-id] --memory-store <memory-store-id>",
		Short: "Delete a workspace managed agent memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildMemoryEntryDeletePath(opts, args[0])
			if err != nil {
				return err
			}
			result, err := client.Delete(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to delete memory entry %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	addMemoryEntryParentFlag(cmd, &opts)
	cmd.Flags().StringVar(&opts.expectedSHA, "expected-content-sha256", "", "Expected memory content SHA-256 precondition")
	return cmd
}

func (o *agentOptions) newMemoryVersionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memory-versions",
		Aliases: []string{"memory_versions", "memory-version", "memory_version"},
		Short:   "Manage workspace managed agent memory versions",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newMemoryVersionListCommand(),
		o.newMemoryVersionGetCommand(),
		o.newMemoryVersionRedactCommand(),
	)
	return cmd
}

func (o *agentOptions) newMemoryVersionListCommand() *cobra.Command {
	opts := agentMemoryVersionOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list --memory-store <memory-store-id>",
		Short: "List workspace managed agent memory versions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildMemoryVersionCollectionPath(opts)
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list memory versions: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addMemoryVersionParentFlag(cmd, &opts)
	cmd.Flags().StringVar(&opts.memoryID, "memory-id", "", "Filter versions by memory id")
	cmd.Flags().StringVar(&opts.apiKeyID, "api-key-id", "", "Filter versions by API key id")
	cmd.Flags().StringVar(&opts.operation, "operation", "", "Filter by operation: created, modified, or deleted")
	cmd.Flags().StringVar(&opts.createdAtGTE, "created-at-gte", "", "Filter versions created at or after this timestamp")
	cmd.Flags().StringVar(&opts.createdAtLTE, "created-at-lte", "", "Filter versions created at or before this timestamp")
	cmd.Flags().StringVar(&opts.view, "view", "", "Memory view query value: basic or full")
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newMemoryVersionGetCommand() *cobra.Command {
	opts := agentMemoryVersionOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "get [version-id] --memory-store <memory-store-id>",
		Short: "Get a workspace managed agent memory version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildMemoryVersionItemPath(opts, args[0])
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get memory version %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addMemoryVersionParentFlag(cmd, &opts)
	cmd.Flags().StringVar(&opts.view, "view", "", "Memory view query value: basic or full")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newMemoryVersionRedactCommand() *cobra.Command {
	opts := agentMemoryVersionOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "redact [version-id] --memory-store <memory-store-id>",
		Short: "Redact a workspace managed agent memory version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildMemoryVersionRedactPath(opts, args[0])
			if err != nil {
				return err
			}
			result, err := client.Archive(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to redact memory version %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addMemoryVersionParentFlag(cmd, &opts)
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newFileContentCommand() *cobra.Command {
	opts := agentFileContentOptions{}
	cmd := &cobra.Command{
		Use:     "content [file-id]",
		Aliases: []string{"download"},
		Short:   "Download workspace managed agent file content",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsStreamClient()
			if err != nil {
				return err
			}
			path, err := buildFileContentPath(args[0])
			if err != nil {
				return err
			}
			if strings.TrimSpace(opts.outputPath) == "" {
				if err := client.GetToWriter(cmd.Context(), path, o.ioStreams.Out); err != nil {
					return fmt.Errorf("failed to download file content %q: %w", args[0], err)
				}
				return nil
			}
			return downloadManagedAgentContent(cmd.Context(), client, path, opts.outputPath)
		},
	}
	cmd.Flags().StringVar(&opts.outputPath, "output-file", "", "Write content to a file instead of stdout")
	return cmd
}

func (o *agentOptions) newSessionFilesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files",
		Short: "Manage workspace managed agent session files",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newSessionFileListCommand(),
		o.newSessionFileGetCommand(),
		o.newSessionFileContentCommand(),
		o.newSessionFileDeleteCommand(),
	)
	return cmd
}

func (o *agentOptions) newSessionFileListCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace managed agent session files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildCollectionPath(opts, "files", agentListOptions{
				limit:    opts.limit,
				afterID:  opts.afterID,
				beforeID: opts.beforeID,
			})
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list session files: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.afterID, "after-id", "", "Return files after this file id")
	cmd.Flags().StringVar(&opts.beforeID, "before-id", "", "Return files before this file id")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSessionFileGetCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "get [file-id]",
		Short: "Get workspace managed agent session file metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildItemPath(opts, "files", args[0])
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get session file %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSessionFileContentCommand() *cobra.Command {
	opts := agentSessionChildOptions{}
	cmd := &cobra.Command{
		Use:     "content [file-id]",
		Aliases: []string{"download"},
		Short:   "Download workspace managed agent session file content",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsStreamClient()
			if err != nil {
				return err
			}
			path, err := buildSessionFileContentPath(opts, args[0])
			if err != nil {
				return err
			}
			if strings.TrimSpace(opts.outputPath) == "" {
				if err := client.GetToWriter(cmd.Context(), path, o.ioStreams.Out); err != nil {
					return fmt.Errorf("failed to download session file content %q: %w", args[0], err)
				}
				return nil
			}
			return downloadManagedAgentContent(cmd.Context(), client, path, opts.outputPath)
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.outputPath, "output-file", "", "Write content to a file instead of stdout")
	return cmd
}

func (o *agentOptions) newSessionFileDeleteCommand() *cobra.Command {
	opts := agentSessionChildOptions{}
	cmd := &cobra.Command{
		Use:   "delete [file-id]",
		Short: "Delete a workspace managed agent session file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildItemPath(opts, "files", args[0])
			if err != nil {
				return err
			}
			result, err := client.Delete(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to delete session file %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	return cmd
}

func (o *agentOptions) newManagedAgentGetCommand(resource managedAgentResource) *cobra.Command {
	output := "text"
	version := 0

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("get [%s]", resource.idName),
		Short: fmt.Sprintf("Get a workspace %s", managedAgentSingularLabel(resource)),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			if resource.use == "agent" && cmd.Flags().Changed("version") && version <= 0 {
				return fmt.Errorf("--version must be greater than 0")
			}
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("%s cannot be empty", resource.idName)
			}
			client, err := newWorkspaceTypedClient()
			if err != nil {
				return err
			}
			var requested param.Opt[int64]
			if resource.use == "agent" && cmd.Flags().Changed("version") {
				requested = param.Int(int64(version))
			}
			var raw []byte
			err = managedAgentOpsFor(resource).get(cmd.Context(), client, args[0], requested, &raw)
			if err != nil {
				return fmt.Errorf("failed to get %s %q: %w", managedAgentSingularLabel(resource), args[0], err)
			}
			return renderRaw(o.ioStreams.Out, output, raw)
		},
	}

	if resource.use == "agent" {
		cmd.Flags().IntVar(&version, "version", 0, "Agent version to get")
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newManagedAgentCreateCommand(resource managedAgentResource) *cobra.Command {
	if resource.multipart {
		return o.newManagedAgentMultipartCreateCommand(resource)
	}

	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{
		Use:   managedAgentCreateUse(resource),
		Short: fmt.Sprintf("Create a workspace %s", managedAgentSingularLabel(resource)),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := buildManagedAgentPayload(cmd, resource, opts, false)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildManagedAgentCollectionPath(cmd, resource, agentListOptions{})
			if err != nil {
				return err
			}
			result, err := client.Create(cmd.Context(), path, payload)
			if err != nil {
				return fmt.Errorf("failed to create %s: %w", managedAgentSingularLabel(resource), err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}

	addManagedAgentPayloadFlags(cmd, &opts, resource, false)
	addSessionAgentFlag(cmd, resource)
	return cmd
}

func (o *agentOptions) newManagedAgentUpdateCommand(resource managedAgentResource) *cobra.Command {
	opts := newManagedAgentPayloadOptions(resource)
	cmd := &cobra.Command{
		Use:   managedAgentUpdateUse(resource),
		Short: fmt.Sprintf("Update a workspace %s", managedAgentSingularLabel(resource)),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := buildManagedAgentPayload(cmd, resource, opts, true)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildManagedAgentItemPath(cmd, resource, args[0])
			if err != nil {
				return err
			}
			result, err := client.Update(cmd.Context(), resource.updateMethod, path, payload)
			if err != nil {
				return fmt.Errorf("failed to update %s %q: %w", managedAgentSingularLabel(resource), args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	if resource.use == "agent" {
		cmd.Long = "Update a workspace managed agent. Use --version for optional optimistic concurrency."
		cmd.Example = "  ork workspace agent update agent-1 --name updated-agent\n" +
			"  ork workspace agent update agent-1 --version 2 --name updated-agent"
	}

	addManagedAgentPayloadFlags(cmd, &opts, resource, true)
	addSessionAgentFlag(cmd, resource, true)
	return cmd
}

func (o *agentOptions) newManagedAgentDeleteCommand(resource managedAgentResource) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("delete [%s]", resource.idName),
		Short: fmt.Sprintf("Delete a workspace %s", managedAgentSingularLabel(resource)),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("%s cannot be empty", resource.idName)
			}
			client, err := newWorkspaceTypedClient()
			if err != nil {
				return err
			}
			var raw []byte
			if err := managedAgentOpsFor(resource).remove(cmd.Context(), client, args[0], &raw); err != nil {
				return fmt.Errorf("failed to delete %s %q: %w", managedAgentSingularLabel(resource), args[0], err)
			}
			return renderRaw(o.ioStreams.Out, "text", raw)
		},
	}

	return cmd
}

func (o *agentOptions) newManagedAgentArchiveCommand(resource managedAgentResource) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("archive [%s]", resource.idName),
		Short: fmt.Sprintf("Archive a workspace %s", managedAgentSingularLabel(resource)),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("%s cannot be empty", resource.idName)
			}
			client, err := newWorkspaceTypedClient()
			if err != nil {
				return err
			}
			var raw []byte
			if err := managedAgentOpsFor(resource).archive(cmd.Context(), client, args[0], &raw); err != nil {
				return fmt.Errorf("failed to archive %s %q: %w", managedAgentSingularLabel(resource), args[0], err)
			}
			return renderRaw(o.ioStreams.Out, "text", raw)
		},
	}

	return cmd
}

func (o *agentOptions) newManagedAgentMultipartCreateCommand(resource managedAgentResource) *cobra.Command {
	opts := agentMultipartOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "create --file <path>",
		Short: fmt.Sprintf("Create a workspace %s", managedAgentSingularLabel(resource)),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			request, err := buildManagedAgentMultipartRequest(resource, opts)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			result, err := client.DoMultipart(cmd.Context(), http.MethodPost, corePathPrefix+resource.basePath, request)
			if err != nil {
				return fmt.Errorf("failed to create %s: %w", managedAgentSingularLabel(resource), err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}

	cmd.Flags().StringArrayVarP(&opts.files, "file", "f", nil,
		"File to upload. For skills this flag can be repeated and each upload uses the local base name.")
	cmd.Flags().StringVar(&opts.displayTitle, "display-title", "", "Skill display title")
	cmd.Flags().StringVar(&opts.contentType, "content-type", "", "Uploaded file content type")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newAgentVersionsCommand() *cobra.Command {
	output := "text"
	opts := agentListOptions{}

	cmd := &cobra.Command{
		Use:   "versions [agent-id]",
		Short: "List workspace managed agent versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			query, err := buildManagedAgentQuery(cmd, opts)
			if err != nil {
				return err
			}
			path := corePathPrefix + "/agents/" + url.PathEscape(args[0]) + "/versions" + query
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list agent versions for %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}

	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSkillVersionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Manage workspace managed agent skill versions",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newSkillVersionListCommand(),
		o.newSkillVersionGetCommand(),
		o.newSkillVersionContentCommand(),
		o.newSkillVersionCreateCommand(),
		o.newSkillVersionDeleteCommand(),
	)
	return cmd
}

func (o *agentOptions) newSkillVersionListCommand() *cobra.Command {
	output := "text"
	opts := agentListOptions{}
	cmd := &cobra.Command{
		Use:   "list [skill-id]",
		Short: "List workspace managed agent skill versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			query, err := buildManagedAgentQuery(cmd, opts)
			if err != nil {
				return err
			}
			path := corePathPrefix + "/skills/" + url.PathEscape(args[0]) + "/versions" + query
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list skill versions for %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSkillVersionGetCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "get [skill-id] [version-number]",
		Short: "Get a workspace managed agent skill version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			version, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err != nil || version <= 0 {
				return fmt.Errorf("skill versions get expects a positive integer version number, got %q", args[1])
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path := corePathPrefix + "/skills/" + url.PathEscape(args[0]) + "/versions/" + strconv.Itoa(version)
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get skill version %q for %q: %w", args[1], args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSkillVersionContentCommand() *cobra.Command {
	opts := agentFileContentOptions{}
	cmd := &cobra.Command{
		Use:     "content [skill-id] [version]",
		Aliases: []string{"download"},
		Short:   "Download a workspace managed agent skill version bundle",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsStreamClient()
			if err != nil {
				return err
			}
			path, err := buildSkillVersionContentPath(args[0], args[1])
			if err != nil {
				return err
			}
			if strings.TrimSpace(opts.outputPath) == "" {
				if err := client.GetToWriter(cmd.Context(), path, o.ioStreams.Out); err != nil {
					return fmt.Errorf("failed to download skill version %q for %q: %w", args[1], args[0], err)
				}
				return nil
			}
			return downloadManagedAgentContent(cmd.Context(), client, path, opts.outputPath)
		},
	}
	cmd.Flags().StringVar(&opts.outputPath, "output-file", "", "Write the zip bundle to a file instead of stdout")
	return cmd
}

func (o *agentOptions) newSkillVersionCreateCommand() *cobra.Command {
	opts := agentMultipartOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "create [skill-id] --file <path>",
		Short: "Create a workspace managed agent skill version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := buildManagedAgentMultipartRequest(managedAgentResource{multiFile: true, skill: true}, opts)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path := corePathPrefix + "/skills/" + url.PathEscape(args[0]) + "/versions"
			result, err := client.DoMultipart(cmd.Context(), http.MethodPost, path, request)
			if err != nil {
				return fmt.Errorf("failed to create skill version for %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	cmd.Flags().StringArrayVarP(&opts.files, "file", "f", nil,
		"Skill file to upload. This flag can be repeated and each upload uses the local base name.")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSkillVersionDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [skill-id] [version]",
		Short: "Delete a workspace managed agent skill version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path := corePathPrefix + "/skills/" + url.PathEscape(args[0]) + "/versions/" + url.PathEscape(args[1])
			result, err := client.Delete(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to delete skill version %q for %q: %w", args[1], args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	return cmd
}

func (o *agentOptions) newSessionResourcesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Manage workspace managed agent session resources",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newSessionResourceListCommand(),
		o.newSessionResourceGetCommand(),
		o.newSessionResourceAddCommand(),
		o.newSessionResourceUpdateCommand(),
		o.newSessionResourceDeleteCommand(),
	)
	return cmd
}

func (o *agentOptions) newSessionOutcomeCommand() *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "outcome [session-id]",
		Short: "Get the evaluated outcome for a workspace managed agent session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionOutcomePath(args[0])
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get outcome for session %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, output, result)
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSessionResourceListCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace managed agent session resources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildCollectionPath(opts, "resources", agentListOptions{
				limit: opts.limit,
				page:  opts.page,
			})
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list session resources: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionChildListFlags(cmd, &opts)
	return cmd
}

func (o *agentOptions) newSessionResourceGetCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "get [resource-id]",
		Short: "Get a workspace managed agent session resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildItemPath(opts, "resources", args[0])
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get session resource %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSessionResourceAddCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a workspace managed agent session resource",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildSessionResourcePayload(opts, false)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildCollectionPath(opts, "resources", agentListOptions{})
			if err != nil {
				return err
			}
			result, err := client.Create(cmd.Context(), path, payload)
			if err != nil {
				return fmt.Errorf("failed to add session resource: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionResourcePayloadFlags(cmd, &opts, false)
	return cmd
}

func (o *agentOptions) newSessionResourceUpdateCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "update [resource-id]",
		Short: "Update a workspace managed agent session resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildSessionResourcePayload(opts, true)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildItemPath(opts, "resources", args[0])
			if err != nil {
				return err
			}
			result, err := client.Update(cmd.Context(), http.MethodPost, path, payload)
			if err != nil {
				return fmt.Errorf("failed to update session resource %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionResourcePayloadFlags(cmd, &opts, true)
	return cmd
}

func (o *agentOptions) newSessionResourceDeleteCommand() *cobra.Command {
	opts := agentSessionChildOptions{}
	cmd := &cobra.Command{
		Use:   "delete [resource-id]",
		Short: "Delete a workspace managed agent session resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildItemPath(opts, "resources", args[0])
			if err != nil {
				return err
			}
			result, err := client.Delete(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to delete session resource %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	return cmd
}

func (o *agentOptions) newSessionEventsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Manage workspace managed agent session events",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newSessionEventListCommand(),
		o.newSessionEventSendCommand(),
		o.newSessionEventStreamCommand(),
	)
	return cmd
}

func (o *agentOptions) newSessionEventListCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace managed agent session events",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildCollectionPath(opts, "events", agentListOptions{
				limit:        opts.limit,
				page:         opts.page,
				order:        opts.order,
				createdAtGT:  opts.createdAtGT,
				createdAtGTE: opts.createdAtGTE,
				createdAtLT:  opts.createdAtLT,
				createdAtLTE: opts.createdAtLTE,
				eventTypes:   opts.eventTypes,
				subpath:      opts.subpath,
			})
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list session events: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionChildListFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.order, "order", "", "Sort order")
	cmd.Flags().StringVar(&opts.createdAtGT, "created-at-gt", "", "Return events created after this timestamp")
	cmd.Flags().StringVar(&opts.createdAtGTE, "created-at-gte", "", "Return events created at or after this timestamp")
	cmd.Flags().StringVar(&opts.createdAtLT, "created-at-lt", "", "Return events created before this timestamp")
	cmd.Flags().StringVar(&opts.createdAtLTE, "created-at-lte", "", "Return events created at or before this timestamp")
	cmd.Flags().StringArrayVar(&opts.eventTypes, "event-type", nil, "Filter by event type. Can be repeated.")
	cmd.Flags().StringVar(&opts.subpath, "subpath", "", "Filter events by subpath")
	return cmd
}

func (o *agentOptions) newSessionEventSendCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send typed events to a workspace managed agent session",
		Long: "Send typed events to a workspace managed agent session. " +
			"Use message, outcome, or tool-confirmation subcommands for typed events. " +
			"The --event-json flag remains for raw compatibility.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			payload, err := buildSessionEventsPayload(opts)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildCollectionPath(opts, "events", agentListOptions{})
			if err != nil {
				return err
			}
			result, err := client.Create(cmd.Context(), path, payload)
			if err != nil {
				return fmt.Errorf("failed to send session events: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	cmd.PersistentFlags().StringVar(&opts.sessionID, "session", "", "Parent managed agent session id")
	_ = cmd.MarkPersistentFlagRequired("session")
	cmd.Flags().StringArrayVar(&opts.eventJSONs, "event-json", nil,
		"Raw session event JSON object. Can be repeated.")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	cmd.AddCommand(
		o.newSessionEventSendMessageCommand(&opts),
		o.newSessionEventSendOutcomeCommand(&opts),
		o.newSessionEventSendToolConfirmationCommand(&opts),
	)
	return cmd
}

func (o *agentOptions) newSessionEventSendMessageCommand(opts *agentSessionChildOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Send a user.message event to a workspace managed agent session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := buildSessionMessageEventPayload(opts.eventText)
			if err != nil {
				return err
			}
			return o.sendSessionEvents(cmd, *opts, payload)
		},
	}
	cmd.Flags().StringVar(&opts.eventText, "text", "", "Text content for user.message")
	_ = cmd.MarkFlagRequired("text")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSessionEventSendOutcomeCommand(opts *agentSessionChildOptions) *cobra.Command {
	opts.maxIterations = 3
	cmd := &cobra.Command{
		Use:   "outcome",
		Short: "Send a user.define_outcome event to a workspace managed agent session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := buildSessionOutcomeEventPayload(
				opts.outcomeDescription,
				opts.outcomeRubric,
				opts.maxIterations,
			)
			if err != nil {
				return err
			}
			return o.sendSessionEvents(cmd, *opts, payload)
		},
	}
	cmd.Flags().StringVar(&opts.outcomeDescription, "description", "", "Outcome description")
	cmd.Flags().StringVar(&opts.outcomeRubric, "rubric", "", "Outcome evaluation rubric")
	cmd.Flags().IntVar(&opts.maxIterations, "max-iterations", opts.maxIterations, "Maximum outcome iterations, 1-20")
	_ = cmd.MarkFlagRequired("description")
	_ = cmd.MarkFlagRequired("rubric")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSessionEventSendToolConfirmationCommand(opts *agentSessionChildOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-confirmation",
		Short: "Send a user.tool_confirmation event to a workspace managed agent session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := buildSessionToolConfirmationEventPayload(opts.toolUseID, opts.decision, opts.denyMessage)
			if err != nil {
				return err
			}
			return o.sendSessionEvents(cmd, *opts, payload)
		},
	}
	cmd.Flags().StringVar(&opts.toolUseID, "tool-use-id", "", "Tool use id to confirm")
	cmd.Flags().StringVar(&opts.decision, "decision", "", "Tool decision/result: allow or deny")
	cmd.Flags().StringVar(&opts.denyMessage, "deny-message", "", "Optional denial message")
	_ = cmd.MarkFlagRequired("tool-use-id")
	_ = cmd.MarkFlagRequired("decision")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) sendSessionEvents(
	cmd *cobra.Command,
	opts agentSessionChildOptions,
	payload map[string]interface{},
) error {
	if err := validateWorkspaceOutput(opts.output); err != nil {
		return err
	}
	client, err := newWorkspaceManagedAgentsClient()
	if err != nil {
		return err
	}
	path, err := buildSessionChildCollectionPath(opts, "events", agentListOptions{})
	if err != nil {
		return err
	}
	result, err := client.Create(cmd.Context(), path, payload)
	if err != nil {
		return fmt.Errorf("failed to send session events: %w", err)
	}
	return renderJSONOrText(o.ioStreams.Out, opts.output, result)
}

func (o *agentOptions) newSessionEventStreamCommand() *cobra.Command {
	opts := agentSessionChildOptions{}
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream workspace managed agent session events",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			streamCtx, cancel, hasTimeout, err := buildSessionEventStreamContext(cmd.Context(), opts.timeout)
			if err != nil {
				return err
			}
			defer cancel()

			client, err := newWorkspaceManagedAgentsStreamClient()
			if err != nil {
				return err
			}
			path, err := buildSessionEventStreamPath(opts)
			if err != nil {
				return err
			}
			if err := streamManagedAgentSSE(client, streamCtx, path, o.ioStreams.Out); err != nil {
				if hasTimeout && errors.Is(streamCtx.Err(), context.DeadlineExceeded) && errors.Is(err, context.DeadlineExceeded) {
					return nil
				}
				return fmt.Errorf("failed to stream session events: %w", err)
			}
			return nil
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.fromCursor, "from-cursor", "", "Resume streaming at this SSE frame cursor (inclusive)")
	cmd.Flags().StringVar(&opts.subpath, "subpath", "", "Stream events from this subpath")
	cmd.Flags().StringArrayVar(&opts.eventDeltas, "event-delta", nil,
		"Stream event delta type: agent.message or agent.thinking. Can be repeated.")
	cmd.Flags().StringVar(&opts.timeout, "timeout", "",
		"Maximum time to keep the stream open, for example 30s or 2m. Defaults to no CLI timeout.")
	return cmd
}

func (o *agentOptions) newSessionThreadsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "threads",
		Short: "Manage workspace managed agent session threads",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newSessionThreadListCommand(),
		o.newSessionThreadGetCommand(),
		o.newSessionThreadArchiveCommand(),
		o.newSessionThreadEventsCommand(),
	)
	return cmd
}

func (o *agentOptions) newSessionThreadListCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace managed agent session threads",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildCollectionPath(opts, "threads", agentListOptions{
				limit: opts.limit,
				page:  opts.page,
			})
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list session threads: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionChildListFlags(cmd, &opts)
	return cmd
}

func (o *agentOptions) newSessionThreadGetCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "get [thread-id]",
		Short: "Get a workspace managed agent session thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildItemPath(opts, "threads", args[0])
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to get session thread %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
	return cmd
}

func (o *agentOptions) newSessionThreadArchiveCommand() *cobra.Command {
	opts := agentSessionChildOptions{}
	cmd := &cobra.Command{
		Use:   "archive [thread-id]",
		Short: "Archive a workspace managed agent session thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionChildItemPath(opts, "threads", args[0])
			if err != nil {
				return err
			}
			result, err := client.Archive(cmd.Context(), path+"/archive")
			if err != nil {
				return fmt.Errorf("failed to archive session thread %q: %w", args[0], err)
			}
			return renderJSONOrText(o.ioStreams.Out, "text", result)
		},
	}
	addSessionChildRequiredFlags(cmd, &opts)
	return cmd
}

func (o *agentOptions) newSessionThreadEventsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Manage workspace managed agent session thread events",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(
		o.newSessionThreadEventListCommand(),
		o.newSessionThreadEventStreamCommand(),
	)
	return cmd
}

func (o *agentOptions) newSessionThreadEventListCommand() *cobra.Command {
	opts := agentSessionChildOptions{output: "text"}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspace managed agent session thread events",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			path, err := buildSessionThreadEventCollectionPath(opts, agentListOptions{
				limit: opts.limit,
				page:  opts.page,
			})
			if err != nil {
				return err
			}
			result, err := client.Get(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("failed to list session thread events: %w", err)
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addSessionThreadEventListFlags(cmd, &opts)
	return cmd
}

func (o *agentOptions) newSessionThreadEventStreamCommand() *cobra.Command {
	opts := agentSessionChildOptions{}
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream workspace managed agent session thread events",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			streamCtx, cancel, hasTimeout, err := buildSessionEventStreamContext(cmd.Context(), opts.timeout)
			if err != nil {
				return err
			}
			defer cancel()

			client, err := newWorkspaceManagedAgentsStreamClient()
			if err != nil {
				return err
			}
			path, err := buildSessionThreadEventStreamPath(opts)
			if err != nil {
				return err
			}
			if err := streamManagedAgentSSE(client, streamCtx, path, o.ioStreams.Out); err != nil {
				if hasTimeout && errors.Is(streamCtx.Err(), context.DeadlineExceeded) && errors.Is(err, context.DeadlineExceeded) {
					return nil
				}
				return fmt.Errorf("failed to stream session thread events: %w", err)
			}
			return nil
		},
	}
	addSessionThreadEventRequiredFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.fromCursor, "from-cursor", "", "Resume streaming at this SSE frame cursor (inclusive)")
	cmd.Flags().StringArrayVar(&opts.eventDeltas, "event-delta", nil,
		"Stream event delta type: agent.message or agent.thinking. Can be repeated.")
	cmd.Flags().StringVar(&opts.timeout, "timeout", "",
		"Maximum time to keep the stream open, for example 30s or 2m. Defaults to no CLI timeout.")
	return cmd
}

func addManagedAgentListFlags(cmd *cobra.Command, opts *agentListOptions, resource managedAgentResource) {
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	if resource.file {
		cmd.Flags().StringVar(&opts.afterID, "after-id", "", "Return files after this file id")
		cmd.Flags().StringVar(&opts.beforeID, "before-id", "", "Return files before this file id")
	} else {
		cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	}
	if managedAgentListSupportsIncludeArchived(resource) {
		cmd.Flags().BoolVar(&opts.includeArchived, "include-archived", false, "Include archived objects")
	}
	addSessionListAgentFlag(cmd, opts, resource)
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func managedAgentListSupportsIncludeArchived(resource managedAgentResource) bool {
	switch resource.use {
	case "agent", "sessions", "memory-stores", "vaults", "environments":
		return true
	default:
		return false
	}
}

func newManagedAgentPayloadOptions(resource managedAgentResource) agentPayloadOptions {
	opts := agentPayloadOptions{output: "text"}
	if resource.use == "environments" {
		opts.configType = "cloud"
		opts.networkingType = "unrestricted"
	}
	return opts
}

func addManagedAgentPayloadFlags(
	cmd *cobra.Command,
	opts *agentPayloadOptions,
	resource managedAgentResource,
	update bool,
) {
	cmd.Flags().StringArrayVar(&opts.metadata, "metadata", nil, "Metadata in key=value form. Can be repeated.")

	switch {
	case resource.use == "agent":
		addAgentPayloadFlags(cmd, opts, update)
	case resource.use == "environments":
		addEnvironmentPayloadFlags(cmd, opts)
	case resource.use == "memory-stores":
		addMemoryStorePayloadFlags(cmd, opts)
	case resource.use == "vaults":
		cmd.Flags().StringVar(&opts.displayName, "display-name", "", "Vault display name")
	case resource.sessions:
		addSessionPayloadFlags(cmd, opts, update)
	}
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func addSessionAgentFlag(cmd *cobra.Command, resource managedAgentResource, update ...bool) {
	if resource.sessions {
		cmd.Flags().String("agent", "", "Parent managed agent id")
		cmd.Flags().Int("agent-version", 0, "Managed agent version to pin on this session")
		cmd.MarkFlagsMutuallyExclusive("agent", "agent-json")
	}
}

func addSessionListAgentFlag(cmd *cobra.Command, opts *agentListOptions, resource managedAgentResource) {
	if resource.sessions {
		cmd.Flags().StringVar(&opts.agentID, "agent", "", "Parent managed agent id")
	}
}

func addVaultCredentialParentFlag(cmd *cobra.Command, opts *vaultCredentialOptions) {
	cmd.Flags().StringVar(&opts.vaultID, "vault", "", "Parent vault id")
	_ = cmd.MarkFlagRequired("vault")
}

func addVaultCredentialPayloadFlags(cmd *cobra.Command, opts *vaultCredentialOptions, update bool) {
	cmd.Flags().StringVar(&opts.displayName, "display-name", "", "Credential display name")
	cmd.Flags().StringVar(&opts.authJSON, "auth-json", "",
		"Credential auth JSON, for example {\"type\":\"static_bearer\",\"mcp_server_url\":\"https://example.com/mcp\",\"token\":\"token\"}")
	cmd.Flags().StringArrayVar(&opts.metadata, "metadata", nil, "Credential metadata in key=value form. Can be repeated.")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func newAgentTriggerOptions() agentTriggerOptions {
	return agentTriggerOptions{
		output:      "text",
		sessionMode: "SESSION_PER_TOPIC",
		sourceType:  "pulsar",
	}
}

func addAgentTriggerPayloadFlags(cmd *cobra.Command, opts *agentTriggerOptions, update bool) {
	cmd.Flags().StringVar(&opts.configJSON, "config-json", "", "Full agent trigger config JSON object")
	cmd.Flags().StringVarP(&opts.configFile, "file", "f", "", "Path to a JSON file containing the full agent trigger config")
	cmd.Flags().StringVar(&opts.name, "name", "", "Agent trigger display name")
	if !update {
		cmd.Flags().StringVar(&opts.agentID, "agent", "", "Managed agent id")
		cmd.Flags().IntVar(&opts.agentVersion, "agent-version", 0, "Managed agent version")
	}
	cmd.Flags().StringVar(&opts.sessionMode, "session-mode", opts.sessionMode,
		"Session mode: SESSION_PER_EVENT, SESSION_PER_TOPIC, SESSION_PER_KEY, or SHARED")
	if update {
		cmd.Flags().StringVar(&opts.sourceType, "source-type", "", "Trigger source type: pulsar, kafka, or cron")
	} else {
		cmd.Flags().StringVar(&opts.sourceType, "source-type", opts.sourceType, "Trigger source type: pulsar, kafka, or cron")
	}
	cmd.Flags().StringVar(&opts.connection, "connection", "", "Workspace connection name")
	cmd.Flags().StringArrayVar(&opts.topics, "topic", nil, "Source topic. Can be repeated.")
	cmd.Flags().StringVar(&opts.topicPattern, "topic-pattern", "", "Source topic pattern")
	cmd.Flags().StringVar(&opts.subscriptionName, "subscription-name", "", "Source subscription name")
	cmd.Flags().StringVar(&opts.typeClassName, "type-class-name", "", "Source schema type class name")
	cmd.Flags().StringVar(&opts.typeClassDefinition, "type-class-definition", "", "Source schema type class Python definition")
	cmd.Flags().StringVar(&opts.typeClassDefinitionFile, "type-class-definition-file", "",
		"Path to a Python file containing source schema type class definitions")
	cmd.Flags().StringVar(&opts.schemaType, "schema-type", "", "Source schema type, for example string, json, or avro")
	cmd.Flags().StringArrayVar(&opts.consumerConfigs, "consumer-config", nil,
		"Kafka consumer additional config in key=value form. Can be repeated.")
	cmd.Flags().StringArrayVar(&opts.inputSchemaConfigs, "input-schema-config", nil,
		"Kafka input schema config in topic=<json-object> form. Can be repeated.")
	cmd.Flags().StringVar(&opts.schedule, "schedule", "", "Cron schedule")
	cmd.Flags().StringVar(&opts.timezone, "timezone", "", "Cron schedule timezone")
	cmd.Flags().StringVar(&opts.payload, "payload", "", "Cron trigger payload")
	cmd.Flags().StringVar(&opts.environmentID, "environment-id", "", "Session environment id")
	cmd.Flags().StringVar(&opts.titleTemplate, "title-template", "", "Session title template")
	cmd.Flags().StringArrayVar(&opts.sessionMetadata, "metadata", nil, "Session metadata in key=value form. Can be repeated.")
	cmd.Flags().StringArrayVar(&opts.vaultIDs, "vault-id", nil, "Session vault id. Can be repeated. Sent as session.vault_ids.")
	cmd.Flags().IntVar(&opts.replicas, "replicas", 0, "Agent trigger replicas")
	if !update {
		cmd.Flags().BoolVar(&opts.paused, "paused", false, "Create the agent trigger as paused")
	}
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func addAgentPayloadFlags(cmd *cobra.Command, opts *agentPayloadOptions, update bool) {
	if update {
		cmd.Flags().IntVar(&opts.version, "version", 0, "Agent version for optimistic concurrency")
	}
	cmd.Flags().StringVar(&opts.model, "model", "", "Agent model name")
	cmd.Flags().StringVar(&opts.modelJSON, "model-json", "", "Agent model JSON value")
	if !update {
		cmd.MarkFlagsOneRequired("model", "model-json")
	}
	cmd.Flags().StringVar(&opts.name, "name", "", "Agent name")
	if !update {
		_ = cmd.MarkFlagRequired("name")
	}
	cmd.Flags().StringVar(&opts.description, "description", "", "Agent description")
	cmd.Flags().StringVar(&opts.system, "system", "", "Agent system prompt")
	cmd.Flags().StringArrayVar(&opts.mcpServers, "mcp-server", nil,
		"MCP server in key=value form: name=<name>,type=<type>,url=<url>. Can be repeated.")
	cmd.Flags().StringArrayVar(&opts.tools, "tool-json", nil, "Agent tool JSON object. Can be repeated.")
	cmd.Flags().StringArrayVar(&opts.skills, "skill", nil,
		"Agent skill reference: [anthropic:|custom:]<skill-id>[@<version>]. Custom skill_ IDs are inferred. Can be repeated.")
	cmd.Flags().StringVar(&opts.multiagentType, "multiagent-type", "", "Agent multiagent type")
	cmd.Flags().StringArrayVar(&opts.multiagentAgents, "multiagent-agent", nil,
		"Multiagent roster entry: <agent-id>, <agent-id>@<version>, or type=<type>,id=<agent-id>,version=<version>. Can be repeated.")
}

func addEnvironmentPayloadFlags(cmd *cobra.Command, opts *agentPayloadOptions) {
	cmd.Flags().StringVar(&opts.name, "name", "", "Environment name")
	cmd.Flags().StringVar(&opts.description, "description", "", "Environment description")
	cmd.Flags().StringVar(&opts.scope, "scope", "", "Environment scope: organization or account")
	cmd.Flags().StringVar(&opts.configType, "config-type", opts.configType, "Environment config type")
	cmd.Flags().StringVar(&opts.networkingType, "networking-type", opts.networkingType,
		"Environment networking type")
	cmd.Flags().StringArrayVar(&opts.configProperties, "config", nil,
		"Additional environment config in key=value form. Can be repeated.")
	cmd.Flags().StringVar(&opts.configJSON, "config-json", "", "Environment config JSON object")
	cmd.Flags().StringArrayVar(&opts.packageApt, "package-apt", nil,
		"Claude environment apt package. Can be repeated; sent as config.packages.apt, not workspace artifact packages.")
	cmd.Flags().StringArrayVar(&opts.packageCargo, "package-cargo", nil,
		"Claude environment cargo package. Can be repeated; sent as config.packages.cargo, not workspace artifact packages.")
	cmd.Flags().StringArrayVar(&opts.packageGem, "package-gem", nil,
		"Claude environment gem package. Can be repeated; sent as config.packages.gem, not workspace artifact packages.")
	cmd.Flags().StringArrayVar(&opts.packageGo, "package-go", nil,
		"Claude environment go package. Can be repeated; sent as config.packages.go, not workspace artifact packages.")
	cmd.Flags().StringArrayVar(&opts.packageNPM, "package-npm", nil,
		"Claude environment npm package. Can be repeated; sent as config.packages.npm, not workspace artifact packages.")
	cmd.Flags().StringArrayVar(&opts.packagePip, "package-pip", nil,
		"Claude environment pip package. Can be repeated; sent as config.packages.pip, not workspace artifact packages.")
}

func addMemoryStorePayloadFlags(cmd *cobra.Command, opts *agentPayloadOptions) {
	cmd.Flags().StringVar(&opts.name, "name", "", "Memory store name")
	cmd.Flags().StringVar(&opts.description, "description", "", "Memory store description")
}

func addSessionPayloadFlags(cmd *cobra.Command, opts *agentPayloadOptions, update bool) {
	if !update {
		cmd.Flags().StringVar(&opts.environmentID, "environment-id", "", "Session environment id (required)")
		_ = cmd.MarkFlagRequired("environment-id")
		cmd.Flags().StringArrayVar(&opts.resources, "resource-json", nil, "Session resource JSON object. Can be repeated.")
		cmd.Flags().StringArrayVar(&opts.initialEvents, "initial-event-json", nil,
			"Session initial event JSON object. Can be repeated.")
	}
	agentJSONDescription := "Session agent reference or agent-with-overrides JSON object"
	if update {
		agentJSONDescription = "Session agent override JSON object"
	}
	cmd.Flags().StringVar(&opts.agentJSON, "agent-json", "", agentJSONDescription)
	cmd.Flags().StringArrayVar(&opts.vaultIDs, "vault-id", nil, "Session vault id. Can be repeated. Sent as vault_ids.")
	cmd.Flags().StringVar(&opts.title, "title", "", "Session title")
}

func managedAgentCreateUse(resource managedAgentResource) string {
	if resource.sessions {
		return "create --environment-id <environment-id> [--agent <agent-id>] [--vault-id <vault-id>]"
	}
	return "create"
}

func managedAgentUpdateUse(resource managedAgentResource) string {
	if resource.sessions {
		return fmt.Sprintf("update [%s] [--agent <agent-id>] [--agent-version <version>] [--vault-id <vault-id>]", resource.idName)
	}
	return fmt.Sprintf("update [%s]", resource.idName)
}

func addSessionChildRequiredFlags(cmd *cobra.Command, opts *agentSessionChildOptions) {
	cmd.Flags().StringVar(&opts.sessionID, "session", "", "Parent managed agent session id")
	_ = cmd.MarkFlagRequired("session")
}

func addSessionChildListFlags(cmd *cobra.Command, opts *agentSessionChildOptions) {
	addSessionChildRequiredFlags(cmd, opts)
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func addSessionThreadEventRequiredFlags(cmd *cobra.Command, opts *agentSessionChildOptions) {
	addSessionChildRequiredFlags(cmd, opts)
	cmd.Flags().StringVar(&opts.threadID, "thread", "", "Parent managed agent session thread id")
	_ = cmd.MarkFlagRequired("thread")
}

func addSessionThreadEventListFlags(cmd *cobra.Command, opts *agentSessionChildOptions) {
	addSessionThreadEventRequiredFlags(cmd, opts)
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum number of results to return")
	cmd.Flags().StringVar(&opts.page, "page", "", "Pagination cursor")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func addSessionResourcePayloadFlags(cmd *cobra.Command, opts *agentSessionChildOptions, update bool) {
	addSessionChildRequiredFlags(cmd, opts)
	if update {
		cmd.Flags().StringVar(&opts.authorizationToken, "authorization-token", "", "GitHub repository authorization token")
		_ = cmd.MarkFlagRequired("authorization-token")
		cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
		return
	}
	cmd.Flags().StringVar(&opts.resourceType, "type", "", "Resource type: file, memory-store, or github-repository")
	cmd.Flags().StringVar(&opts.fileID, "file-id", "", "File resource id")
	cmd.Flags().StringVar(&opts.memoryStoreID, "memory-store-id", "", "Memory store resource id")
	cmd.Flags().StringVar(&opts.access, "access", "", "File or memory store access")
	cmd.Flags().StringVar(&opts.instructions, "instructions", "", "Memory store instructions")
	cmd.Flags().StringVar(&opts.url, "url", "", "GitHub repository URL")
	cmd.Flags().StringVar(&opts.authorizationToken, "authorization-token", "", "GitHub repository authorization token")
	cmd.Flags().StringVar(&opts.checkoutType, "checkout-type", "", "GitHub checkout type: branch or commit")
	cmd.Flags().StringVar(&opts.branch, "branch", "", "GitHub branch checkout name")
	cmd.Flags().StringVar(&opts.commit, "commit", "", "GitHub commit checkout SHA")
	cmd.Flags().StringVar(&opts.mountPath, "mount-path", "", "Resource mount path")
	cmd.Flags().StringVar(&opts.mountStrategy, "mount-strategy", "", "File mount strategy (tarball_prefetch)")
	_ = cmd.MarkFlagRequired("type")
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func buildManagedAgentCollectionPath(
	cmd *cobra.Command,
	resource managedAgentResource,
	opts agentListOptions,
) (string, error) {
	basePath, err := managedAgentBasePath(cmd, resource)
	if err != nil {
		return "", err
	}
	query, err := buildManagedAgentQuery(cmd, opts)
	if err != nil {
		return "", err
	}
	return basePath + query, nil
}

func buildManagedAgentItemPath(cmd *cobra.Command, resource managedAgentResource, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("%s cannot be empty", resource.idName)
	}
	basePath, err := managedAgentBasePath(cmd, resource)
	if err != nil {
		return "", err
	}
	return basePath + "/" + url.PathEscape(id), nil
}

func appendAgentVersionQuery(path string, version int) string {
	u, err := url.Parse(path)
	if err != nil {
		values := url.Values{}
		values.Set("version", strconv.Itoa(version))
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		return path + separator + values.Encode()
	}

	values := u.Query()
	values.Set("version", strconv.Itoa(version))
	u.RawQuery = values.Encode()
	return u.String()
}

func buildVaultCredentialCollectionPath(opts vaultCredentialOptions) (string, error) {
	basePath, err := vaultCredentialBasePath(opts)
	if err != nil {
		return "", err
	}
	listOpts := agentListOptions{
		limit:           opts.limit,
		page:            opts.page,
		includeArchived: opts.includeArchived,
	}
	query, err := buildManagedAgentQuery(nil, listOpts)
	if err != nil {
		return "", err
	}
	return basePath + query, nil
}

func buildVaultCredentialItemPath(opts vaultCredentialOptions, credentialID string) (string, error) {
	if strings.TrimSpace(credentialID) == "" {
		return "", fmt.Errorf("credential-id cannot be empty")
	}
	basePath, err := vaultCredentialBasePath(opts)
	if err != nil {
		return "", err
	}
	return basePath + "/" + url.PathEscape(credentialID), nil
}

func buildVaultCredentialActionPath(opts vaultCredentialOptions, credentialID string, action string) (string, error) {
	if strings.TrimSpace(action) == "" {
		return "", fmt.Errorf("credential action cannot be empty")
	}
	itemPath, err := buildVaultCredentialItemPath(opts, credentialID)
	if err != nil {
		return "", err
	}
	return itemPath + "/" + url.PathEscape(strings.TrimSpace(action)), nil
}

func vaultCredentialBasePath(opts vaultCredentialOptions) (string, error) {
	if strings.TrimSpace(opts.vaultID) == "" {
		return "", fmt.Errorf("--vault is required")
	}
	return corePathPrefix + "/vaults/" + url.PathEscape(strings.TrimSpace(opts.vaultID)) + "/credentials", nil
}

func addMemoryEntryParentFlag(cmd *cobra.Command, opts *agentMemoryEntryOptions) {
	cmd.Flags().StringVar(&opts.memoryStoreID, "memory-store", "", "Parent memory store id")
	_ = cmd.MarkFlagRequired("memory-store")
}

func addMemoryEntryPayloadFlags(cmd *cobra.Command, opts *agentMemoryEntryOptions, update bool) {
	cmd.Flags().StringVar(&opts.path, "path", "", "Memory path")
	cmd.Flags().StringVar(&opts.content, "content", "", "Memory entry content")
	cmd.Flags().StringVar(&opts.contentJSON, "content-json", "", "Raw memory entry JSON payload")
	if update {
		cmd.Flags().StringVar(&opts.preconditionSHA, "precondition-content-sha256", "",
			"Only update when the current content SHA-256 matches")
	}
	cmd.Flags().StringVarP(&opts.output, "output", "o", opts.output, "Output format (text, json, yaml)")
}

func buildMemoryEntryPayload(opts agentMemoryEntryOptions, update bool) (map[string]interface{}, error) {
	if strings.TrimSpace(opts.contentJSON) != "" &&
		(strings.TrimSpace(opts.content) != "" || strings.TrimSpace(opts.path) != "" || strings.TrimSpace(opts.preconditionSHA) != "") {
		return nil, fmt.Errorf("--content-json cannot be combined with --content, --path, or --precondition-content-sha256")
	}
	if strings.TrimSpace(opts.contentJSON) != "" {
		payload, err := parseJSONObject(opts.contentJSON, "content-json")
		if err != nil {
			return nil, err
		}
		if !update {
			path, ok := payload["path"].(string)
			if !ok || strings.TrimSpace(path) == "" {
				return nil, fmt.Errorf("--content-json must include a non-empty path")
			}
			if _, ok := payload["content"]; !ok {
				return nil, fmt.Errorf("--content-json must include content")
			}
		}
		return payload, nil
	}

	payload := map[string]interface{}{}
	if strings.TrimSpace(opts.path) != "" {
		payload["path"] = strings.TrimSpace(opts.path)
	}
	if strings.TrimSpace(opts.content) != "" {
		payload["content"] = opts.content
	}
	if strings.TrimSpace(opts.preconditionSHA) != "" {
		payload["precondition"] = map[string]interface{}{
			"type":           "content_sha256",
			"content_sha256": strings.TrimSpace(opts.preconditionSHA),
		}
	}
	if !update {
		if _, ok := payload["path"]; !ok {
			return nil, fmt.Errorf("--path is required")
		}
		if _, ok := payload["content"]; !ok {
			return nil, fmt.Errorf("--content or --content-json is required")
		}
	} else if len(payload) == 0 {
		return nil, fmt.Errorf("at least one of --path, --content, --precondition-content-sha256, or --content-json is required")
	}
	return payload, nil
}

func buildMemoryEntryCollectionPath(opts agentMemoryEntryOptions) (string, error) {
	basePath, err := memoryEntryBasePath(opts)
	if err != nil {
		return "", err
	}
	query, err := buildManagedAgentQuery(nil, agentListOptions{
		limit: opts.limit,
		page:  opts.page,
	})
	if err != nil {
		return "", err
	}
	values := url.Values{}
	if depth := strings.TrimSpace(opts.depth); depth != "" {
		parsedDepth, err := strconv.Atoi(depth)
		if err != nil || (parsedDepth != 0 && parsedDepth != 1) {
			return "", fmt.Errorf("--depth must be 0 or 1")
		}
		values.Set("depth", strconv.Itoa(parsedDepth))
	}
	setQueryValue(values, "path_prefix", opts.pathPrefix)
	view, err := validateMemoryView(opts.view)
	if err != nil {
		return "", err
	}
	setQueryValue(values, "view", view)
	return appendQueryValues(basePath+query, values), nil
}

func buildMemoryEntryItemPath(opts agentMemoryEntryOptions, memoryID string) (string, error) {
	if strings.TrimSpace(memoryID) == "" {
		return "", fmt.Errorf("memory-id cannot be empty")
	}
	basePath, err := memoryEntryBasePath(opts)
	if err != nil {
		return "", err
	}
	view, err := validateMemoryView(opts.view)
	if err != nil {
		return "", err
	}
	return appendQueryParam(basePath+"/"+url.PathEscape(strings.TrimSpace(memoryID)), "view", view), nil
}

func buildMemoryEntryDeletePath(opts agentMemoryEntryOptions, memoryID string) (string, error) {
	itemPath, err := buildMemoryEntryItemPath(agentMemoryEntryOptions{memoryStoreID: opts.memoryStoreID}, memoryID)
	if err != nil {
		return "", err
	}
	return appendQueryParam(itemPath, "expected_content_sha256", opts.expectedSHA), nil
}

func memoryEntryBasePath(opts agentMemoryEntryOptions) (string, error) {
	if strings.TrimSpace(opts.memoryStoreID) == "" {
		return "", fmt.Errorf("--memory-store is required")
	}
	return corePathPrefix + "/memory_stores/" + url.PathEscape(strings.TrimSpace(opts.memoryStoreID)) + "/memories", nil
}

func buildMemoryVersionCollectionPath(opts agentMemoryVersionOptions) (string, error) {
	basePath, err := memoryVersionBasePath(opts)
	if err != nil {
		return "", err
	}
	query, err := buildManagedAgentQuery(nil, agentListOptions{
		limit: opts.limit,
		page:  opts.page,
	})
	if err != nil {
		return "", err
	}
	operation := strings.TrimSpace(opts.operation)
	if operation != "" && operation != "created" && operation != "modified" && operation != "deleted" {
		return "", fmt.Errorf("--operation must be one of: created, modified, deleted")
	}
	values := url.Values{}
	setQueryValue(values, "memory_id", opts.memoryID)
	setQueryValue(values, "api_key_id", opts.apiKeyID)
	setQueryValue(values, "operation", operation)
	setQueryValue(values, "created_at[gte]", opts.createdAtGTE)
	setQueryValue(values, "created_at[lte]", opts.createdAtLTE)
	view, err := validateMemoryView(opts.view)
	if err != nil {
		return "", err
	}
	setQueryValue(values, "view", view)
	return appendQueryValues(basePath+query, values), nil
}

func buildMemoryVersionItemPath(opts agentMemoryVersionOptions, versionID string) (string, error) {
	if strings.TrimSpace(versionID) == "" {
		return "", fmt.Errorf("version-id cannot be empty")
	}
	basePath, err := memoryVersionBasePath(opts)
	if err != nil {
		return "", err
	}
	view, err := validateMemoryView(opts.view)
	if err != nil {
		return "", err
	}
	path := basePath + "/" + url.PathEscape(strings.TrimSpace(versionID))
	return appendQueryParam(path, "view", view), nil
}

func validateMemoryView(value string) (string, error) {
	view := strings.TrimSpace(value)
	if view != "" && view != "basic" && view != "full" {
		return "", fmt.Errorf("--view must be one of: basic, full")
	}
	return view, nil
}

func buildMemoryVersionRedactPath(opts agentMemoryVersionOptions, versionID string) (string, error) {
	itemPath, err := buildMemoryVersionItemPath(opts, versionID)
	if err != nil {
		return "", err
	}
	return itemPath + "/redact", nil
}

func memoryVersionBasePath(opts agentMemoryVersionOptions) (string, error) {
	if strings.TrimSpace(opts.memoryStoreID) == "" {
		return "", fmt.Errorf("--memory-store is required")
	}
	return corePathPrefix + "/memory_stores/" + url.PathEscape(strings.TrimSpace(opts.memoryStoreID)) + "/memory_versions", nil
}

func addMemoryVersionParentFlag(cmd *cobra.Command, opts *agentMemoryVersionOptions) {
	cmd.Flags().StringVar(&opts.memoryStoreID, "memory-store", "", "Parent memory store id")
	_ = cmd.MarkFlagRequired("memory-store")
}

func appendQueryParam(path string, key string, value string) string {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return path
	}
	parsed, err := url.Parse(path)
	if err != nil {
		values := url.Values{}
		values.Set(strings.TrimSpace(key), strings.TrimSpace(value))
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		return path + separator + values.Encode()
	}
	values := parsed.Query()
	values.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func setQueryValue(values url.Values, key string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		values.Set(key, trimmed)
	}
}

func appendQueryValues(path string, values url.Values) string {
	if len(values) == 0 {
		return path
	}
	parsed, err := url.Parse(path)
	if err != nil {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		return path + separator + values.Encode()
	}
	query := parsed.Query()
	for key, entries := range values {
		for _, value := range entries {
			query.Add(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func buildFileContentPath(fileID string) (string, error) {
	if strings.TrimSpace(fileID) == "" {
		return "", fmt.Errorf("file-id cannot be empty")
	}
	return corePathPrefix + "/files/" + url.PathEscape(strings.TrimSpace(fileID)) + "/content", nil
}

func buildSkillVersionContentPath(skillID string, version string) (string, error) {
	if strings.TrimSpace(skillID) == "" {
		return "", fmt.Errorf("skill-id cannot be empty")
	}
	if strings.TrimSpace(version) == "" {
		return "", fmt.Errorf("version cannot be empty")
	}
	return corePathPrefix + "/skills/" + url.PathEscape(strings.TrimSpace(skillID)) +
		"/versions/" + url.PathEscape(strings.TrimSpace(version)) + "/content", nil
}

func buildSessionOutcomePath(sessionID string) (string, error) {
	if strings.TrimSpace(sessionID) == "" {
		return "", fmt.Errorf("session-id cannot be empty")
	}
	return corePathPrefix + "/sessions/" + url.PathEscape(strings.TrimSpace(sessionID)) + "/outcome", nil
}

func buildSessionFileContentPath(opts agentSessionChildOptions, fileID string) (string, error) {
	itemPath, err := buildSessionChildItemPath(opts, "files", fileID)
	if err != nil {
		return "", err
	}
	return itemPath + "/content", nil
}

func downloadManagedAgentContent(
	ctx context.Context,
	client workspaceManagedAgentsClient,
	path string,
	outputPath string,
) error {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return fmt.Errorf("output path cannot be empty")
	}
	if err := os.MkdirAll(filepath.Dir(trimmed), 0700); err != nil && filepath.Dir(trimmed) != "." {
		return fmt.Errorf("failed to create output directory %q: %w", filepath.Dir(trimmed), err)
	}
	tempFile, err := os.CreateTemp(filepath.Dir(trimmed), ".orca-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary output file: %w", err)
	}
	tempName := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	if err := client.GetToWriter(ctx, path, tempFile); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to download content: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary output file: %w", err)
	}
	if existing, err := os.Stat(trimmed); err == nil && existing.IsDir() {
		return fmt.Errorf("output path %q is a directory", trimmed)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect output path %q: %w", trimmed, err)
	}
	if err := os.Remove(trimmed); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to replace output file %q: %w", trimmed, err)
	}
	if err := os.Rename(tempName, trimmed); err != nil {
		return fmt.Errorf("failed to move downloaded file to %q: %w", trimmed, err)
	}
	cleanup = false
	return nil
}

func buildAgentTriggerItemPath(id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("trigger-id cannot be empty")
	}
	return corePathPrefix + "/triggers/" + url.PathEscape(strings.TrimSpace(id)), nil
}

func buildAgentTriggerCollectionPath(cmd *cobra.Command, opts agentListOptions) (string, error) {
	query, err := buildManagedAgentQuery(cmd, opts)
	if err != nil {
		return "", err
	}
	return corePathPrefix + "/triggers" + query, nil
}

func buildAgentTriggerActionPath(id string, action string) (string, error) {
	if strings.TrimSpace(action) == "" {
		return "", fmt.Errorf("agent trigger action cannot be empty")
	}
	itemPath, err := buildAgentTriggerItemPath(id)
	if err != nil {
		return "", err
	}
	return itemPath + "/" + url.PathEscape(strings.TrimSpace(action)), nil
}

func buildAgentTriggerSessionsPath(id string, opts agentListOptions) (string, error) {
	itemPath, err := buildAgentTriggerItemPath(id)
	if err != nil {
		return "", err
	}
	query, err := buildManagedAgentQuery(nil, opts)
	if err != nil {
		return "", err
	}
	return itemPath + "/sessions" + query, nil
}

func buildSessionChildCollectionPath(
	opts agentSessionChildOptions,
	child string,
	listOpts agentListOptions,
) (string, error) {
	basePath, err := sessionChildBasePath(opts, child)
	if err != nil {
		return "", err
	}
	query, err := buildManagedAgentQuery(nil, listOpts)
	if err != nil {
		return "", err
	}
	return basePath + query, nil
}

func buildSessionChildItemPath(opts agentSessionChildOptions, child string, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("%s id cannot be empty", child)
	}
	basePath, err := sessionChildBasePath(opts, child)
	if err != nil {
		return "", err
	}
	return basePath + "/" + url.PathEscape(id), nil
}

func buildSessionEventStreamPath(opts agentSessionChildOptions) (string, error) {
	path, err := sessionChildBasePath(opts, "events/stream")
	if err != nil {
		return "", err
	}
	values := url.Values{}
	setQueryValue(values, "from_cursor", opts.fromCursor)
	setQueryValue(values, "subpath", opts.subpath)
	deltas, err := validateEventDeltas(opts.eventDeltas)
	if err != nil {
		return "", err
	}
	for _, delta := range deltas {
		values.Add("event_deltas", delta)
	}
	return appendQueryValues(path, values), nil
}

func buildSessionThreadEventCollectionPath(
	opts agentSessionChildOptions,
	listOpts agentListOptions,
) (string, error) {
	basePath, err := buildSessionChildItemPath(opts, "threads", opts.threadID)
	if err != nil {
		return "", err
	}
	query, err := buildManagedAgentQuery(nil, listOpts)
	if err != nil {
		return "", err
	}
	return basePath + "/events" + query, nil
}

func buildSessionThreadEventStreamPath(opts agentSessionChildOptions) (string, error) {
	basePath, err := buildSessionChildItemPath(opts, "threads", opts.threadID)
	if err != nil {
		return "", err
	}
	values := url.Values{}
	setQueryValue(values, "from_cursor", opts.fromCursor)
	deltas, err := validateEventDeltas(opts.eventDeltas)
	if err != nil {
		return "", err
	}
	for _, delta := range deltas {
		values.Add("event_deltas", delta)
	}
	return appendQueryValues(basePath+"/stream", values), nil
}

func validateEventDeltas(values []string) ([]string, error) {
	deltas := trimStringList(values)
	for _, delta := range deltas {
		if delta != "agent.message" && delta != "agent.thinking" {
			return nil, fmt.Errorf("--event-delta must be one of: agent.message, agent.thinking")
		}
	}
	return deltas, nil
}

func buildSessionEventStreamContext(
	parent context.Context,
	timeout string,
) (context.Context, context.CancelFunc, bool, error) {
	trimmed := strings.TrimSpace(timeout)
	if trimmed == "" || trimmed == "0" {
		return parent, func() {}, false, nil
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return nil, nil, false, fmt.Errorf("invalid --timeout %q: %w", timeout, err)
	}
	if duration < 0 {
		return nil, nil, false, fmt.Errorf("--timeout must be greater than or equal to 0")
	}
	if duration == 0 {
		return parent, func() {}, false, nil
	}
	ctx, cancel := context.WithTimeout(parent, duration)
	return ctx, cancel, true, nil
}

func sessionChildBasePath(opts agentSessionChildOptions, child string) (string, error) {
	if strings.TrimSpace(opts.sessionID) == "" {
		return "", fmt.Errorf("--session is required")
	}
	return corePathPrefix + "/sessions/" + url.PathEscape(opts.sessionID) + "/" + child, nil
}

func managedAgentBasePath(cmd *cobra.Command, resource managedAgentResource) (string, error) {
	if !resource.sessions {
		return corePathPrefix + resource.basePath, nil
	}
	return corePathPrefix + "/sessions", nil
}

func buildManagedAgentQuery(cmd *cobra.Command, opts agentListOptions) (string, error) {
	values := url.Values{}
	if (cmd == nil || cmd.Flags().Changed("limit")) && opts.limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", opts.limit))
	}
	if strings.TrimSpace(opts.page) != "" {
		values.Set("page", opts.page)
	}
	if strings.TrimSpace(opts.afterID) != "" {
		values.Set("after_id", strings.TrimSpace(opts.afterID))
	}
	if strings.TrimSpace(opts.beforeID) != "" {
		values.Set("before_id", strings.TrimSpace(opts.beforeID))
	}
	if (cmd == nil || cmd.Flags().Changed("include-archived")) && opts.includeArchived {
		values.Set("include_archived", fmt.Sprintf("%t", opts.includeArchived))
	}
	if strings.TrimSpace(opts.agentID) != "" {
		values.Set("agent_id", strings.TrimSpace(opts.agentID))
	}
	if strings.TrimSpace(opts.order) != "" {
		order := strings.TrimSpace(opts.order)
		if order != "asc" && order != "desc" {
			return "", fmt.Errorf("--order must be one of: asc, desc")
		}
		values.Set("order", order)
	}
	if strings.TrimSpace(opts.createdAtGT) != "" {
		values.Set("created_at[gt]", strings.TrimSpace(opts.createdAtGT))
	}
	if strings.TrimSpace(opts.createdAtGTE) != "" {
		values.Set("created_at[gte]", strings.TrimSpace(opts.createdAtGTE))
	}
	if strings.TrimSpace(opts.createdAtLT) != "" {
		values.Set("created_at[lt]", strings.TrimSpace(opts.createdAtLT))
	}
	if strings.TrimSpace(opts.createdAtLTE) != "" {
		values.Set("created_at[lte]", strings.TrimSpace(opts.createdAtLTE))
	}
	for _, eventType := range trimStringList(opts.eventTypes) {
		values.Add("types", eventType)
	}
	if strings.TrimSpace(opts.subpath) != "" {
		values.Set("subpath", strings.TrimSpace(opts.subpath))
	}
	if len(values) == 0 {
		return "", nil
	}
	return "?" + values.Encode(), nil
}

func buildManagedAgentPayload(
	cmd *cobra.Command,
	resource managedAgentResource,
	opts agentPayloadOptions,
	update bool,
) (map[string]interface{}, error) {
	payload := map[string]interface{}{}

	metadata, err := parseKeyValuePairs(opts.metadata, "metadata")
	if err != nil {
		return nil, err
	}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}

	switch {
	case resource.use == "agent":
		if err := buildAgentPayload(cmd, payload, opts, update); err != nil {
			return nil, err
		}
	case resource.use == "environments":
		if err := buildEnvironmentPayload(cmd, payload, opts, update); err != nil {
			return nil, err
		}
	case resource.use == "memory-stores":
		if err := buildMemoryStorePayload(cmd, payload, opts, update); err != nil {
			return nil, err
		}
	case resource.use == "vaults":
		if !update && strings.TrimSpace(opts.displayName) == "" {
			return nil, fmt.Errorf("--display-name is required")
		}
		setStringPayload(cmd, payload, "display-name", "display_name", opts.displayName, update)
	case resource.sessions:
		if err := buildSessionPayload(cmd, payload, opts, update); err != nil {
			return nil, err
		}
	}
	return payload, nil
}

func buildVaultCredentialPayload(
	cmd *cobra.Command,
	opts vaultCredentialOptions,
	update bool,
) (map[string]interface{}, error) {
	payload := map[string]interface{}{}
	setStringPayload(cmd, payload, "display-name", "display_name", opts.displayName, update)

	if includeStringFlag(cmd, "auth-json", opts.authJSON, update) {
		auth, err := parseJSONObject(opts.authJSON, "auth-json")
		if err != nil {
			return nil, err
		}
		payload["auth"] = auth
	}
	metadata, err := parseKeyValuePairs(opts.metadata, "metadata")
	if err != nil {
		return nil, err
	}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}
	return payload, nil
}

func buildAgentTriggerPayload(
	cmd *cobra.Command,
	opts agentTriggerOptions,
	update bool,
) (map[string]interface{}, error) {
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
	if err := validateAgentTriggerFlags(cmd, opts, update); err != nil {
		return nil, err
	}

	payload := map[string]interface{}{}
	setStringPayload(cmd, payload, "name", "name", opts.name, update)
	if !update {
		agentRef := map[string]interface{}{
			"type": "agent",
			"id":   strings.TrimSpace(opts.agentID),
		}
		if opts.agentVersion > 0 {
			agentRef["version"] = opts.agentVersion
		}
		payload["agent"] = agentRef
	}
	if includeStringFlag(cmd, "session-mode", opts.sessionMode, update) {
		payload["session_mode"] = strings.ToUpper(strings.TrimSpace(opts.sessionMode))
	}

	source, err := buildAgentTriggerSourcePayload(cmd, opts, update)
	if err != nil {
		return nil, err
	}
	if len(source) > 0 {
		payload["source"] = source
	}

	session, err := buildAgentTriggerSessionPayload(cmd, opts, update)
	if err != nil {
		return nil, err
	}
	if len(session) > 0 {
		payload["session"] = session
	}
	if cmd.Flags().Changed("replicas") {
		payload["replicas"] = opts.replicas
	}
	if !update && cmd.Flags().Changed("paused") {
		payload["paused"] = opts.paused
	}
	return payload, nil
}

func validateAgentTriggerFlags(cmd *cobra.Command, opts agentTriggerOptions, update bool) error {
	if strings.TrimSpace(opts.name) == "" && !update {
		return fmt.Errorf("--name is required")
	}
	if !update {
		if strings.TrimSpace(opts.agentID) == "" {
			return fmt.Errorf("--agent is required")
		}
		if cmd.Flags().Changed("agent-version") && opts.agentVersion <= 0 {
			return fmt.Errorf("--agent-version must be greater than 0")
		}
		if strings.TrimSpace(opts.environmentID) == "" {
			return fmt.Errorf("--environment-id is required")
		}
	}

	sessionMode := strings.ToUpper(strings.TrimSpace(opts.sessionMode))
	if !update || cmd.Flags().Changed("session-mode") {
		switch sessionMode {
		case "SESSION_PER_EVENT", "SESSION_PER_TOPIC", "SESSION_PER_KEY", "SHARED":
		default:
			return fmt.Errorf("--session-mode must be one of: SESSION_PER_EVENT, SESSION_PER_TOPIC, SESSION_PER_KEY, SHARED")
		}
	}

	sourceType := strings.ToLower(strings.TrimSpace(opts.sourceType))
	if !update || cmd.Flags().Changed("source-type") {
		switch sourceType {
		case "pulsar", "kafka", "cron":
		default:
			return fmt.Errorf("--source-type must be one of: pulsar, kafka, cron")
		}
	}
	hasTopics := len(trimStringList(opts.topics)) > 0
	hasTopicPattern := strings.TrimSpace(opts.topicPattern) != ""
	if hasTopics && hasTopicPattern {
		return fmt.Errorf("--topic and --topic-pattern cannot be used together")
	}
	if update {
		if agentTriggerSourceFlagsChanged(cmd) && !cmd.Flags().Changed("source-type") {
			return fmt.Errorf("--source-type is required when updating trigger source fields")
		}
		return nil
	}
	if sourceType == "cron" {
		if sessionMode != "SESSION_PER_EVENT" && sessionMode != "SHARED" {
			return fmt.Errorf("cron triggers require --session-mode SESSION_PER_EVENT or SHARED")
		}
		if strings.TrimSpace(opts.schedule) == "" {
			return fmt.Errorf("--schedule is required for cron triggers")
		}
		if strings.TrimSpace(opts.payload) == "" {
			return fmt.Errorf("--payload is required for cron triggers")
		}
		return nil
	}
	if strings.TrimSpace(opts.connection) == "" {
		return fmt.Errorf("--connection is required for %s triggers", sourceType)
	}
	if hasTopics == hasTopicPattern {
		return fmt.Errorf("exactly one of --topic or --topic-pattern is required")
	}
	return nil
}

func agentTriggerSourceFlagsChanged(cmd *cobra.Command) bool {
	for _, flag := range []string{
		"connection",
		"topic",
		"topic-pattern",
		"subscription-name",
		"type-class-name",
		"type-class-definition",
		"type-class-definition-file",
		"schema-type",
		"consumer-config",
		"input-schema-config",
		"schedule",
		"timezone",
		"payload",
	} {
		if cmd.Flags().Changed(flag) {
			return true
		}
	}
	return false
}

func buildAgentTriggerSourcePayload(
	cmd *cobra.Command,
	opts agentTriggerOptions,
	update bool,
) (map[string]interface{}, error) {
	source := map[string]interface{}{}
	if includeStringFlag(cmd, "source-type", opts.sourceType, update) {
		source["type"] = strings.ToLower(strings.TrimSpace(opts.sourceType))
	}
	setStringPayload(cmd, source, "connection", "connection", opts.connection, update)
	if len(opts.topics) > 0 {
		source["topics"] = trimStringList(opts.topics)
	}
	setStringPayload(cmd, source, "topic-pattern", "topic_pattern", opts.topicPattern, update)
	setStringPayload(cmd, source, "subscription-name", "subscription_name", opts.subscriptionName, update)
	setStringPayload(cmd, source, "type-class-name", "type_class_name", opts.typeClassName, update)
	if strings.TrimSpace(opts.typeClassDefinition) != "" && strings.TrimSpace(opts.typeClassDefinitionFile) != "" {
		return nil, fmt.Errorf("--type-class-definition and --type-class-definition-file cannot be used together")
	}
	typeClassDefinition := opts.typeClassDefinition
	if strings.TrimSpace(opts.typeClassDefinitionFile) != "" {
		data, err := os.ReadFile(opts.typeClassDefinitionFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read --type-class-definition-file %q: %w", opts.typeClassDefinitionFile, err)
		}
		typeClassDefinition = string(data)
	}
	if includeStringFlag(cmd, "type-class-definition", typeClassDefinition, update) ||
		(update && cmd.Flags().Changed("type-class-definition-file")) ||
		(!update && strings.TrimSpace(typeClassDefinition) != "") {
		source["type_class_definition"] = typeClassDefinition
	}
	setStringPayload(cmd, source, "schema-type", "schema_type", opts.schemaType, update)
	setStringPayload(cmd, source, "schedule", "schedule", opts.schedule, update)
	setStringPayload(cmd, source, "timezone", "timezone", opts.timezone, update)
	setStringPayload(cmd, source, "payload", "payload", opts.payload, update)
	consumerConfig, err := parseKeyValueScalarPairs(opts.consumerConfigs, "consumer-config")
	if err != nil {
		return nil, err
	}
	if len(consumerConfig) > 0 {
		source["consumer_additional_config"] = consumerConfig
	}
	inputSchemaConfig, err := parseKeyValueObjectPairs(opts.inputSchemaConfigs, "input-schema-config")
	if err != nil {
		return nil, err
	}
	if len(inputSchemaConfig) > 0 {
		source["input_schema_configs"] = inputSchemaConfig
	}
	return source, nil
}

func buildAgentTriggerSessionPayload(
	cmd *cobra.Command,
	opts agentTriggerOptions,
	update bool,
) (map[string]interface{}, error) {
	session := map[string]interface{}{}
	setStringPayload(cmd, session, "environment-id", "environment_id", opts.environmentID, update)
	setStringPayload(cmd, session, "title-template", "title_template", opts.titleTemplate, update)
	metadata, err := parseKeyValuePairs(opts.sessionMetadata, "metadata")
	if err != nil {
		return nil, err
	}
	if len(metadata) > 0 {
		session["metadata"] = metadata
	}
	if len(opts.vaultIDs) > 0 {
		vaultIDs := make([]string, 0, len(opts.vaultIDs))
		for _, vaultID := range opts.vaultIDs {
			if strings.TrimSpace(vaultID) != "" {
				vaultIDs = append(vaultIDs, strings.TrimSpace(vaultID))
			}
		}
		session["vault_ids"] = vaultIDs
	}
	return session, nil
}

func buildSessionEventsPayload(opts agentSessionChildOptions) (map[string]interface{}, error) {
	events, err := parseJSONObjectList(opts.eventJSONs, "event-json")
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("at least one --event-json is required")
	}
	return map[string]interface{}{"events": events}, nil
}

func buildAgentPayload(
	cmd *cobra.Command,
	payload map[string]interface{},
	opts agentPayloadOptions,
	update bool,
) error {
	if update && cmd.Flags().Changed("version") {
		if opts.version <= 0 {
			return fmt.Errorf("--version must be greater than zero")
		}
		payload["version"] = opts.version
	}
	if strings.TrimSpace(opts.model) != "" && strings.TrimSpace(opts.modelJSON) != "" {
		return fmt.Errorf("--model and --model-json cannot be used together")
	}
	if !update && strings.TrimSpace(opts.model) == "" && strings.TrimSpace(opts.modelJSON) == "" {
		return fmt.Errorf("--model is required for agent create")
	}
	if !update && strings.TrimSpace(opts.name) == "" {
		return fmt.Errorf("--name is required for agent create")
	}
	if includeStringFlag(cmd, "model", opts.model, update) {
		payload["model"] = strings.TrimSpace(opts.model)
	}
	if includeStringFlag(cmd, "model-json", opts.modelJSON, update) {
		model, err := parseJSONValue(opts.modelJSON, "model-json")
		if err != nil {
			return err
		}
		payload["model"] = model
	}
	setStringPayload(cmd, payload, "name", "name", opts.name, update)
	setStringPayload(cmd, payload, "description", "description", opts.description, update)
	setStringPayload(cmd, payload, "system", "system", opts.system, update)

	mcpServers, err := parseMapListFlag(opts.mcpServers, "mcp-server")
	if err != nil {
		return err
	}
	if len(mcpServers) > 0 {
		payload["mcp_servers"] = mcpServers
	}

	tools, err := parseJSONObjectList(opts.tools, "tool-json")
	if err != nil {
		return err
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}

	skills, err := parseSkillReferences(opts.skills)
	if err != nil {
		return err
	}
	if len(skills) > 0 {
		payload["skills"] = skills
	}

	multiagent, err := buildAgentMultiagent(opts)
	if err != nil {
		return err
	}
	if multiagent != nil {
		payload["multiagent"] = multiagent
	}
	return nil
}

func buildMemoryStorePayload(
	cmd *cobra.Command,
	payload map[string]interface{},
	opts agentPayloadOptions,
	update bool,
) error {
	if !update && strings.TrimSpace(opts.name) == "" {
		return fmt.Errorf("--name is required for memory store create")
	}
	setStringPayload(cmd, payload, "name", "name", opts.name, update)
	setStringPayload(cmd, payload, "description", "description", opts.description, update)
	return nil
}

func buildEnvironmentPayload(
	cmd *cobra.Command,
	payload map[string]interface{},
	opts agentPayloadOptions,
	update bool,
) error {
	if !update && strings.TrimSpace(opts.name) == "" {
		return fmt.Errorf("--name is required for environment create")
	}
	setStringPayload(cmd, payload, "name", "name", opts.name, update)
	setStringPayload(cmd, payload, "description", "description", opts.description, update)
	if scope := strings.TrimSpace(opts.scope); scope != "" && scope != "organization" && scope != "account" {
		return fmt.Errorf("--scope must be one of: organization, account")
	}
	setStringPayload(cmd, payload, "scope", "scope", opts.scope, update)

	includeConfig := !update ||
		cmd.Flags().Changed("config-json") ||
		cmd.Flags().Changed("config-type") ||
		cmd.Flags().Changed("networking-type") ||
		cmd.Flags().Changed("config") ||
		environmentPackageFlagsChanged(cmd)
	if !includeConfig {
		return nil
	}

	config := map[string]interface{}{}
	if strings.TrimSpace(opts.configJSON) != "" {
		parsed, err := parseJSONObject(opts.configJSON, "config-json")
		if err != nil {
			return err
		}
		config = parsed
	}
	if strings.TrimSpace(opts.configJSON) == "" || cmd.Flags().Changed("config-type") {
		if strings.TrimSpace(opts.configType) != "" {
			config["type"] = strings.TrimSpace(opts.configType)
		}
	}
	if strings.TrimSpace(opts.configJSON) == "" || cmd.Flags().Changed("networking-type") {
		if strings.TrimSpace(opts.networkingType) != "" {
			networking, _ := config["networking"].(map[string]interface{})
			if networking == nil {
				networking = map[string]interface{}{}
			}
			networking["type"] = strings.TrimSpace(opts.networkingType)
			config["networking"] = networking
		}
	}
	properties, err := parseKeyValuePairs(opts.configProperties, "config")
	if err != nil {
		return err
	}
	for key, value := range properties {
		config[key] = parseScalarValue(value)
	}
	if err := mergeEnvironmentPackageFlags(cmd, config, opts); err != nil {
		return err
	}
	payload["config"] = config
	return nil
}

var environmentPackageManagers = []string{"apt", "cargo", "gem", "go", "npm", "pip"}

func environmentPackageFlagsChanged(cmd *cobra.Command) bool {
	for _, manager := range environmentPackageManagers {
		if cmd.Flags().Changed("package-" + manager) {
			return true
		}
	}
	return false
}

func mergeEnvironmentPackageFlags(
	cmd *cobra.Command,
	config map[string]interface{},
	opts agentPayloadOptions,
) error {
	packages, err := normalizeEnvironmentPackages(config["packages"])
	if err != nil {
		return err
	}
	for _, item := range []struct {
		manager string
		values  []string
	}{
		{manager: "apt", values: opts.packageApt},
		{manager: "cargo", values: opts.packageCargo},
		{manager: "gem", values: opts.packageGem},
		{manager: "go", values: opts.packageGo},
		{manager: "npm", values: opts.packageNPM},
		{manager: "pip", values: opts.packagePip},
	} {
		if !cmd.Flags().Changed("package-" + item.manager) {
			continue
		}
		values := trimStringList(item.values)
		if len(values) == 0 {
			delete(packages, item.manager)
			continue
		}
		packages[item.manager] = values
	}
	managerCount := len(packages)
	if _, ok := packages["type"]; ok {
		managerCount--
	}
	if managerCount == 0 {
		delete(config, "packages")
		return nil
	}
	packages["type"] = "packages"
	config["packages"] = packages
	return nil
}

func normalizeEnvironmentPackages(value interface{}) (map[string]interface{}, error) {
	if value == nil {
		return map[string]interface{}{}, nil
	}
	if legacy, ok, err := normalizeStringList(value); ok || err != nil {
		if err != nil {
			return nil, fmt.Errorf("config.packages must be an object or string array: %w", err)
		}
		if len(legacy) == 0 {
			return map[string]interface{}{}, nil
		}
		return map[string]interface{}{"type": "packages", "apt": legacy}, nil
	}
	object, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("config.packages must be an object or string array")
	}
	packages := map[string]interface{}{}
	for key, raw := range object {
		manager := strings.TrimSpace(key)
		if manager == "type" {
			typeName, ok := raw.(string)
			if !ok || strings.TrimSpace(typeName) != "packages" {
				return nil, fmt.Errorf("config.packages.type must be packages")
			}
			packages["type"] = "packages"
			continue
		}
		if !isEnvironmentPackageManager(manager) {
			return nil, fmt.Errorf("config.packages.%s is not supported", key)
		}
		values, ok, err := normalizeStringList(raw)
		if err != nil || !ok {
			if err != nil {
				return nil, fmt.Errorf("config.packages.%s must be a string array: %w", key, err)
			}
			return nil, fmt.Errorf("config.packages.%s must be a string array", key)
		}
		if len(values) > 0 {
			packages[manager] = values
		}
	}
	return packages, nil
}

func isEnvironmentPackageManager(manager string) bool {
	for _, allowed := range environmentPackageManagers {
		if manager == allowed {
			return true
		}
	}
	return false
}

func normalizeStringList(value interface{}) ([]string, bool, error) {
	switch typed := value.(type) {
	case []string:
		return trimStringList(typed), true, nil
	case []interface{}:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, true, fmt.Errorf("found %T", item)
			}
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				values = append(values, trimmed)
			}
		}
		return values, true, nil
	default:
		return nil, false, nil
	}
}

func trimStringList(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			trimmed = append(trimmed, text)
		}
	}
	return trimmed
}

func buildSessionPayload(
	cmd *cobra.Command,
	payload map[string]interface{},
	opts agentPayloadOptions,
	update bool,
) error {
	agentID, err := cmd.Flags().GetString("agent")
	if err != nil {
		return err
	}
	agentID = strings.TrimSpace(agentID)
	agentJSON := strings.TrimSpace(opts.agentJSON)
	agentVersion, err := cmd.Flags().GetInt("agent-version")
	if err != nil {
		return err
	}
	if agentJSON != "" && (agentID != "" || cmd.Flags().Changed("agent-version")) {
		return fmt.Errorf("--agent-json cannot be combined with --agent or --agent-version")
	}

	if !update {
		if strings.TrimSpace(opts.environmentID) == "" {
			return fmt.Errorf("--environment-id is required")
		}
		payload["environment_id"] = strings.TrimSpace(opts.environmentID)
		if agentJSON != "" {
			agent, err := parseJSONObject(agentJSON, "agent-json")
			if err != nil {
				return err
			}
			payload["agent"] = agent
		} else if agentID != "" {
			if agentVersion > 0 {
				payload["agent"] = map[string]interface{}{
					"type":    "agent",
					"id":      agentID,
					"version": agentVersion,
				}
			} else {
				payload["agent"] = agentID
			}
		} else if cmd.Flags().Changed("agent-version") {
			return fmt.Errorf("--agent is required when --agent-version is set")
		}
	} else if agentJSON != "" {
		agent, err := parseJSONObject(agentJSON, "agent-json")
		if err != nil {
			return err
		}
		payload["agent"] = agent
	} else if cmd.Flags().Changed("agent") || cmd.Flags().Changed("agent-version") {
		if agentID == "" {
			return fmt.Errorf("--agent is required when --agent-version is set")
		}
		agent := map[string]interface{}{
			"type": "agent",
			"id":   agentID,
		}
		if agentVersion > 0 {
			agent["version"] = agentVersion
		}
		payload["agent"] = agent
	}
	if len(opts.vaultIDs) > 0 {
		vaultIDs := make([]string, 0, len(opts.vaultIDs))
		for _, vaultID := range opts.vaultIDs {
			if strings.TrimSpace(vaultID) != "" {
				vaultIDs = append(vaultIDs, strings.TrimSpace(vaultID))
			}
		}
		payload["vault_ids"] = vaultIDs
	}
	setStringPayload(cmd, payload, "title", "title", opts.title, update)

	if !update {
		resources, err := parseJSONObjectList(opts.resources, "resource-json")
		if err != nil {
			return err
		}
		if len(resources) > 0 {
			payload["resources"] = resources
		}
		initialEvents, err := parseJSONObjectList(opts.initialEvents, "initial-event-json")
		if err != nil {
			return err
		}
		if len(initialEvents) > 0 {
			payload["initial_events"] = initialEvents
		}
	}
	return nil
}

func setStringPayload(
	cmd *cobra.Command,
	payload map[string]interface{},
	flagName string,
	key string,
	value string,
	update bool,
) {
	if includeStringFlag(cmd, flagName, value, update) {
		payload[key] = strings.TrimSpace(value)
	}
}

func includeStringFlag(cmd *cobra.Command, flagName string, value string, update bool) bool {
	if update {
		return cmd.Flags().Changed(flagName)
	}
	return strings.TrimSpace(value) != ""
}

func parseKeyValuePairs(values []string, flagName string) (map[string]string, error) {
	result := map[string]string{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("--%s must use key=value format", flagName)
		}
		result[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return result, nil
}

func parseKeyValueScalarPairs(values []string, flagName string) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("--%s must use key=value format", flagName)
		}
		result[strings.TrimSpace(key)] = parseScalarValue(strings.TrimSpace(val))
	}
	return result, nil
}

func parseKeyValueObjectPairs(values []string, flagName string) (map[string]map[string]interface{}, error) {
	result := map[string]map[string]interface{}{}
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("--%s must use key=<json-object> format", flagName)
		}
		parsed, err := parseJSONObject(strings.TrimSpace(raw), flagName)
		if err != nil {
			return nil, err
		}
		result[strings.TrimSpace(key)] = parsed
	}
	return result, nil
}

func parseMapListFlag(values []string, flagName string) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		pairs := strings.Split(value, ",")
		item := map[string]interface{}{}
		for _, pair := range pairs {
			key, val, ok := strings.Cut(pair, "=")
			if !ok || strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("--%s must use comma-separated key=value entries", flagName)
			}
			item[strings.TrimSpace(key)] = parseScalarValue(strings.TrimSpace(val))
		}
		result = append(result, item)
	}
	return result, nil
}

func parseJSONObjectList(values []string, flagName string) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		parsed, err := parseJSONObject(value, flagName)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, nil
}

func parseJSONObject(value string, flagName string) (map[string]interface{}, error) {
	parsed, err := parseJSONValue(value, flagName)
	if err != nil {
		return nil, err
	}
	object, ok := parsed.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("--%s must be a JSON object", flagName)
	}
	return object, nil
}

func parseJSONValue(value string, flagName string) (interface{}, error) {
	var parsed interface{}
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse --%s as JSON: %w", flagName, err)
	}
	return parsed, nil
}

func parseScalarValue(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed
	}
	return trimmed
}

func parseSkillReferences(values []string) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, fmt.Errorf("--skill cannot be empty")
		}
		skillType := ""
		if prefix, remainder, ok := strings.Cut(trimmed, ":"); ok && (prefix == "anthropic" || prefix == "custom") {
			skillType = prefix
			trimmed = strings.TrimSpace(remainder)
		}
		skillID := trimmed
		version := ""
		if parsedSkillID, parsedVersion, ok := strings.Cut(trimmed, "@"); ok {
			if strings.TrimSpace(parsedSkillID) == "" || strings.TrimSpace(parsedVersion) == "" {
				return nil, fmt.Errorf("--skill must use [anthropic:|custom:]<skill-id>[@<version>] format")
			}
			skillID = strings.TrimSpace(parsedSkillID)
			version = strings.TrimSpace(parsedVersion)
		}
		if strings.TrimSpace(skillID) == "" {
			return nil, fmt.Errorf("--skill cannot be empty")
		}
		if skillType == "" {
			if strings.HasPrefix(skillID, "skill_") {
				skillType = "custom"
			} else {
				skillType = "anthropic"
			}
		}
		item := map[string]interface{}{
			"type":     skillType,
			"skill_id": skillID,
		}
		if version != "" {
			item["version"] = version
		}
		result = append(result, item)
	}
	return result, nil
}

func buildAgentMultiagent(opts agentPayloadOptions) (map[string]interface{}, error) {
	if strings.TrimSpace(opts.multiagentType) == "" && len(opts.multiagentAgents) == 0 {
		return nil, nil
	}
	multiagent := map[string]interface{}{}
	if strings.TrimSpace(opts.multiagentType) != "" {
		multiagent["type"] = strings.TrimSpace(opts.multiagentType)
	}
	agents, err := parseMultiagentRoster(opts.multiagentAgents)
	if err != nil {
		return nil, err
	}
	if len(agents) > 0 {
		multiagent["agents"] = agents
	}
	return multiagent, nil
}

func parseMultiagentRoster(values []string) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, fmt.Errorf("--multiagent-agent cannot be empty")
		}
		if strings.Contains(trimmed, "=") {
			entry, err := parseMultiagentRosterMap(trimmed)
			if err != nil {
				return nil, err
			}
			result = append(result, entry)
			continue
		}
		entry := map[string]interface{}{
			"type": "agent",
			"id":   trimmed,
		}
		if id, version, ok := strings.Cut(trimmed, "@"); ok {
			parsedVersion, err := strconv.Atoi(strings.TrimSpace(version))
			if err != nil || strings.TrimSpace(id) == "" {
				return nil, fmt.Errorf("--multiagent-agent must use <agent-id> or <agent-id>@<version> format")
			}
			entry["id"] = strings.TrimSpace(id)
			entry["version"] = parsedVersion
		}
		result = append(result, entry)
	}
	return result, nil
}

func parseMultiagentRosterMap(value string) (map[string]interface{}, error) {
	pairs, err := parseMapListFlag([]string{value}, "multiagent-agent")
	if err != nil {
		return nil, err
	}
	entry := pairs[0]
	if version, ok := entry["version"]; ok {
		switch typed := version.(type) {
		case float64:
			if typed != math.Trunc(typed) {
				return nil, fmt.Errorf("--multiagent-agent version must be an integer")
			}
			entry["version"] = int(typed)
		case string:
			parsed, err := strconv.Atoi(typed)
			if err != nil {
				return nil, fmt.Errorf("--multiagent-agent version must be an integer")
			}
			entry["version"] = parsed
		}
	}
	return entry, nil
}

func buildSessionResourcePayload(opts agentSessionChildOptions, update bool) (map[string]interface{}, error) {
	if update {
		if strings.TrimSpace(opts.authorizationToken) == "" {
			return nil, fmt.Errorf("--authorization-token is required")
		}
		return map[string]interface{}{
			"authorization_token": strings.TrimSpace(opts.authorizationToken),
		}, nil
	}

	payload := map[string]interface{}{}
	resourceType := normalizeSessionResourceType(opts.resourceType)
	if resourceType == "" {
		return nil, fmt.Errorf("--type is required")
	}
	payload["type"] = resourceType
	switch resourceType {
	case "file":
		if strings.TrimSpace(opts.fileID) == "" {
			return nil, fmt.Errorf("--file-id is required when --type=file")
		}
		setPayloadString(payload, "file_id", opts.fileID)
		setPayloadString(payload, "mount_path", opts.mountPath)
		setPayloadString(payload, "access", opts.access)
		setPayloadString(payload, "instructions", opts.instructions)
		if strings.TrimSpace(opts.mountStrategy) != "" {
			if strings.TrimSpace(opts.mountStrategy) != "tarball_prefetch" {
				return nil, fmt.Errorf("--mount-strategy must be tarball_prefetch")
			}
			payload["mount_strategy"] = "tarball_prefetch"
		}
	case "memory_store":
		if strings.TrimSpace(opts.memoryStoreID) == "" {
			return nil, fmt.Errorf("--memory-store-id is required when --type=memory-store")
		}
		setPayloadString(payload, "memory_store_id", opts.memoryStoreID)
		setPayloadString(payload, "access", opts.access)
		setPayloadString(payload, "instructions", opts.instructions)
	case "github_repository":
		if strings.TrimSpace(opts.url) == "" {
			return nil, fmt.Errorf("--url is required when --type=github-repository")
		}
		if strings.TrimSpace(opts.authorizationToken) == "" {
			return nil, fmt.Errorf("--authorization-token is required when --type=github-repository")
		}
		setPayloadString(payload, "authorization_token", opts.authorizationToken)
		setPayloadString(payload, "url", opts.url)
		setPayloadString(payload, "mount_path", opts.mountPath)
		setPayloadString(payload, "access", opts.access)
		setPayloadString(payload, "instructions", opts.instructions)
		checkout, err := buildSessionResourceCheckout(opts)
		if err != nil {
			return nil, err
		}
		if checkout != nil {
			payload["checkout"] = checkout
		}
	default:
		return nil, fmt.Errorf("--type must be one of: file, memory-store, github-repository")
	}
	return payload, nil
}

func normalizeSessionResourceType(value string) string {
	switch strings.TrimSpace(value) {
	case "memory-store":
		return "memory_store"
	case "github-repository":
		return "github_repository"
	default:
		return strings.TrimSpace(value)
	}
}

func buildSessionResourceCheckout(opts agentSessionChildOptions) (map[string]interface{}, error) {
	if strings.TrimSpace(opts.branch) != "" && strings.TrimSpace(opts.commit) != "" {
		return nil, fmt.Errorf("--branch and --commit cannot be used together")
	}
	checkoutType := strings.TrimSpace(opts.checkoutType)
	if checkoutType == "" {
		if strings.TrimSpace(opts.branch) != "" {
			checkoutType = "branch"
		} else if strings.TrimSpace(opts.commit) != "" {
			checkoutType = "commit"
		}
	}
	if checkoutType == "" {
		return nil, nil
	}
	checkout := map[string]interface{}{"type": checkoutType}
	switch checkoutType {
	case "branch":
		if strings.TrimSpace(opts.branch) == "" {
			return nil, fmt.Errorf("--branch is required when --checkout-type=branch")
		}
		setPayloadString(checkout, "name", opts.branch)
	case "commit":
		if strings.TrimSpace(opts.commit) == "" {
			return nil, fmt.Errorf("--commit is required when --checkout-type=commit")
		}
		setPayloadString(checkout, "sha", opts.commit)
	default:
		return nil, fmt.Errorf("--checkout-type must be one of: branch, commit")
	}
	return checkout, nil
}

func setPayloadString(payload map[string]interface{}, key string, value string) {
	if strings.TrimSpace(value) != "" {
		payload[key] = strings.TrimSpace(value)
	}
}

func buildManagedAgentMultipartRequest(
	resource managedAgentResource,
	opts agentMultipartOptions,
) (registry.MultipartRequest, error) {
	if len(opts.files) == 0 {
		return registry.MultipartRequest{}, fmt.Errorf("at least one --file is required")
	}
	if resource.file && len(opts.files) != 1 {
		return registry.MultipartRequest{}, fmt.Errorf("files create accepts exactly one --file")
	}

	request := registry.MultipartRequest{}
	if resource.skill {
		request.Fields = map[string]string{"display_title": opts.displayTitle}
	}

	uploadFileNames, err := managedAgentMultipartUploadFileNames(resource, opts.files)
	if err != nil {
		return registry.MultipartRequest{}, err
	}
	for i, filePath := range opts.files {
		file, err := readManagedAgentMultipartFile(filePath, opts.contentType, resource)
		if err != nil {
			return registry.MultipartRequest{}, err
		}
		if uploadFileNames[i] != "" {
			file.FileName = uploadFileNames[i]
		}
		if resource.file {
			file.FieldName = "file"
			request.File = file
		} else {
			file.FieldName = "files[]"
			request.Files = append(request.Files, file)
		}
	}

	return request, nil
}

func managedAgentMultipartUploadFileNames(
	resource managedAgentResource,
	filePaths []string,
) ([]string, error) {
	uploadFileNames := make([]string, len(filePaths))
	if !resource.skill {
		return uploadFileNames, nil
	}
	if len(filePaths) == 1 && strings.EqualFold(filepath.Ext(strings.TrimSpace(filePaths[0])), ".zip") {
		if strings.TrimSpace(filePaths[0]) == "" {
			return nil, fmt.Errorf("--file cannot be empty")
		}
		return uploadFileNames, nil
	}

	absolutePaths := make([]string, len(filePaths))
	var skillRoot string
	for i, filePath := range filePaths {
		trimmed := strings.TrimSpace(filePath)
		if trimmed == "" {
			return nil, fmt.Errorf("--file cannot be empty")
		}
		absolutePath, err := filepath.Abs(trimmed)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve upload file %q: %w", trimmed, err)
		}
		absolutePaths[i] = absolutePath
		if filepath.Base(absolutePath) == "SKILL.md" {
			skillRoot = filepath.Dir(absolutePath)
		}
	}
	if skillRoot == "" {
		return nil, fmt.Errorf("skill uploads must include a file named SKILL.md")
	}

	topLevelParent := filepath.Dir(skillRoot)
	topLevelDir := filepath.Base(skillRoot)
	for i, absolutePath := range absolutePaths {
		relativePath, err := filepath.Rel(topLevelParent, absolutePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve skill upload path %q: %w", filePaths[i], err)
		}
		if relativePath == "." ||
			strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) ||
			filepath.IsAbs(relativePath) ||
			(relativePath != topLevelDir &&
				!strings.HasPrefix(relativePath, topLevelDir+string(filepath.Separator))) {
			return nil, fmt.Errorf("skill upload file %q must be under the same top-level directory as SKILL.md",
				filePaths[i])
		}
		uploadFileNames[i] = filepath.ToSlash(relativePath)
	}
	return uploadFileNames, nil
}

func readManagedAgentMultipartFile(
	filePath string,
	contentType string,
	resource managedAgentResource,
) (*registry.MultipartFile, error) {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" {
		return nil, fmt.Errorf("--file cannot be empty")
	}
	content, err := os.ReadFile(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload file %q: %w", trimmed, err)
	}
	fileName := filepath.Base(trimmed)
	if strings.TrimSpace(fileName) == "" || fileName == "." || fileName == string(filepath.Separator) {
		return nil, fmt.Errorf("failed to infer upload filename from %q", trimmed)
	}
	if strings.TrimSpace(contentType) == "" && resource.skill && strings.EqualFold(fileName, "SKILL.md") {
		contentType = "text/markdown"
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = mime.TypeByExtension(filepath.Ext(fileName))
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	return &registry.MultipartFile{
		FileName:    fileName,
		ContentType: contentType,
		Content:     content,
	}, nil
}

func managedAgentSingularLabel(resource managedAgentResource) string {
	if resource.use == "agent" {
		return "managed agent"
	}
	return "managed agent " + resource.singular
}

func managedAgentPluralLabel(resource managedAgentResource) string {
	if resource.use == "agent" {
		return "managed agents"
	}
	return "managed agent " + resource.plural
}

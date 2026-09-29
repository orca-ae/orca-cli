// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// This file binds the generic managed-agent commands to the SDK's typed
// resources.
//
// The commands used to build their own request paths and send them through an
// untyped passthrough. Path construction, escaping and parameter validation now
// belong to the SDK, which tests them once for every operation rather than the
// CLI re-deriving and re-testing them. What stays here is what the CLI is
// actually responsible for: flags, validation of user input, and output.
//
// Output is rendered from the bytes the deployment sent, not from the decoded
// value. A typed struct only carries the fields the SDK models, so rendering
// one would silently drop anything else the server returned - and this CLI's
// JSON output is something people script against.

// newWorkspaceTypedClient builds the SDK client the typed commands run on. It
// is a variable so tests can point it at an httptest server.
var newWorkspaceTypedClient = func() (*registry.Client, error) {
	return newWorkspaceRegistryClient()
}

// renderRaw renders the response exactly as the deployment sent it.
//
// It decodes and re-renders rather than writing the bytes straight through, so
// the -o json / -o yaml / text formatting stays what it has always been. The
// decoded value is the server's own JSON, which is what the untyped passthrough
// produced, so output is unchanged.
func renderRaw(writer io.Writer, output string, raw []byte) error {
	var value interface{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("failed to decode the response: %w", err)
		}
	}
	return renderJSONOrText(writer, output, value)
}

// managedAgentOps binds one resource's generic commands to the typed calls that
// implement them.
//
// A generic command cannot build a path from a base string any more - each
// resource has its own method with its own parameters - so the descriptor
// carries closures instead. The command shape and flags are unchanged; only the
// request-making step moved.
//
// A nil field means the resource does not offer that operation, which is how
// the commands already decide what to register.
type managedAgentOps struct {
	list    func(ctx context.Context, client *registry.Client, opts agentListOptions, raw *[]byte) error
	get     func(ctx context.Context, client *registry.Client, id string, version param.Opt[int64], raw *[]byte) error
	remove  func(ctx context.Context, client *registry.Client, id string, raw *[]byte) error
	archive func(ctx context.Context, client *registry.Client, id string, raw *[]byte) error
}

// listPageParams converts the shared list flags into the page-token parameters
// most core resources take.
//
// Only flags the caller actually set are forwarded, matching what the CLI sent
// before: an unset --limit must not become limit=0, and values are trimmed, so
// a stray space does not become part of a cursor.
func listPageParams(opts agentListOptions) (limit param.Opt[int64], page param.Opt[string]) {
	if opts.limit > 0 {
		limit = param.Int(int64(opts.limit))
	}
	if page := strings.TrimSpace(opts.page); page != "" {
		return limit, param.String(page)
	}
	return limit, param.Opt[string]{}
}

// listIDCursorParams is listPageParams for the two resources that page by ID
// cursor rather than page token: files and session files.
func listIDCursorParams(opts agentListOptions) (limit param.Opt[int64], afterID, beforeID param.Opt[string]) {
	if opts.limit > 0 {
		limit = param.Int(int64(opts.limit))
	}
	if after := strings.TrimSpace(opts.afterID); after != "" {
		afterID = param.String(after)
	}
	if before := strings.TrimSpace(opts.beforeID); before != "" {
		beforeID = param.String(before)
	}
	return limit, afterID, beforeID
}

// optIncludeArchived forwards --include-archived only when it is true.
//
// That is what the CLI sent before: passing --include-archived=false omitted
// the parameter rather than sending false. Preserving it keeps the request
// identical for a caller who passes the flag explicitly off.
func optIncludeArchived(opts agentListOptions) param.Opt[bool] {
	if !opts.includeArchived {
		return param.Opt[bool]{}
	}
	return param.Bool(true)
}

// managedAgentOpsFor returns the typed operations for a resource descriptor.
func managedAgentOpsFor(resource managedAgentResource) managedAgentOps {
	switch {
	case resource.use == "agent":
		return managedAgentOps{
			list: func(ctx context.Context, c *registry.Client, o agentListOptions, raw *[]byte) error {
				limit, page := listPageParams(o)
				_, err := c.Agents.List(ctx, registry.AgentListParams{
					Limit:           limit,
					Page:            page,
					IncludeArchived: optIncludeArchived(o),
				}, option.WithRawJSON(raw))
				return err
			},
			get: func(ctx context.Context, c *registry.Client, id string, version param.Opt[int64], raw *[]byte) error {
				_, err := c.Agents.Get(ctx, id, registry.AgentGetParams{Version: version},
					option.WithRawJSON(raw))
				return err
			},
			archive: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Agents.Archive(ctx, id, option.WithRawJSON(raw))
				return err
			},
		}

	case resource.sessions:
		return managedAgentOps{
			list: func(ctx context.Context, c *registry.Client, o agentListOptions, raw *[]byte) error {
				limit, page := listPageParams(o)
				params := registry.SessionListParams{
					Limit:           limit,
					Page:            page,
					IncludeArchived: optIncludeArchived(o),
				}
				if agentID := strings.TrimSpace(o.agentID); agentID != "" {
					params.AgentID = param.String(agentID)
				}
				_, err := c.Sessions.List(ctx, params, option.WithRawJSON(raw))
				return err
			},
			get: func(ctx context.Context, c *registry.Client, id string, _ param.Opt[int64], raw *[]byte) error {
				_, err := c.Sessions.Get(ctx, id, option.WithRawJSON(raw))
				return err
			},
			remove: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Sessions.Delete(ctx, id, option.WithRawJSON(raw))
				return err
			},
			archive: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Sessions.Archive(ctx, id, option.WithRawJSON(raw))
				return err
			},
		}

	case resource.use == "memory-stores":
		return managedAgentOps{
			list: func(ctx context.Context, c *registry.Client, o agentListOptions, raw *[]byte) error {
				limit, page := listPageParams(o)
				_, err := c.MemoryStores.List(ctx, registry.MemoryStoreListParams{
					Limit:           limit,
					Page:            page,
					IncludeArchived: optIncludeArchived(o),
				}, option.WithRawJSON(raw))
				return err
			},
			get: func(ctx context.Context, c *registry.Client, id string, _ param.Opt[int64], raw *[]byte) error {
				_, err := c.MemoryStores.Get(ctx, id, option.WithRawJSON(raw))
				return err
			},
			remove: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.MemoryStores.Delete(ctx, id, option.WithRawJSON(raw))
				return err
			},
			archive: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.MemoryStores.Archive(ctx, id, option.WithRawJSON(raw))
				return err
			},
		}

	case resource.use == "vaults":
		return managedAgentOps{
			list: func(ctx context.Context, c *registry.Client, o agentListOptions, raw *[]byte) error {
				limit, page := listPageParams(o)
				_, err := c.Vaults.List(ctx, registry.VaultListParams{
					Limit:           limit,
					Page:            page,
					IncludeArchived: optIncludeArchived(o),
				}, option.WithRawJSON(raw))
				return err
			},
			get: func(ctx context.Context, c *registry.Client, id string, _ param.Opt[int64], raw *[]byte) error {
				_, err := c.Vaults.Get(ctx, id, option.WithRawJSON(raw))
				return err
			},
			remove: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Vaults.Delete(ctx, id, option.WithRawJSON(raw))
				return err
			},
			archive: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Vaults.Archive(ctx, id, option.WithRawJSON(raw))
				return err
			},
		}

	case resource.use == "environments":
		return managedAgentOps{
			list: func(ctx context.Context, c *registry.Client, o agentListOptions, raw *[]byte) error {
				limit, page := listPageParams(o)
				_, err := c.Environments.List(ctx, registry.EnvironmentListParams{
					Limit:           limit,
					Page:            page,
					IncludeArchived: optIncludeArchived(o),
				}, option.WithRawJSON(raw))
				return err
			},
			get: func(ctx context.Context, c *registry.Client, id string, _ param.Opt[int64], raw *[]byte) error {
				_, err := c.Environments.Get(ctx, id, option.WithRawJSON(raw))
				return err
			},
			remove: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Environments.Delete(ctx, id, option.WithRawJSON(raw))
				return err
			},
			archive: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Environments.Archive(ctx, id, option.WithRawJSON(raw))
				return err
			},
		}

	case resource.file:
		return managedAgentOps{
			list: func(ctx context.Context, c *registry.Client, o agentListOptions, raw *[]byte) error {
				limit, afterID, beforeID := listIDCursorParams(o)
				_, err := c.Files.List(ctx, registry.FileListParams{
					Limit:    limit,
					AfterID:  afterID,
					BeforeID: beforeID,
				}, option.WithRawJSON(raw))
				return err
			},
			get: func(ctx context.Context, c *registry.Client, id string, _ param.Opt[int64], raw *[]byte) error {
				_, err := c.Files.Get(ctx, id, option.WithRawJSON(raw))
				return err
			},
			remove: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Files.Delete(ctx, id, option.WithRawJSON(raw))
				return err
			},
		}

	case resource.skill:
		return managedAgentOps{
			list: func(ctx context.Context, c *registry.Client, o agentListOptions, raw *[]byte) error {
				limit, page := listPageParams(o)
				_, err := c.Skills.List(ctx, registry.SkillListParams{Limit: limit, Page: page},
					option.WithRawJSON(raw))
				return err
			},
			get: func(ctx context.Context, c *registry.Client, id string, _ param.Opt[int64], raw *[]byte) error {
				_, err := c.Skills.Get(ctx, id, option.WithRawJSON(raw))
				return err
			},
			remove: func(ctx context.Context, c *registry.Client, id string, raw *[]byte) error {
				_, err := c.Skills.Delete(ctx, id, option.WithRawJSON(raw))
				return err
			},
		}
	}

	return managedAgentOps{}
}

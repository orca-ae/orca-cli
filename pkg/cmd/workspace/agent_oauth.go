// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/orca-ae/orca-cli/pkg/mcpoauth"
	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

type authorizeMCPOAuth func(context.Context, mcpoauth.Options, io.Writer) (map[string]any, error)

func (o *agentOptions) newVaultCredentialCreateCommand() *cobra.Command {
	return o.newVaultCredentialCreateCommandWithOAuth(mcpoauth.Authorize)
}

func (o *agentOptions) newVaultCredentialCreateCommandWithOAuth(authorize authorizeMCPOAuth) *cobra.Command {
	opts := vaultCredentialOptions{output: "text"}
	oauth := mcpoauth.Options{CallbackAddress: "127.0.0.1:0", Timeout: 5 * time.Minute}
	cmd := &cobra.Command{
		Use:   "create --vault <vault-id> (--auth-json <json> | --mcp-server-url <url>)",
		Short: "Create a vault credential, optionally completing MCP OAuth in the browser",
		Long: "Create a vault credential from auth JSON, or authorize an MCP server using OAuth " +
			"discovery, PKCE, public clients or dynamically registered Basic clients, and a loopback browser callback. " +
			"OAuth tokens are sent directly to the vault, not saved locally. " +
			"For SSH, use --no-browser with a forwarded --callback-address port.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateWorkspaceOutput(opts.output); err != nil {
				return err
			}
			useOAuth := cmd.Flags().Changed("mcp-server-url")
			if useOAuth {
				if err := mcpoauth.ValidateOptions(oauth); err != nil {
					return err
				}
			} else {
				for _, name := range []string{"oauth-issuer", "oauth-client-id", "oauth-scope", "oauth-timeout", "callback-address", "no-browser", "no-refresh", "allow-http"} {
					if cmd.Flags().Changed(name) {
						return fmt.Errorf("--%s requires --mcp-server-url", name)
					}
				}
				if strings.TrimSpace(opts.authJSON) == "" {
					return fmt.Errorf("--auth-json must be a non-empty JSON object")
				}
			}
			// Finish local validation and registry client setup before starting a browser flow.
			payload, err := buildVaultCredentialPayload(cmd, opts, false)
			if err != nil {
				return err
			}
			if useOAuth {
				if err := validateOAuthCredentialPayload(payload); err != nil {
					return err
				}
			}
			path, err := buildVaultCredentialCollectionPath(opts)
			if err != nil {
				return err
			}
			client, err := newWorkspaceManagedAgentsClient()
			if err != nil {
				return err
			}
			if useOAuth {
				auth, err := authorize(cmd.Context(), oauth, o.ioStreams.ErrOut)
				if err != nil {
					return fmt.Errorf("MCP OAuth authorization failed: %w", err)
				}
				payload["auth"] = auth
			}
			result, err := client.Create(cmd.Context(), path, payload)
			if err != nil {
				if useOAuth {
					// Registry error bodies may echo the submitted token-bearing payload.
					var apiErr *registry.APIError
					if errors.As(err, &apiErr) {
						return fmt.Errorf("OAuth completed but vault credential registration failed (HTTP %d); check the vault before retrying; tokens were not saved locally", apiErr.StatusCode)
					}
					return fmt.Errorf("OAuth completed but vault credential registration failed; check registry connectivity and the vault before retrying; tokens were not saved locally")
				}
				return fmt.Errorf("failed to create vault credential: %w", err)
			}
			if useOAuth {
				result = redactOAuthCredentialResult(result, payload["auth"].(map[string]any))
			}
			return renderJSONOrText(o.ioStreams.Out, opts.output, result)
		},
	}
	addVaultCredentialParentFlag(cmd, &opts)
	addVaultCredentialPayloadFlags(cmd, &opts, false)
	cmd.Flags().StringVar(&oauth.ServerURL, "mcp-server-url", "", "MCP HTTP endpoint to authorize and register in the vault")
	cmd.Flags().StringVar(&oauth.Issuer, "oauth-issuer", "", "Select an advertised authorization server or pin the expected issuer of a single OAuth proxy")
	cmd.Flags().StringVar(&oauth.ClientID, "oauth-client-id", "", "Pre-registered public client ID (default: dynamic client registration)")
	cmd.Flags().StringArrayVar(&oauth.Scopes, "oauth-scope", nil, "OAuth scope override; repeat or separate scopes with spaces")
	cmd.Flags().DurationVar(&oauth.Timeout, "oauth-timeout", oauth.Timeout, "Timeout for the complete OAuth flow")
	cmd.Flags().StringVar(&oauth.CallbackAddress, "callback-address", oauth.CallbackAddress, "OAuth callback listener at 127.0.0.1:<port>; 0 selects a free port")
	cmd.Flags().BoolVar(&oauth.NoBrowser, "no-browser", false, "Print the authorization URL without opening a browser")
	cmd.Flags().BoolVar(&oauth.NoRefresh, "no-refresh", false, "Do not request offline access or a refresh-token grant")
	cmd.Flags().BoolVar(&oauth.AllowHTTP, "allow-http", false, "Allow HTTP OAuth endpoints on numeric loopback addresses for local testing only")
	cmd.MarkFlagsOneRequired("auth-json", "mcp-server-url")
	cmd.MarkFlagsMutuallyExclusive("auth-json", "mcp-server-url")
	return cmd
}

// Mirror registry limits before spending a one-time browser authorization. Zod
// measures string lengths in UTF-16 code units, not Go bytes or Unicode runes.
func validateOAuthCredentialPayload(payload map[string]any) error {
	if name, ok := payload["display_name"].(string); ok && len(utf16.Encode([]rune(name))) > 255 {
		return fmt.Errorf("--display-name must not exceed 255 characters")
	}
	metadata, _ := payload["metadata"].(map[string]string)
	if len(metadata) > 16 {
		return fmt.Errorf("--metadata must contain at most 16 pairs")
	}
	for key, value := range metadata {
		if len(utf16.Encode([]rune(key))) > 64 || len(utf16.Encode([]rune(value))) > 512 {
			return fmt.Errorf("--metadata keys must not exceed 64 characters and values must not exceed 512 characters")
		}
	}
	return nil
}

// Only emit documented credential fields. Defense in depth: redact known token
// values too, including token-bearing JSON embedded inside metadata strings.
func redactOAuthCredentialResult(value any, auth map[string]any) any {
	var replacements []string
	addSecret := func(value any) {
		if secret, ok := value.(string); ok && secret != "" {
			replacements = append(replacements, secret, "[REDACTED]")
			encoded, _ := json.Marshal(secret)
			replacements = append(replacements, string(encoded[1:len(encoded)-1]), "[REDACTED]")
		}
	}
	addSecret(auth["access_token"])
	if refresh, ok := auth["refresh"].(map[string]any); ok {
		addSecret(refresh["refresh_token"])
		if method, ok := refresh["token_endpoint_auth"].(map[string]any); ok {
			addSecret(method["client_secret"])
		}
	}
	fields, _ := value.(map[string]any)
	result := map[string]any{}
	for _, key := range []string{"id", "type", "vault_id", "display_name", "auth", "metadata", "archived_at", "created_at", "updated_at"} {
		if item, ok := fields[key]; ok {
			result[key] = item
		}
	}
	return redactOAuthValue(result, strings.NewReplacer(replacements...))
}

func redactOAuthValue(value any, secrets *strings.Replacer) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, item := range v {
			switch strings.ToLower(key) {
			case "auth", "access_token", "refresh_token", "client_secret":
				result[key] = "[REDACTED]"
			default:
				result[secrets.Replace(key)] = redactOAuthValue(item, secrets)
			}
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = redactOAuthValue(item, secrets)
		}
		return result
	case string:
		return secrets.Replace(v)
	default:
		return value
	}
}

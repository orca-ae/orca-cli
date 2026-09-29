# OpenAPI command gap inventory

This document records the remaining operation-level gap between the `ork` commands and the
deployment OpenAPI contract.

## Source of truth and counting rules

The audit compares the command tree with the engine's API contract,
[`services/registry-service-ts/openapi/managed-agents.yaml`](https://github.com/orca-ae/orca-agent-engine/blob/main/services/registry-service-ts/openapi/managed-agents.yaml),
and with the OpenAPI descriptions of the policy, pricing and hosted extension groups.

Operations that a deployment deliberately doesn't serve are excluded before comparing the core
surface. Consequently:

- removed `POST /v1/git-creds` is not part of the deployment surface;
- unsupported `DELETE /v1/agents/{id}` is not counted as a command gap;
- query parameters that a deployment doesn't accept are not exposed by the CLI;
- deployment-only `provider` query extensions are intentionally unsupported by the CLI.

Commands are matched to operations by HTTP method and path. A command that makes several requests
can cover several operations; several commands can also map to one operation. Optional request
headers and full request/response schema equivalence are outside the operation-counting rule.

| Surface | Supported OpenAPI operations | Covered by CLI | Missing |
|---|---:|---:|---:|
| Managed Agents core | 81 | 81 | 0 |
| Hosted extension group | 105 | 104 | 1 |
| Policy extension | 8 | 8 | 0 |
| Pricing extension | 3 | 3 | 0 |
| Total | 197 | 196 | 1 |

## Missing Managed Agents core operations

None. The CLI covers every core operation that a deployment serves.

## Missing hosted extension operations

All paths below are relative to `/apis/cloud.sn.io/v1`.

### Kafka Connect

The OpenAPI operation remains intentionally disabled because the hosted Kafka Connect runtime
doesn't expose the route.

| Operation ID | Method | Path |
|---|---|---|
| `validateConfigs` | `PUT` | `/connectors/kafka/connector-plugins/{pluginName}/config/validate` |

`getApiResources` is covered by `ork api-resources`.

## Policy and pricing extension operations

The CLI covers every operation in the policy and pricing extension contract:

- `guardrails create`, `list`, `get`, `update`, `archive`, and `delete` cover the guardrail
  lifecycle; `guardrails list-types` exposes the builtin catalog;
- `model-prices list` and `get` expose the effective pricing catalog;
- `api-resources --group policy.runorca.ai` and
  `api-resources --group pricing.runorca.ai` cover both group discovery endpoints.

These commands use the same capability rule as the SDK: they first require the corresponding
group in authenticated `GET /apis`, using the existing per-runtime discovery cache.

## Parameter and command-contract alignment

The following previously missing parameters are now exposed and forwarded:

- memory entry list: `depth`, `path_prefix`, and `view`;
- memory version list/get: `api_key_id`, `operation`, `created_at[gte]`, `created_at[lte]`, and
  `view`;
- session event list: all four `created_at` bounds, repeated `types`, and `subpath`;
- session event stream: `from_cursor`, `subpath`, and repeated `event_deltas`;
- session thread stream: `from_cursor` and repeated `event_deltas`;
- vault credential list: `include_archived`;
- environment create/update: `scope`.

The following existing command mismatches are also corrected:

- session file listing uses `after_id` and `before_id`, not `page`;
- agent version listing no longer sends unsupported archive/date filters;
- session thread event listing no longer exposes unsupported `order`;
- file upload sets the MIME type on the multipart file part without sending an undocumented
  `content_type` form field;
- memory `view`, event `order`, event delta, environment `scope`, and memory version `operation`
  values are checked against their OpenAPI enums.

A deployment deliberately doesn't accept some upstream filters on agents, sessions, files, memory
stores, memory versions, and skills. Tests assert that those parameters and every deployment-only
`provider` parameter remain absent from the CLI.

## Removed legacy surface

The hosted extension group's OpenAPI no longer contains Agent Function operations. The legacy
`agent-functions` command group and its client and types have therefore been removed and are not
included in the operation totals above.

## Follow-up

Expose Kafka connector config validation only after the runtime implements the documented route.

# Onboarding Scripts

Go utilities for safely automating new-employee access provisioning across Microsoft Entra ID, Azure RBAC, Bitbucket Data Center, and Microsoft Teams.

The scripts are intentionally dry-run by default. They provide a repeatable, auditable alternative to manual access requests while keeping credentials out of source code and command-line arguments.

## Included tools

| File | Purpose |
| --- | --- |
| `onboard-azure.go` | Adds an existing Entra ID user to a group, grants an Azure RBAC role at an explicit scope, and writes an owner-only local audit record. |
| `teams-bitbucket-onboard.go` | Adds an existing Bitbucket Data Center user to a group and can send a completion notification to a Microsoft Teams webhook. |

## Prerequisites

- Go 1.20 or later
- Azure CLI, authenticated with `az login`, for Azure/Entra operations
- An operator authorized to manage the selected Entra group and Azure role assignments
- Bitbucket Data Center/Server personal access token for Bitbucket operations
- Optional Microsoft Teams incoming workflow/webhook URL for status notifications

## Azure and Entra onboarding

Preview the proposed change first:

```bash
go run onboard-azure.go \
  --email user@company.com \
  --entra-group Engineering \
  --role Reader \
  --scope /subscriptions/<subscription-id>/resourceGroups/<resource-group>
```

Apply the change after review:

```bash
go run onboard-azure.go \
  --email user@company.com \
  --entra-group Engineering \
  --role Reader \
  --scope /subscriptions/<subscription-id>/resourceGroups/<resource-group> \
  --apply
```

Use the narrowest possible Azure scope. A resource-group scope is generally safer than granting access across an entire subscription.

## Bitbucket Data Center and Teams notification

Preview:

```bash
go run teams-bitbucket-onboard.go \
  --bitbucket-url https://bitbucket.company.com \
  --bitbucket-group developers \
  --bitbucket-user jdoe \
  --teams-webhook https://<teams-workflow-webhook>
```

Apply:

```bash
export BITBUCKET_TOKEN="<personal-access-token>"

go run teams-bitbucket-onboard.go \
  --bitbucket-url https://bitbucket.company.com \
  --bitbucket-group developers \
  --bitbucket-user jdoe \
  --teams-webhook https://<teams-workflow-webhook> \
  --apply
```

This integration is for Bitbucket Data Center/Server. Bitbucket Cloud has a different administration and provisioning model; use Atlassian Administration or the organization’s approved provisioning integration instead.

## Security notes

- Do not commit access tokens, Teams webhook URLs, or generated audit records.
- Store production secrets in Azure Key Vault and inject them at runtime.
- Use a managed identity when these scripts become Azure Functions or Container App workers.
- Put a Teams bot or authenticated workflow in front of the workers; do not expose the scripts directly on the public internet.
- Preserve approvals and audit records for every provisioning request.

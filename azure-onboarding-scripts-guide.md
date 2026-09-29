# Azure onboarding scripts: detailed implementation guide

This guide documents two companion command-line programs in this directory:

1. `onboard-azure.go` — Azure/Entra onboarding: group membership and Azure RBAC.
2. `teams-bitbucket-onboard.go` — Bitbucket Data Center group assignment and optional Teams status notification.

Both start in dry-run mode. A change happens only when `--apply` is passed.

## Architecture and boundaries

`onboard-azure.go` owns Microsoft Entra ID membership and Azure role assignments. It authenticates through the locally logged-in Azure CLI, so Azure credentials never appear in source code, arguments, or environment variables.

`teams-bitbucket-onboard.go` owns Bitbucket Data Center membership and a final Teams webhook message. It obtains the Bitbucket personal access token only from `BITBUCKET_TOKEN`. Microsoft Teams is the notification/request surface; an incoming webhook cannot itself validate an interactive Teams command. A production command interface should be a Teams bot hosted on Azure Functions or Container Apps, which validates Teams identity before calling this worker.

The Bitbucket program targets Data Center/Server only. Bitbucket Cloud uses Atlassian Administration and has a different provisioning model.

## Safe operating sequence

1. Sign in with `az login`; select the intended subscription.
2. Use a narrow resource-group scope instead of a subscription scope unless broader access is required.
3. Run dry-run first.
4. Confirm the user, group, role, scope, and Bitbucket username with the request owner.
5. Run again with `--apply`.
6. Keep the local Azure audit JSON in the ticket/change record. Review Bitbucket and Teams audit records centrally.

## Azure script, source-order explanation

### Header, package, and imports (lines 1-21)

Lines 1-9 are documentation and safe invocation examples. Line 10 declares an executable Go program. Lines 12-21 import the standard-library packages used for JSON records, flags, console output, process execution, paths, text cleanup, and UTC timestamps. No Azure SDK or third-party dependency is used.

### Azure response and audit models (lines 23-38)

Lines 23-27 define `azureUser`, the small subset of `az ad user show` JSON that the program needs: immutable object ID, display name, and UPN. The JSON tags on lines 24-26 map Azure CLI response keys to Go fields.

Lines 29-38 define `audit`, the local evidence record. Lines 30-35 preserve who was targeted and where; lines 36-37 record whether this execution actually changed group membership or RBAC. False means the assignment already existed, not necessarily that the user lacks access inherited from a parent scope or group.

### Flags and input validation (lines 40-56)

Line 41 declares required `--email`. Line 42 declares required `--entra-group`, accepting a group name or object ID. Line 43 defaults `--role` to `Reader`, deliberately low privilege. Line 44 holds the Azure scope. Line 45 creates the safety switch, `--apply`; false is the default. Line 46 parses arguments.

Lines 48-50 reject missing identity inputs. Lines 51-53 do a lightweight UPN sanity check; this is not full email validation. Lines 54-56 prohibit applied RBAC changes without an explicit scope, preventing accidental assignments at an unspecified or broad target.

### Plan output and dry-run gate (lines 58-73)

Lines 58-62 print the requested subject and, when supplied, RBAC target. Lines 64-73 implement the most important safety property: without `--apply`, the program prints its plan and returns before calling Azure CLI or writing anything. Lines 65-71 explain every intended operation; line 72 terminates cleanly.

### Azure CLI identity lookup (lines 75-81)

Line 75 verifies the CLI is installed and authenticated. Line 76 calls `az ad user show`, requesting only the fields modeled by `azureUser`; `-o json` makes parsing reliable. Line 77 declares the destination object. Lines 78-80 decode the response and reject malformed or empty-object-ID results. Line 81 prints a human confirmation before permission changes.

### Entra group operation (lines 83-91)

Line 83 initializes the audit record before actions. Line 84 asks Entra whether the object ID is already a member of the requested group and reduces the answer to `true` or `false`. Lines 85-88 add the member only when absent, mark the audit result, and report success. Lines 89-90 explicitly report the no-op case. This is idempotence: rerunning a successful request does not create duplicate membership.

### Azure RBAC operation (lines 93-100)

Line 93 lists matching role assignments for this principal, exact scope, and role. Lines 94-97 create the assignment only when the response is empty. `--assignee-object-id` avoids an ambiguous display-name lookup; `--assignee-principal-type User` tells Azure the object type. Lines 98-99 report an existing direct assignment. The check does not attempt to calculate effective access inherited from groups or parent scopes.

### Audit write and helper functions (lines 102-137)

Line 102 writes the result after Azure operations complete. Lines 105-110 define `ensureAzureCLI`: line 106 finds `az` on PATH, lines 107-108 provide a useful failure, and line 109 verifies a current Azure account context.

Lines 112-119 define `run`, the shared process wrapper. Line 113 constructs a command without using a shell, avoiding shell-injection from input strings. Line 114 captures standard output and standard error. Lines 115-117 stop on a non-zero exit and show Azure's response. Line 118 returns successful output.

Lines 121-132 define `writeAudit`. Line 122 produces readable JSON. Line 126 uses a UTC timestamp in the filename. Line 128 writes with mode `0600`, so only the local account can read the audit record. Line 131 reports its path. Lines 134-137 define `fail`, which prints to stderr and exits with status 1.

### Azure commands

Dry-run:

`go run onboard-azure.go --email user@company.com --entra-group Engineering --role Reader --scope /subscriptions/<subscription-id>/resourceGroups/<resource-group>`

Apply:

`go run onboard-azure.go --email user@company.com --entra-group Engineering --role Reader --scope /subscriptions/<subscription-id>/resourceGroups/<resource-group> --apply`

The operator needs permission to update that Entra group and permission to create role assignments at the exact scope, generally Owner or User Access Administrator. Use a managed identity, not a human CLI identity, once this runs in Azure.

## Teams and Bitbucket script, source-order explanation

### Header, package, and imports (lines 1-21)

Lines 1-7 establish purpose and the Bitbucket Data Center limitation. Line 8 makes an executable program. Lines 10-21 import only standard packages: byte buffers and JSON for HTTP payloads; flags; HTTP/URL handling; environment access; response reading; string cleanup; and request timeouts.

### Flags and URL validation (lines 23-40)

Line 24 requires `--bitbucket-url`. Line 25 requires an existing Bitbucket group. Line 26 requires a Bitbucket username, which is often different from an email address. Line 27 accepts an optional Teams webhook. Line 28 makes dry-run default; line 29 parses flags.

Lines 31-33 reject omitted mandatory Bitbucket inputs. Lines 34-37 parse the base URL and accept only an HTTPS URL with a host. Lines 38-40 apply the same HTTPS requirement to an optional Teams webhook. This blocks accidental plain HTTP but cannot establish that a URL is the intended organization endpoint; protect CI/CD configuration and review inputs.

### Dry-run gate (lines 42-51)

Line 42 prints the request using the parsed host. Lines 43-51 show each planned action and return unless `--apply` is set. No token is read and no HTTP request is made in dry-run mode.

### Token, payload, and Bitbucket request (lines 53-75)

Line 53 reads `BITBUCKET_TOKEN` at runtime. Lines 54-56 reject an absent token without logging its value. Line 57 safely joins the administrator-provided base URL with the Data Center group-management endpoint. Line 58 creates exactly this JSON: `{ "group": "...", "users": ["..."] }`.

Lines 59-62 build the POST request. Lines 63-65 provide bearer authorization and JSON content negotiation. Line 66 sets a 20-second client timeout so unavailable services do not hang indefinitely. Lines 67-71 issue the request and ensure the response body closes. Line 72 reads at most 4 KiB of an error body; lines 73-75 treat every non-2xx response as a failure while preventing a huge diagnostic response from consuming memory.

### Completion and Teams notification (lines 77-83)

Line 77 builds a status message after Bitbucket returns success. Line 78 prints it locally. Lines 79-81 post the same status only when a webhook was supplied, then confirm it. The current program does not send a failure notification because `fail` exits immediately; production orchestration should catch errors and notify a support channel without exposing tokens or sensitive response bodies.

### Teams helper, HTTPS helper, and failure helper (lines 85-111)

Lines 85-101 define `postTeams`. Line 86 wraps the message as Teams webhook JSON. Lines 87-91 build the HTTP POST and set content type. Lines 92-100 send, close, and validate the response with the same bounded error-body behavior as Bitbucket.

Lines 103-106 define `isHTTPS`; it parses a URL and returns true only for a host-bearing HTTPS URL. Lines 108-111 are the same stderr/non-zero error mechanism as the Azure script.

### Bitbucket Data Center and Teams commands

Preview:

`go run teams-bitbucket-onboard.go --bitbucket-url https://bitbucket.company.com --bitbucket-group developers --bitbucket-user jdoe --teams-webhook https://<teams-workflow-webhook>`

Apply:

`BITBUCKET_TOKEN='<personal-access-token>' go run teams-bitbucket-onboard.go --bitbucket-url https://bitbucket.company.com --bitbucket-group developers --bitbucket-user jdoe --teams-webhook https://<teams-workflow-webhook> --apply`

The token must have administrative permission to alter Bitbucket Data Center group membership. Keep it in Azure Key Vault when the program is hosted; inject it into the worker at runtime and never commit it.

## Production hardening checklist

1. Replace human `az login` with a dedicated user-assigned managed identity.
2. Split Azure, Bitbucket, and notification actions into independent retryable workflow activities.
3. Authenticate a Teams bot request and authorize the requester before enqueueing work; never expose these CLI programs directly to the public internet.
4. Store Bitbucket tokens and Teams webhook URLs in Key Vault. Rotate them and restrict the worker identity to read only required secrets.
5. Put requests in Service Bus, run workers with retry/backoff, and use a dead-letter queue for failures.
6. Record request ID, requester, approval, target account, target roles, timestamps, and outcomes in a central immutable audit store.
7. Add an offboarding flow that revokes high-risk access first, is idempotent, and alerts on partial failure.
8. Test with a non-production Entra tenant, subscription/resource group, Bitbucket group, and Teams channel before granting production permissions.

## Full source reference

The line-by-line narrative above is paired with the full source files kept beside this guide. Use `nl -ba onboard-azure.go` and `nl -ba teams-bitbucket-onboard.go` to view the exact numbered source on the command line.

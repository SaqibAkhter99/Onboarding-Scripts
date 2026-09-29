// onboard-azure adds an existing Entra ID user to an Entra group and assigns
// an Azure RBAC role. It deliberately delegates authentication to Azure CLI;
// no credentials are read from flags, files, or environment variables.
//
// Examples:
//
//	go run onboard-azure.go --email user@company.com --entra-group Engineering --dry-run
//	go run onboard-azure.go --email user@company.com --entra-group Engineering \
//	  --role Reader --scope /subscriptions/<subscription-id> --apply
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type azureUser struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	UserPrincipalName string `json:"userPrincipalName"`
}

type audit struct {
	Timestamp  string `json:"timestamp"`
	Email      string `json:"email"`
	Group      string `json:"entraGroup"`
	Role       string `json:"role"`
	Scope      string `json:"scope"`
	UserID     string `json:"userId"`
	GroupAdded bool   `json:"groupAdded"`
	RoleAdded  bool   `json:"roleAdded"`
}

func main() {
	email := flag.String("email", "", "existing user's email / UPN (required)")
	group := flag.String("entra-group", "", "existing Entra ID security group name or object ID (required)")
	role := flag.String("role", "Reader", "Azure RBAC role to assign")
	scope := flag.String("scope", "", "RBAC scope, e.g. /subscriptions/<id> (required with --apply)")
	apply := flag.Bool("apply", false, "perform changes; otherwise print the planned actions")
	flag.Parse()

	if *email == "" || *group == "" {
		fail("--email and --entra-group are required")
	}
	if !strings.Contains(*email, "@") {
		fail("--email must be a valid email/UPN")
	}
	if *apply && *scope == "" {
		fail("--scope is required with --apply to prevent an accidental broad role assignment")
	}

	fmt.Printf("Onboarding request: %s -> group %q", *email, *group)
	if *scope != "" {
		fmt.Printf(", role %q at %s", *role, *scope)
	}
	fmt.Println()

	if !*apply {
		fmt.Println("DRY RUN: no Azure resources or identities will be changed.")
		fmt.Println("Would: resolve the user with Azure CLI")
		fmt.Printf("Would: add the user to Entra group %q if absent\n", *group)
		if *scope != "" {
			fmt.Printf("Would: assign %q at %s if absent\n", *role, *scope)
		}
		fmt.Println("Re-run with --apply and an explicit --scope after reviewing the plan.")
		return
	}

	ensureAzureCLI()
	userJSON := run("az", "ad", "user", "show", "--id", *email, "--query", "{id:id,displayName:displayName,userPrincipalName:userPrincipalName}", "-o", "json")
	var user azureUser
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil || user.ID == "" {
		fail("could not resolve Entra user %q: %v", *email, err)
	}
	fmt.Printf("Resolved: %s (%s)\n", user.DisplayName, user.UserPrincipalName)

	result := audit{Timestamp: time.Now().UTC().Format(time.RFC3339), Email: *email, Group: *group, Role: *role, Scope: *scope, UserID: user.ID}
	member := strings.TrimSpace(run("az", "ad", "group", "member", "check", "--group", *group, "--member-id", user.ID, "--query", "value", "-o", "tsv"))
	if member != "true" {
		run("az", "ad", "group", "member", "add", "--group", *group, "--member-id", user.ID)
		result.GroupAdded = true
		fmt.Println("Added to Entra group.")
	} else {
		fmt.Println("Already in Entra group; skipped.")
	}

	assignments := strings.TrimSpace(run("az", "role", "assignment", "list", "--assignee", user.ID, "--scope", *scope, "--role", *role, "-o", "json"))
	if assignments == "" || assignments == "[]" {
		run("az", "role", "assignment", "create", "--assignee-object-id", user.ID, "--assignee-principal-type", "User", "--role", *role, "--scope", *scope, "-o", "none")
		result.RoleAdded = true
		fmt.Println("Assigned Azure RBAC role.")
	} else {
		fmt.Println("Azure RBAC role already exists at this scope; skipped.")
	}

	writeAudit(result)
}

func ensureAzureCLI() {
	if _, err := exec.LookPath("az"); err != nil {
		fail("Azure CLI (az) is required. Install it, run 'az login', then retry")
	}
	run("az", "account", "show", "-o", "none")
}

func run(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fail("command failed: %s %s\n%s", name, strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return string(output)
}

func writeAudit(entry audit) {
	b, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		fail("create audit record: %v", err)
	}
	name := "onboarding-audit-" + time.Now().UTC().Format("20060102T150405Z") + ".json"
	path := filepath.Join(".", name)
	if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
		fail("write audit record: %v", err)
	}
	fmt.Printf("Audit record written to %s\n", path)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

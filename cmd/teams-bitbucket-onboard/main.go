// teams-bitbucket-onboard is the Teams/Bitbucket counterpart to
// cmd/onboard-azure. It adds an existing Bitbucket Data Center user to a group
// and optionally posts the result to a Microsoft Teams incoming-workflow/webhook URL.
//
// It targets Bitbucket Data Center/Server. Bitbucket Cloud user provisioning is
// organization-specific and should be performed through Atlassian Administration
// (or its configured SCIM/identity process), not this Data Center endpoint.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	baseURL := flag.String("bitbucket-url", "", "Bitbucket Data Center base URL, e.g. https://bitbucket.example.com (required)")
	group := flag.String("bitbucket-group", "", "existing Bitbucket group (required)")
	user := flag.String("bitbucket-user", "", "Bitbucket username, usually not an email address (required)")
	apply := flag.Bool("apply", false, "perform the Bitbucket group change; otherwise print the plan")
	flag.Parse()

	if *baseURL == "" || *group == "" || *user == "" {
		fail("--bitbucket-url, --bitbucket-group, and --bitbucket-user are required")
	}
	parsed, err := url.Parse(*baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		fail("--bitbucket-url must be an HTTPS base URL")
	}
	teamsWebhook := os.Getenv("TEAMS_WEBHOOK_URL")
	if teamsWebhook != "" && !isHTTPS(teamsWebhook) {
		fail("TEAMS_WEBHOOK_URL must be an HTTPS URL")
	}

	fmt.Printf("Request: add Bitbucket user %q to group %q at %s\n", *user, *group, parsed.Host)
	if !*apply {
		fmt.Println("DRY RUN: no Bitbucket changes or Teams notifications will be made.")
		fmt.Println("Would: POST /rest/api/1.0/admin/groups/add-users to Bitbucket Data Center")
		if teamsWebhook != "" {
			fmt.Println("Would: post completion status to Teams.")
		}
		fmt.Println("Re-run with --apply after reviewing the request.")
		fmt.Println("RESULT: dry run")
		return
	}

	token := os.Getenv("BITBUCKET_TOKEN")
	if token == "" {
		fail("BITBUCKET_TOKEN must contain a Bitbucket Data Center personal access token")
	}
	endpoint := strings.TrimRight(*baseURL, "/") + "/rest/api/1.0/admin/groups/add-users"
	body, _ := json.Marshal(map[string]any{"group": *group, "users": []string{*user}})
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		fail("create Bitbucket request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	bitbucketClient := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := bitbucketClient.Do(request)
	if err != nil {
		fail("call Bitbucket: %v", err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		fail("Bitbucket returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}

	message := fmt.Sprintf("Onboarding complete: Bitbucket user %s added to group %s.", *user, *group)
	fmt.Println(message)
	if teamsWebhook != "" {
		if err := postTeams(&http.Client{Timeout: 20 * time.Second}, teamsWebhook, message); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Bitbucket change succeeded, but Teams notification failed: %v\n", err)
			fmt.Println("RESULT: applied; notification failed")
			os.Exit(2)
		}
		fmt.Println("Teams notification sent.")
	}
	fmt.Println("RESULT: applied")
}

func postTeams(client *http.Client, webhook, message string) error {
	payload, _ := json.Marshal(map[string]string{"text": message})
	request, err := http.NewRequest(http.MethodPost, webhook, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("post request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("teams returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func isHTTPS(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

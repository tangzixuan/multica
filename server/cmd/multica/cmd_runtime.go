package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var runtimeCmd = &cobra.Command{
	Use:   "runtime",
	Short: "Work with agent runtimes",
}

var runtimeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List runtimes in the workspace",
	RunE:  runRuntimeList,
}

var runtimeUsageCmd = &cobra.Command{
	Use:   "usage <runtime-id>",
	Short: "Get token usage for a runtime",
	Args:  exactArgs(1),
	RunE:  runRuntimeUsage,
}

var runtimeActivityCmd = &cobra.Command{
	Use:   "activity <runtime-id>",
	Short: "Get hourly task activity for a runtime",
	Args:  exactArgs(1),
	RunE:  runRuntimeActivity,
}

var runtimeUpdateCmd = &cobra.Command{
	Use:   "update <runtime-id>",
	Short: "Initiate a CLI update on a runtime",
	Args:  exactArgs(1),
	RunE:  runRuntimeUpdate,
}

var runtimeLocalSkillsCmd = &cobra.Command{
	Use:   "local-skills",
	Short: "List and import runtime-local skills",
}

var runtimeLocalSkillsListCmd = &cobra.Command{
	Use:   "list <runtime-id>",
	Short: "List local skills exposed by a runtime",
	Args:  exactArgs(1),
	RunE:  runRuntimeLocalSkillsList,
}

var runtimeLocalSkillsImportCmd = &cobra.Command{
	Use:   "import <runtime-id> <skill-key>",
	Short: "Import a runtime-local skill into the workspace",
	Args:  exactArgs(2),
	RunE:  runRuntimeLocalSkillsImport,
}

func init() {
	runtimeCmd.AddCommand(runtimeListCmd)
	runtimeCmd.AddCommand(runtimeUsageCmd)
	runtimeCmd.AddCommand(runtimeActivityCmd)
	runtimeCmd.AddCommand(runtimeUpdateCmd)
	runtimeCmd.AddCommand(runtimeLocalSkillsCmd)
	runtimeLocalSkillsCmd.AddCommand(runtimeLocalSkillsListCmd)
	runtimeLocalSkillsCmd.AddCommand(runtimeLocalSkillsImportCmd)

	// runtime list
	runtimeListCmd.Flags().String("output", "table", "Output format: table or json")

	// runtime usage
	runtimeUsageCmd.Flags().String("output", "table", "Output format: table or json")
	runtimeUsageCmd.Flags().Int("days", 90, "Number of days of usage data to retrieve (max 365)")

	// runtime activity
	runtimeActivityCmd.Flags().String("output", "table", "Output format: table or json")

	// runtime update
	runtimeUpdateCmd.Flags().String("target-version", "", "Target version to update to (required)")
	runtimeUpdateCmd.Flags().String("output", "json", "Output format: table or json")
	runtimeUpdateCmd.Flags().Bool("wait", false, "Wait for update to complete (poll until done)")

	// runtime local-skills
	runtimeLocalSkillsListCmd.Flags().String("output", "table", "Output format: table or json")
	runtimeLocalSkillsImportCmd.Flags().String("name", "", "Override imported skill name")
	runtimeLocalSkillsImportCmd.Flags().String("description", "", "Override imported skill description")
	runtimeLocalSkillsImportCmd.Flags().String("output", "json", "Output format: table or json")
}

// ---------------------------------------------------------------------------
// Runtime commands
// ---------------------------------------------------------------------------

func runRuntimeList(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var runtimes []map[string]any
	if err := client.GetJSON(ctx, "/api/runtimes", &runtimes); err != nil {
		return fmt.Errorf("list runtimes: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, runtimes)
	}

	headers := []string{"ID", "NAME", "MODE", "PROVIDER", "STATUS", "LAST_SEEN"}
	rows := make([][]string, 0, len(runtimes))
	for _, rt := range runtimes {
		rows = append(rows, []string{
			strVal(rt, "id"),
			strVal(rt, "name"),
			strVal(rt, "runtime_mode"),
			strVal(rt, "provider"),
			strVal(rt, "status"),
			strVal(rt, "last_seen_at"),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runRuntimeUsage(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	days, _ := cmd.Flags().GetInt("days")
	if days < 1 || days > 365 {
		return fmt.Errorf("--days must be between 1 and 365")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var usage []map[string]any
	path := fmt.Sprintf("/api/runtimes/%s/usage?days=%d", args[0], days)
	if err := client.GetJSON(ctx, path, &usage); err != nil {
		return fmt.Errorf("get runtime usage: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, usage)
	}

	headers := []string{"DATE", "PROVIDER", "MODEL", "INPUT_TOKENS", "OUTPUT_TOKENS", "CACHE_READ", "CACHE_WRITE"}
	rows := make([][]string, 0, len(usage))
	for _, u := range usage {
		rows = append(rows, []string{
			strVal(u, "date"),
			strVal(u, "provider"),
			strVal(u, "model"),
			strVal(u, "input_tokens"),
			strVal(u, "output_tokens"),
			strVal(u, "cache_read_tokens"),
			strVal(u, "cache_write_tokens"),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runRuntimeActivity(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var activity []map[string]any
	if err := client.GetJSON(ctx, "/api/runtimes/"+args[0]+"/activity", &activity); err != nil {
		return fmt.Errorf("get runtime activity: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, activity)
	}

	headers := []string{"HOUR", "COUNT"}
	rows := make([][]string, 0, len(activity))
	for _, a := range activity {
		rows = append(rows, []string{
			strVal(a, "hour"),
			strVal(a, "count"),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runRuntimeUpdate(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	targetVersion, _ := cmd.Flags().GetString("target-version")
	if targetVersion == "" {
		return fmt.Errorf("--target-version is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	body := map[string]any{
		"target_version": targetVersion,
	}

	var update map[string]any
	if err := client.PostJSON(ctx, "/api/runtimes/"+args[0]+"/update", body, &update); err != nil {
		return fmt.Errorf("initiate update: %w", err)
	}

	wait, _ := cmd.Flags().GetBool("wait")
	if !wait {
		output, _ := cmd.Flags().GetString("output")
		if output == "json" {
			return cli.PrintJSON(os.Stdout, update)
		}
		fmt.Printf("Update initiated: %s (status: %s)\n", strVal(update, "id"), strVal(update, "status"))
		return nil
	}

	// Poll until completed/failed/timeout.
	updateID := strVal(update, "id")
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for update (last status: %s)", strVal(update, "status"))
		case <-time.After(2 * time.Second):
		}

		if err := client.GetJSON(ctx, "/api/runtimes/"+args[0]+"/update/"+updateID, &update); err != nil {
			return fmt.Errorf("get update status: %w", err)
		}

		status := strVal(update, "status")
		if status == "completed" || status == "failed" || status == "timeout" {
			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				return cli.PrintJSON(os.Stdout, update)
			}
			if status == "completed" {
				fmt.Printf("Update completed: %s\n", strVal(update, "output"))
			} else {
				fmt.Printf("Update %s: %s\n", status, strVal(update, "error"))
			}
			return nil
		}
	}
}

const (
	runtimeLocalSkillsPollInterval  = 500 * time.Millisecond
	runtimeLocalSkillsListTimeout   = 30 * time.Second
	runtimeLocalSkillsImportTimeout = 4 * time.Minute
)

func runRuntimeLocalSkillsList(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	runtimeID := args[0]
	ctx, cancel := context.WithTimeout(context.Background(), runtimeLocalSkillsListTimeout)
	defer cancel()

	var req map[string]any
	if err := client.PostJSON(ctx, "/api/runtimes/"+runtimeID+"/local-skills", map[string]any{}, &req); err != nil {
		return fmt.Errorf("initiate local skill list: %w", err)
	}

	req, err = pollRuntimeLocalSkillRequest(ctx, client, "/api/runtimes/"+runtimeID+"/local-skills/"+strVal(req, "id"), req)
	if err != nil {
		return err
	}
	if strVal(req, "status") != "completed" {
		return fmt.Errorf("local skill list %s: %s", strVal(req, "status"), strVal(req, "error"))
	}
	if supported, ok := req["supported"].(bool); ok && !supported {
		return fmt.Errorf("runtime does not support local skills")
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, req)
	}

	skills, _ := req["skills"].([]any)
	headers := []string{"KEY", "NAME", "PROVIDER", "FILES", "SOURCE"}
	rows := make([][]string, 0, len(skills))
	for _, item := range skills {
		s, _ := item.(map[string]any)
		rows = append(rows, []string{
			strVal(s, "key"),
			strVal(s, "name"),
			strVal(s, "provider"),
			strVal(s, "file_count"),
			strVal(s, "source_path"),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runRuntimeLocalSkillsImport(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	runtimeID := args[0]
	body := map[string]any{"skill_key": args[1]}
	if cmd.Flags().Changed("name") {
		v, _ := cmd.Flags().GetString("name")
		body["name"] = v
	}
	if cmd.Flags().Changed("description") {
		v, _ := cmd.Flags().GetString("description")
		body["description"] = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtimeLocalSkillsImportTimeout)
	defer cancel()

	var req map[string]any
	if err := client.PostJSON(ctx, "/api/runtimes/"+runtimeID+"/local-skills/import", body, &req); err != nil {
		return fmt.Errorf("initiate local skill import: %w", err)
	}

	req, err = pollRuntimeLocalSkillRequest(ctx, client, "/api/runtimes/"+runtimeID+"/local-skills/import/"+strVal(req, "id"), req)
	if err != nil {
		return err
	}
	if strVal(req, "status") != "completed" {
		return fmt.Errorf("local skill import %s: %s", strVal(req, "status"), strVal(req, "error"))
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, req)
	}
	skill, _ := req["skill"].(map[string]any)
	fmt.Printf("Skill imported: %s (%s)\n", strVal(skill, "name"), strVal(skill, "id"))
	return nil
}

func pollRuntimeLocalSkillRequest(ctx context.Context, client *cli.APIClient, path string, req map[string]any) (map[string]any, error) {
	for {
		status := strVal(req, "status")
		if status == "completed" || status == "failed" || status == "timeout" {
			return req, nil
		}

		select {
		case <-ctx.Done():
			return req, fmt.Errorf("timed out waiting for runtime local skill request (last status: %s)", status)
		case <-time.After(runtimeLocalSkillsPollInterval):
		}

		var next map[string]any
		if err := client.GetJSON(ctx, path, &next); err != nil {
			return req, fmt.Errorf("get runtime local skill request: %w", err)
		}
		req = next
	}
}

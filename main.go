package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/simonkoeck/g2/pkg/semantic"
	"github.com/simonkoeck/g2/pkg/ui"
)

func main() {
	args := os.Args[1:]

	// If no args or not a merge command, passthrough to git
	if len(args) == 0 || args[0] != "merge" {
		passthrough("git", args...)
		return
	}

	// Smart merge mode
	smartMerge(args)
}

// passthrough replaces the current process with git
func passthrough(cmd string, args ...string) {
	gitPath, err := exec.LookPath(cmd)
	if err != nil {
		ui.Error(fmt.Sprintf("Could not find %s: %v", cmd, err))
		os.Exit(1)
	}

	// syscall.Exec replaces the current process entirely
	// This preserves stdin/stdout/stderr, colors, and interactivity
	execArgs := append([]string{cmd}, args...)
	if err := syscall.Exec(gitPath, execArgs, os.Environ()); err != nil {
		ui.Error(fmt.Sprintf("Failed to exec %s: %v", cmd, err))
		os.Exit(1)
	}
}

// smartMerge runs git merge with semantic conflict analysis
func smartMerge(args []string) {
	// Check if we're in a git repo
	if !isGitRepo() {
		ui.Error("Not a git repository")
		os.Exit(128)
	}

	// Parse g2-specific flags and filter them out for git
	config := semantic.DefaultMergeConfig()
	var gitArgs []string
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			config.DryRun = true
		case "--verbose", "-v":
			config.Verbose = true
		case "--no-backup":
			config.CreateBackup = false
		default:
			gitArgs = append(gitArgs, arg)
		}
	}

	// Display header
	ui.Header("G2 Smart Merge")

	// In dry-run mode, we still need to run git merge to detect conflicts
	// but we won't write any synthesized changes
	if config.DryRun {
		ui.Info("Dry-run mode: no files will be modified")
		fmt.Println()
	}

	// Run git merge
	ui.Step("Running git merge...")
	cmd := exec.Command("git", gitArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()

	if err == nil {
		// Merge succeeded without conflicts
		fmt.Println()
		ui.Success("Merge completed successfully!")
		return
	}

	// Check exit code
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		ui.Error(fmt.Sprintf("Merge failed: %v", err))
		os.Exit(1)
	}

	exitCode := exitErr.ExitCode()
	if exitCode != 1 {
		// Exit code other than 1 indicates a different error
		os.Exit(exitCode)
	}

	// Exit code 1 typically means conflicts
	fmt.Println()
	ui.Warning("Merge conflicts detected!")
	fmt.Println()

	// Get conflicting files
	ui.Step("Analyzing conflicts...")
	conflictingFiles, err := semantic.GetConflictingFiles()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get conflicting files: %v", err))
		os.Exit(1)
	}

	if len(conflictingFiles) == 0 {
		ui.Info("No conflicting files found (merge may have failed for another reason)")
		os.Exit(1)
	}

	// Analyze each file
	var allConflicts []ui.Conflict
	for _, file := range conflictingFiles {
		var analysis *semantic.ConflictAnalysis

		if semantic.IsSemanticFile(file) {
			analysis = semantic.AnalyzeConflict(file)
		} else {
			analysis = semantic.AnalyzeNonSemanticFile(file)
		}

		allConflicts = append(allConflicts, analysis.Conflicts...)
	}

	// Display conflict table
	fmt.Println()
	ui.ConflictTable(allConflicts)

	// Count conflicts needing resolution
	needsResolution := 0
	for _, c := range allConflicts {
		if c.Status == "Needs Resolution" {
			needsResolution++
		}
	}

	ui.Summary(needsResolution, len(allConflicts))

	// Synthesize files
	fmt.Println()
	if config.DryRun {
		ui.Step("Dry run - showing proposed changes...")
	} else {
		ui.Step("Synthesizing files...")
	}
	allAutoMerged := true
	filesWithMarkers := 0

	for _, file := range conflictingFiles {
		if semantic.IsSemanticFile(file) {
			synthesis := semantic.AnalyzeConflictForSynthesis(file)
			result := semantic.SynthesizeFile(synthesis, config)

			if result.Error != nil {
				ui.Warning(fmt.Sprintf("Failed to synthesize %s: %v", file, result.Error))
				allAutoMerged = false
			} else if !result.AllAutoMerged {
				allAutoMerged = false
				filesWithMarkers++
			}
		} else {
			allAutoMerged = false
		}
	}

	// Final status
	fmt.Println()
	if config.DryRun {
		ui.Info("Dry run complete - no files were modified")
		os.Exit(0)
	} else if allAutoMerged && needsResolution == 0 {
		ui.Success("All conflicts auto-merged and staged!")
		os.Exit(0)
	} else {
		if filesWithMarkers > 0 {
			ui.Info(fmt.Sprintf("%d file(s) have conflict markers - resolve manually", filesWithMarkers))
		}
		// Exit with error code to indicate conflicts remain
		os.Exit(1)
	}
}

// isGitRepo checks if the current directory is inside a git repository
func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Stderr = nil
	cmd.Stdout = nil
	return cmd.Run() == nil
}

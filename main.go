package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/simonkoeck/g2/pkg/exitcode"
	"github.com/simonkoeck/g2/pkg/git"
	"github.com/simonkoeck/g2/pkg/logging"
	"github.com/simonkoeck/g2/pkg/output"
	"github.com/simonkoeck/g2/pkg/semantic"
	"github.com/simonkoeck/g2/pkg/ui"
)

// Package-level git executor (replaceable for testing)
var gitExec git.Executor = git.NewDefaultExecutor()

func main() {
	args := os.Args[1:]

	// If no args or not a merge command, passthrough to git
	if len(args) == 0 || args[0] != "merge" {
		passthrough("git", args...)
		return
	}

	// Smart merge mode
	os.Exit(smartMerge(args))
}

// passthrough replaces the current process with git
func passthrough(cmd string, args ...string) {
	gitPath, err := exec.LookPath(cmd)
	if err != nil {
		ui.Error(fmt.Sprintf("Could not find %s: %v", cmd, err))
		os.Exit(exitcode.GitError)
	}

	// syscall.Exec replaces the current process entirely
	// This preserves stdin/stdout/stderr, colors, and interactivity
	execArgs := append([]string{cmd}, args...)
	if err := syscall.Exec(gitPath, execArgs, os.Environ()); err != nil {
		ui.Error(fmt.Sprintf("Failed to exec %s: %v", cmd, err))
		os.Exit(exitcode.GitError)
	}
}

// smartMerge runs git merge with semantic conflict analysis
// Returns the exit code to use
func smartMerge(args []string) int {
	ctx := context.Background()

	// Parse g2-specific flags and filter them out for git
	config := semantic.DefaultMergeConfig()
	var gitArgs []string
	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			config.DryRun = true
		case arg == "--verbose" || arg == "-v":
			config.Verbose = true
		case arg == "--no-backup":
			config.CreateBackup = false
		case arg == "--json":
			config.JSONOutput = true
		case strings.HasPrefix(arg, "--log-level="):
			config.LogLevel = strings.TrimPrefix(arg, "--log-level=")
		case strings.HasPrefix(arg, "--timeout="):
			if d, err := time.ParseDuration(strings.TrimPrefix(arg, "--timeout=")); err == nil {
				config.GitTimeout = d
			}
		default:
			gitArgs = append(gitArgs, arg)
		}
	}

	// Initialize logging
	logging.Init(logging.Config{
		Level:      logging.ParseLevel(config.LogLevel),
		JSONFormat: config.JSONOutput,
	})

	// Update git executor timeout if a custom timeout was specified via flag
	// (config.GitTimeout defaults to DefaultTimeout, so only update if different)
	if config.GitTimeout > 0 && config.GitTimeout != git.DefaultTimeout {
		gitExec = git.NewExecutorWithTimeout(config.GitTimeout)
		semantic.SetGitExecutor(gitExec)
	}

	// Initialize JSON result if needed
	var jsonResult *output.MergeResult
	if config.JSONOutput {
		jsonResult = output.NewMergeResult()
		jsonResult.DryRun = config.DryRun
	}

	// Check if we're in a git repo
	if !isGitRepo(ctx) {
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("not a git repository"))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error("Not a git repository")
		}
		return exitcode.NotGitRepo
	}

	// Display header (unless JSON output)
	if !config.JSONOutput {
		ui.Header("G2 Smart Merge")
	}

	// In dry-run mode, we still need to run git merge to detect conflicts
	// but we won't write any synthesized changes
	if config.DryRun && !config.JSONOutput {
		ui.Info("Dry-run mode: no files will be modified")
		fmt.Println()
	}

	// Run git merge
	if !config.JSONOutput {
		ui.Step("Running git merge...")
	}
	logging.Debug("running git merge", "args", gitArgs)

	err := gitExec.RunWithStdio(ctx, gitArgs...)

	if err == nil {
		// Merge succeeded without conflicts
		if config.JSONOutput {
			jsonResult.Success = true
			output.WriteJSONStdout(jsonResult)
		} else {
			fmt.Println()
			ui.Success("Merge completed successfully!")
		}
		return exitcode.Success
	}

	// Check for timeout
	if git.IsTimeoutError(err) {
		logging.Error("git merge timed out", "error", err)
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("git merge timed out"))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error("Git merge timed out")
		}
		return exitcode.TimeoutError
	}

	// Check exit code - handle both *exec.ExitError and any error with ExitCode() method
	type exitCoder interface {
		ExitCode() int
	}
	var mergeExitCode int
	if exitErr, ok := err.(*exec.ExitError); ok {
		mergeExitCode = exitErr.ExitCode()
	} else if ec, ok := err.(exitCoder); ok {
		mergeExitCode = ec.ExitCode()
	} else {
		logging.Error("merge failed with unexpected error", "error", err)
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("merge failed: %v", err))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error(fmt.Sprintf("Merge failed: %v", err))
		}
		return exitcode.GitError
	}
	if mergeExitCode != 1 {
		// Exit code other than 1 indicates a different error
		logging.Error("git merge failed with exit code", "exit_code", mergeExitCode)
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("git merge failed with exit code %d", mergeExitCode))
			output.WriteJSONStdout(jsonResult)
		}
		return mergeExitCode
	}

	// Exit code 1 typically means conflicts
	if !config.JSONOutput {
		fmt.Println()
		ui.Warning("Merge conflicts detected!")
		fmt.Println()
	}
	logging.Info("merge conflicts detected")

	// Get conflicting files
	if !config.JSONOutput {
		ui.Step("Analyzing conflicts...")
	}
	conflictingFiles, err := semantic.GetConflictingFilesWithContext(ctx)
	if err != nil {
		logging.Error("failed to get conflicting files", "error", err)
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("failed to get conflicting files: %v", err))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error(fmt.Sprintf("Failed to get conflicting files: %v", err))
		}
		return exitcode.GitError
	}

	if len(conflictingFiles) == 0 {
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("no conflicting files found"))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Info("No conflicting files found (merge may have failed for another reason)")
		}
		return exitcode.GitError
	}

	// Analyze each file (use synthesis analysis for both display and synthesis)
	synthesesByFile := make(map[string]*semantic.SynthesisAnalysis)

	for _, file := range conflictingFiles {
		if semantic.IsSemanticFile(file) {
			synthesis := semantic.AnalyzeConflictForSynthesis(file)
			synthesesByFile[file] = synthesis
		}
	}

	// Detect inter-file moves across all analyses
	var allAnalyses []*semantic.SynthesisAnalysis
	for _, a := range synthesesByFile {
		allAnalyses = append(allAnalyses, a)
	}
	interFileMoves := semantic.DetectInterFileMoves(allAnalyses)
	semantic.ApplyInterFileMoves(allAnalyses, interFileMoves)

	// Find and apply import updates for inter-file moves
	if len(interFileMoves) > 0 && !config.DryRun {
		repoRoot, _ := getRepoRoot(ctx)
		if repoRoot != "" {
			importUpdates, err := semantic.FindImportUpdates(interFileMoves, repoRoot)
			if err != nil {
				logging.Warn("failed to scan for import updates", "error", err)
				if config.Verbose && !config.JSONOutput {
					ui.Warning(fmt.Sprintf("Failed to scan for import updates: %v", err))
				}
			}
			if len(importUpdates) > 0 {
				if !config.JSONOutput {
					fmt.Println()
					ui.Step("Updating imports for moved definitions...")
				}
				results := semantic.ApplyImportUpdates(importUpdates, repoRoot)

				// Stage updated files
				for _, result := range results {
					if result.Error == nil && result.UpdatesCount > 0 {
						if err := gitExec.Run(ctx, "add", result.File); err != nil {
							logging.Warn("failed to stage import update", "file", result.File, "error", err)
						} else if config.Verbose && !config.JSONOutput {
							ui.Info(fmt.Sprintf("  Updated %d import(s) in %s", result.UpdatesCount, result.File))
						}
					}
				}

				totalUpdates := 0
				for _, r := range results {
					totalUpdates += r.UpdatesCount
				}
				if totalUpdates > 0 && !config.JSONOutput {
					ui.Info(fmt.Sprintf("Updated %d import(s) in %d file(s)", totalUpdates, len(results)))
				}
			}
		}
	} else if len(interFileMoves) > 0 && config.DryRun {
		// In dry-run mode, show what imports would be updated
		repoRoot, _ := getRepoRoot(ctx)
		if repoRoot != "" {
			importUpdates, _ := semantic.FindImportUpdates(interFileMoves, repoRoot)
			if len(importUpdates) > 0 && !config.JSONOutput {
				fmt.Println()
				ui.Step("Import updates needed (dry-run):")
				fmt.Print(semantic.FormatImportUpdateSummary(importUpdates))
			}
		}
	}

	// Now collect all conflicts for display (including non-semantic files)
	var allConflicts []ui.Conflict
	for _, file := range conflictingFiles {
		if semantic.IsSemanticFile(file) {
			synthesis := synthesesByFile[file]
			for _, sc := range synthesis.Conflicts {
				allConflicts = append(allConflicts, sc.UIConflict)
			}
		} else {
			analysis := semantic.AnalyzeNonSemanticFile(file)
			allConflicts = append(allConflicts, analysis.Conflicts...)
		}
	}

	// Display conflict table (unless JSON output)
	if !config.JSONOutput {
		fmt.Println()
		ui.ConflictTable(allConflicts)
	}

	// Count conflicts needing resolution
	needsResolution := 0
	for _, c := range allConflicts {
		if c.Status == "Needs Resolution" {
			needsResolution++
		}
	}

	if !config.JSONOutput {
		ui.Summary(needsResolution, len(allConflicts))
	}

	// Synthesize files
	if !config.JSONOutput {
		fmt.Println()
		if config.DryRun {
			ui.Step("Dry run - showing proposed changes...")
		} else {
			ui.Step("Synthesizing files...")
		}
	}
	allAutoMerged := true
	filesWithMarkers := 0

	for _, file := range conflictingFiles {
		if semantic.IsSemanticFile(file) {
			synthesis := synthesesByFile[file]
			result := semantic.SynthesizeFile(synthesis, config)

			// Add to JSON result if needed
			if config.JSONOutput {
				fileResult := output.FileResult{
					File:           file,
					ConflictCount:  result.ConflictCount,
					ResolvedCount:  result.AutoMergeCount,
					AllAutoMerged:  result.AllAutoMerged,
					HasMarkers:     !result.AllAutoMerged && result.Success,
				}
				if result.Error != nil {
					fileResult.Error = result.Error.Error()
				}
				jsonResult.AddFileResult(fileResult)
			}

			if result.Error != nil {
				logging.Warn("failed to synthesize file", "file", file, "error", result.Error)
				if !config.JSONOutput {
					ui.Warning(fmt.Sprintf("Failed to synthesize %s: %v", file, result.Error))
				}
				allAutoMerged = false
			} else if !result.AllAutoMerged {
				allAutoMerged = false
				filesWithMarkers++
			}
		} else {
			allAutoMerged = false
			// Add non-semantic file to JSON result
			if config.JSONOutput {
				jsonResult.AddFileResult(output.FileResult{
					File:          file,
					ConflictCount: 1,
					HasMarkers:    true,
				})
			}
		}
	}

	// Final status
	if !config.JSONOutput {
		fmt.Println()
	}

	if config.DryRun {
		// Abort the merge to restore repo state
		if err := gitExec.Run(ctx, "merge", "--abort"); err != nil {
			logging.Debug("merge abort failed (may not be in merge state)", "error", err)
		}
		if config.JSONOutput {
			jsonResult.Finalize()
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Info("Dry run complete - no files were modified")
		}
		return exitcode.Success
	} else if allAutoMerged && needsResolution == 0 {
		if config.JSONOutput {
			jsonResult.Finalize()
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Success("All conflicts auto-merged and staged!")
		}
		return exitcode.Success
	} else {
		if !config.JSONOutput {
			if filesWithMarkers > 0 {
				ui.Info(fmt.Sprintf("%d file(s) have conflict markers - resolve manually", filesWithMarkers))
			}
		}
		if config.JSONOutput {
			jsonResult.Finalize()
			output.WriteJSONStdout(jsonResult)
		}
		// Exit with error code to indicate conflicts remain
		return exitcode.ConflictsRemain
	}
}

// isGitRepo checks if the current directory is inside a git repository
func isGitRepo(ctx context.Context) bool {
	return gitExec.Run(ctx, "rev-parse", "--git-dir") == nil
}

// getRepoRoot returns the root directory of the git repository
func getRepoRoot(ctx context.Context) (string, error) {
	output, err := gitExec.Output(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// SmartMergeWithExecutor runs smart merge with a custom executor (for testing)
func SmartMergeWithExecutor(args []string, exec git.Executor) int {
	oldExec := gitExec
	gitExec = exec
	semantic.SetGitExecutor(exec)
	defer func() {
		gitExec = oldExec
		semantic.SetGitExecutor(oldExec)
	}()
	return smartMerge(args)
}

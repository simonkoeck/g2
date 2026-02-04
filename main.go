package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/simonkoeck/g2/pkg/exitcode"
	"github.com/simonkoeck/g2/pkg/git"
	"github.com/simonkoeck/g2/pkg/logging"
	"github.com/simonkoeck/g2/pkg/output"
	"github.com/simonkoeck/g2/pkg/semantic"
	"github.com/simonkoeck/g2/pkg/tui"
	"github.com/simonkoeck/g2/pkg/ui"
)

// Version is set at build time
var Version = "dev"

// OperationType represents the type of git operation
type OperationType int

const (
	OpMerge OperationType = iota
	OpRebase
	OpCherryPick
)

func (o OperationType) String() string {
	switch o {
	case OpMerge:
		return "merge"
	case OpRebase:
		return "rebase"
	case OpCherryPick:
		return "cherry-pick"
	default:
		return "unknown"
	}
}

// Package-level git executor (replaceable for testing)
var gitExec git.Executor = git.NewDefaultExecutor()

func main() {
	args := os.Args[1:]

	// Handle global flags first
	if len(args) > 0 {
		switch args[0] {
		case "--help", "-h", "help":
			printHelp()
			return
		case "--version", "-V", "version":
			fmt.Printf("g2 version %s\n", Version)
			return
		}
	}

	// If no args, check if we're in the middle of an operation
	if len(args) == 0 {
		if op := detectInProgressOperation(); op != nil {
			os.Exit(handleInProgressOperation(*op))
		}
		passthrough("git", args...)
		return
	}

	// Route to appropriate handler
	switch args[0] {
	case "merge":
		os.Exit(smartMerge(args))
	case "rebase":
		os.Exit(smartRebase(args))
	case "cherry-pick":
		os.Exit(smartCherryPick(args))
	case "continue":
		// g2 continue - continue any in-progress operation
		os.Exit(continueOperation())
	case "abort":
		// g2 abort - abort any in-progress operation
		os.Exit(abortOperation())
	case "status":
		// g2 status - show current operation status
		os.Exit(showStatus())
	default:
		passthrough("git", args...)
	}
}

func printHelp() {
	help := `G2 - Smart Git with Semantic Conflict Resolution

USAGE:
    g2 <command> [options] [args]

COMMANDS:
    merge <branch>       Merge a branch with semantic conflict resolution
    rebase <branch>      Rebase onto a branch with semantic conflict resolution
    cherry-pick <commit> Cherry-pick commits with semantic conflict resolution
    continue             Continue an in-progress operation after resolving conflicts
    abort                Abort an in-progress operation
    status               Show current operation status
    help                 Show this help message
    version              Show version information

    Any other command is passed through to git.

OPTIONS:
    --dry-run            Show what would be done without making changes
    --json               Output results as JSON
    --verbose, -v        Show detailed progress
    --no-backup          Don't create .orig backup files
    --log-level=LEVEL    Set log level (debug, info, warn, error)
    --timeout=DURATION   Set git command timeout (e.g., 30s, 1m)

EXAMPLES:
    g2 merge feature-branch
    g2 rebase main
    g2 cherry-pick abc123
    g2 merge --dry-run feature-branch
    g2 merge --json feature-branch | jq .

G2 automatically detects and resolves:
    - Function/class moves and renames
    - Identical changes made in both branches
    - Formatting-only differences
    - Inter-file moves with import updates
`
	fmt.Print(help)
}

// InProgressOperation represents an ongoing git operation
type InProgressOperation struct {
	Type    OperationType
	GitDir  string
	RepoDir string
}

// detectInProgressOperation checks if there's an ongoing merge/rebase/cherry-pick
func detectInProgressOperation() *InProgressOperation {
	ctx := context.Background()

	gitDirOutput, err := gitExec.Output(ctx, "rev-parse", "--git-dir")
	if err != nil {
		return nil
	}
	gitDir := strings.TrimSpace(string(gitDirOutput))

	repoOutput, err := gitExec.Output(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil
	}
	repoDir := strings.TrimSpace(string(repoOutput))

	// Check for rebase in progress
	if fileExists(filepath.Join(gitDir, "rebase-merge")) ||
		fileExists(filepath.Join(gitDir, "rebase-apply")) {
		return &InProgressOperation{Type: OpRebase, GitDir: gitDir, RepoDir: repoDir}
	}

	// Check for cherry-pick in progress
	if fileExists(filepath.Join(gitDir, "CHERRY_PICK_HEAD")) {
		return &InProgressOperation{Type: OpCherryPick, GitDir: gitDir, RepoDir: repoDir}
	}

	// Check for merge in progress
	if fileExists(filepath.Join(gitDir, "MERGE_HEAD")) {
		return &InProgressOperation{Type: OpMerge, GitDir: gitDir, RepoDir: repoDir}
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// handleInProgressOperation handles when g2 is called with no args during an operation
func handleInProgressOperation(op InProgressOperation) int {
	ui.Header("G2 Smart " + strings.Title(op.Type.String()))
	ui.Info(fmt.Sprintf("Detected %s in progress", op.Type.String()))
	fmt.Println()

	// Check for conflicts
	conflictingFiles, err := semantic.GetConflictingFiles()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to check conflicts: %v", err))
		return exitcode.GitError
	}

	if len(conflictingFiles) > 0 {
		ui.Warning(fmt.Sprintf("Found %d file(s) with conflicts", len(conflictingFiles)))
		fmt.Println()
		ui.Info("Run 'g2 continue' after resolving conflicts, or 'g2 abort' to cancel")
		return exitcode.ConflictsRemain
	}

	ui.Success("No conflicts detected")
	ui.Info(fmt.Sprintf("Run 'g2 continue' to finish the %s", op.Type.String()))
	return exitcode.Success
}

// continueOperation continues any in-progress operation
func continueOperation() int {
	op := detectInProgressOperation()
	if op == nil {
		ui.Error("No operation in progress")
		return exitcode.GitError
	}

	ctx := context.Background()
	config := parseGlobalConfig([]string{})

	// First, try to resolve any remaining conflicts
	conflictingFiles, _ := semantic.GetConflictingFiles()
	if len(conflictingFiles) > 0 {
		ui.Header("G2 Smart " + strings.Title(op.Type.String()))
		ui.Step("Resolving remaining conflicts...")

		exitCode := resolveConflicts(ctx, config, op.Type)
		if exitCode != exitcode.Success {
			return exitCode
		}
	}

	// Now continue the operation
	var args []string
	switch op.Type {
	case OpMerge:
		// For merge, just commit if all conflicts are resolved
		args = []string{"commit", "--no-edit"}
	case OpRebase:
		args = []string{"rebase", "--continue"}
	case OpCherryPick:
		args = []string{"cherry-pick", "--continue"}
	}

	ui.Step(fmt.Sprintf("Continuing %s...", op.Type.String()))
	err := gitExec.RunWithStdio(ctx, args...)
	if err != nil {
		// Check if there are still conflicts
		if files, _ := semantic.GetConflictingFiles(); len(files) > 0 {
			ui.Warning("Conflicts still remain - resolve them and run 'g2 continue'")
			return exitcode.ConflictsRemain
		}
		ui.Error(fmt.Sprintf("Failed to continue %s: %v", op.Type.String(), err))
		return exitcode.GitError
	}

	ui.Success(fmt.Sprintf("%s completed!", strings.Title(op.Type.String())))
	return exitcode.Success
}

// abortOperation aborts any in-progress operation
func abortOperation() int {
	op := detectInProgressOperation()
	if op == nil {
		ui.Error("No operation in progress")
		return exitcode.GitError
	}

	ctx := context.Background()
	var args []string
	switch op.Type {
	case OpMerge:
		args = []string{"merge", "--abort"}
	case OpRebase:
		args = []string{"rebase", "--abort"}
	case OpCherryPick:
		args = []string{"cherry-pick", "--abort"}
	}

	ui.Step(fmt.Sprintf("Aborting %s...", op.Type.String()))
	err := gitExec.RunWithStdio(ctx, args...)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to abort %s: %v", op.Type.String(), err))
		return exitcode.GitError
	}

	ui.Success(fmt.Sprintf("%s aborted", strings.Title(op.Type.String())))
	return exitcode.Success
}

// showStatus shows current operation status
func showStatus() int {
	op := detectInProgressOperation()

	if op == nil {
		ui.Info("No operation in progress")
		// Pass through to git status
		passthrough("git", "status")
		return exitcode.Success
	}

	ui.Header("G2 Status")
	ui.Info(fmt.Sprintf("Operation: %s in progress", op.Type.String()))

	conflictingFiles, _ := semantic.GetConflictingFiles()
	if len(conflictingFiles) > 0 {
		fmt.Println()
		ui.Warning(fmt.Sprintf("Conflicting files (%d):", len(conflictingFiles)))
		for _, f := range conflictingFiles {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println()
		ui.Info("Run 'g2 continue' to resolve and continue, or 'g2 abort' to cancel")
	} else {
		fmt.Println()
		ui.Success("All conflicts resolved")
		ui.Info(fmt.Sprintf("Run 'g2 continue' to finish the %s", op.Type.String()))
	}

	return exitcode.Success
}

// passthrough replaces the current process with git
func passthrough(cmd string, args ...string) {
	gitPath, err := exec.LookPath(cmd)
	if err != nil {
		ui.Error(fmt.Sprintf("Could not find %s: %v", cmd, err))
		os.Exit(exitcode.GitError)
	}

	execArgs := append([]string{cmd}, args...)
	if err := syscall.Exec(gitPath, execArgs, os.Environ()); err != nil {
		ui.Error(fmt.Sprintf("Failed to exec %s: %v", cmd, err))
		os.Exit(exitcode.GitError)
	}
}

// parseGlobalConfig parses g2-specific flags from args
func parseGlobalConfig(args []string) semantic.MergeConfig {
	config := semantic.DefaultMergeConfig()
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
		}
	}
	return config
}

// filterG2Flags removes g2-specific flags from args, returning git args
func filterG2Flags(args []string) []string {
	var gitArgs []string
	for _, arg := range args {
		switch {
		case arg == "--dry-run",
			arg == "--verbose", arg == "-v",
			arg == "--no-backup",
			arg == "--json",
			strings.HasPrefix(arg, "--log-level="),
			strings.HasPrefix(arg, "--timeout="):
			// Skip g2-specific flags
		default:
			gitArgs = append(gitArgs, arg)
		}
	}
	return gitArgs
}

// initConfig initializes logging and executor based on config
func initConfig(config semantic.MergeConfig) {
	logging.Init(logging.Config{
		Level:      logging.ParseLevel(config.LogLevel),
		JSONFormat: config.JSONOutput,
	})

	if config.GitTimeout > 0 && config.GitTimeout != git.DefaultTimeout {
		gitExec = git.NewExecutorWithTimeout(config.GitTimeout)
		semantic.SetGitExecutor(gitExec)
	}
}

// smartMerge runs git merge with semantic conflict analysis
func smartMerge(args []string) int {
	ctx := context.Background()
	config := parseGlobalConfig(args)
	gitArgs := filterG2Flags(args)
	initConfig(config)

	var jsonResult *output.MergeResult
	if config.JSONOutput {
		jsonResult = output.NewMergeResult()
		jsonResult.DryRun = config.DryRun
	}

	if !isGitRepo(ctx) {
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("not a git repository"))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error("Not a git repository")
		}
		return exitcode.NotGitRepo
	}

	if !config.JSONOutput {
		ui.Header("G2 Smart Merge")
		if config.DryRun {
			ui.Info("Dry-run mode: no files will be modified")
			fmt.Println()
		}
		ui.Step("Running git merge...")
	}
	logging.Debug("running git merge", "args", gitArgs)

	err := gitExec.RunWithStdio(ctx, gitArgs...)
	return handleOperationResult(ctx, config, jsonResult, err, OpMerge)
}

// smartRebase runs git rebase with semantic conflict analysis
func smartRebase(args []string) int {
	ctx := context.Background()
	config := parseGlobalConfig(args)
	gitArgs := filterG2Flags(args)
	initConfig(config)

	var jsonResult *output.MergeResult
	if config.JSONOutput {
		jsonResult = output.NewMergeResult()
		jsonResult.DryRun = config.DryRun
	}

	if !isGitRepo(ctx) {
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("not a git repository"))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error("Not a git repository")
		}
		return exitcode.NotGitRepo
	}

	// Check for --continue, --abort, --skip
	for _, arg := range gitArgs {
		switch arg {
		case "--continue":
			return continueOperation()
		case "--abort":
			return abortOperation()
		case "--skip":
			// Pass through to git
			if !config.JSONOutput {
				ui.Step("Skipping current commit...")
			}
			if err := gitExec.RunWithStdio(ctx, gitArgs...); err != nil {
				return exitcode.GitError
			}
			return exitcode.Success
		}
	}

	if !config.JSONOutput {
		ui.Header("G2 Smart Rebase")
		if config.DryRun {
			ui.Info("Dry-run mode: no files will be modified")
			fmt.Println()
		}
		ui.Step("Running git rebase...")
	}
	logging.Debug("running git rebase", "args", gitArgs)

	err := gitExec.RunWithStdio(ctx, gitArgs...)
	return handleOperationResult(ctx, config, jsonResult, err, OpRebase)
}

// smartCherryPick runs git cherry-pick with semantic conflict analysis
func smartCherryPick(args []string) int {
	ctx := context.Background()
	config := parseGlobalConfig(args)
	gitArgs := filterG2Flags(args)
	initConfig(config)

	var jsonResult *output.MergeResult
	if config.JSONOutput {
		jsonResult = output.NewMergeResult()
		jsonResult.DryRun = config.DryRun
	}

	if !isGitRepo(ctx) {
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("not a git repository"))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error("Not a git repository")
		}
		return exitcode.NotGitRepo
	}

	// Check for --continue, --abort
	for _, arg := range gitArgs {
		switch arg {
		case "--continue":
			return continueOperation()
		case "--abort":
			return abortOperation()
		}
	}

	if !config.JSONOutput {
		ui.Header("G2 Smart Cherry-Pick")
		if config.DryRun {
			ui.Info("Dry-run mode: no files will be modified")
			fmt.Println()
		}
		ui.Step("Running git cherry-pick...")
	}
	logging.Debug("running git cherry-pick", "args", gitArgs)

	err := gitExec.RunWithStdio(ctx, gitArgs...)
	return handleOperationResult(ctx, config, jsonResult, err, OpCherryPick)
}

// handleOperationResult handles the result of a git operation (merge/rebase/cherry-pick)
func handleOperationResult(ctx context.Context, config semantic.MergeConfig, jsonResult *output.MergeResult, err error, opType OperationType) int {
	if err == nil {
		if config.JSONOutput {
			jsonResult.Success = true
			output.WriteJSONStdout(jsonResult)
		} else {
			fmt.Println()
			ui.Success(fmt.Sprintf("%s completed successfully!", strings.Title(opType.String())))
		}
		return exitcode.Success
	}

	// Check for timeout
	if git.IsTimeoutError(err) {
		logging.Error("git operation timed out", "operation", opType.String(), "error", err)
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("git %s timed out", opType.String()))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error(fmt.Sprintf("Git %s timed out", opType.String()))
		}
		return exitcode.TimeoutError
	}

	// Check exit code
	type exitCoder interface {
		ExitCode() int
	}
	var opExitCode int
	if exitErr, ok := err.(*exec.ExitError); ok {
		opExitCode = exitErr.ExitCode()
	} else if ec, ok := err.(exitCoder); ok {
		opExitCode = ec.ExitCode()
	} else {
		logging.Error("operation failed with unexpected error", "operation", opType.String(), "error", err)
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("%s failed: %v", opType.String(), err))
			output.WriteJSONStdout(jsonResult)
		} else {
			ui.Error(fmt.Sprintf("%s failed: %v", strings.Title(opType.String()), err))
		}
		return exitcode.GitError
	}

	// For rebase/cherry-pick, exit code 1 or 128 can indicate conflicts
	// For merge, exit code 1 indicates conflicts
	if opExitCode != 1 && !(opType != OpMerge && opExitCode == 128) {
		logging.Error("git operation failed", "operation", opType.String(), "exit_code", opExitCode)
		if config.JSONOutput {
			jsonResult.SetError(fmt.Errorf("git %s failed with exit code %d", opType.String(), opExitCode))
			output.WriteJSONStdout(jsonResult)
		}
		return opExitCode
	}

	// Likely conflicts - try to resolve them
	if !config.JSONOutput {
		fmt.Println()
		ui.Warning("Conflicts detected!")
		fmt.Println()
	}
	logging.Info("conflicts detected", "operation", opType.String())

	return resolveConflictsWithJSON(ctx, config, jsonResult, opType)
}

// resolveConflicts resolves conflicts without JSON tracking
func resolveConflicts(ctx context.Context, config semantic.MergeConfig, opType OperationType) int {
	return resolveConflictsWithJSON(ctx, config, nil, opType)
}

// resolveConflictsWithJSON resolves conflicts with optional JSON output
func resolveConflictsWithJSON(ctx context.Context, config semantic.MergeConfig, jsonResult *output.MergeResult, opType OperationType) int {
	if !config.JSONOutput {
		ui.Step("Analyzing conflicts...")
	}

	conflictingFiles, err := semantic.GetConflictingFilesWithContext(ctx)
	if err != nil {
		logging.Error("failed to get conflicting files", "error", err)
		if config.JSONOutput && jsonResult != nil {
			jsonResult.SetError(fmt.Errorf("failed to get conflicting files: %v", err))
			output.WriteJSONStdout(jsonResult)
		} else if !config.JSONOutput {
			ui.Error(fmt.Sprintf("Failed to get conflicting files: %v", err))
		}
		return exitcode.GitError
	}

	if len(conflictingFiles) == 0 {
		if config.JSONOutput && jsonResult != nil {
			jsonResult.SetError(fmt.Errorf("no conflicting files found"))
			output.WriteJSONStdout(jsonResult)
		} else if !config.JSONOutput {
			ui.Info("No conflicting files found (operation may have failed for another reason)")
		}
		return exitcode.GitError
	}

	// Analyze each file
	synthesesByFile := make(map[string]*semantic.SynthesisAnalysis)
	for _, file := range conflictingFiles {
		if semantic.IsSemanticFile(file) {
			synthesis := semantic.AnalyzeConflictForSynthesis(file)
			synthesesByFile[file] = synthesis
		}
	}

	// Detect inter-file moves
	var allAnalyses []*semantic.SynthesisAnalysis
	for _, a := range synthesesByFile {
		allAnalyses = append(allAnalyses, a)
	}
	interFileMoves := semantic.DetectInterFileMoves(allAnalyses)
	semantic.ApplyInterFileMoves(allAnalyses, interFileMoves)

	// Handle import updates for inter-file moves
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

	// Collect all conflicts for display
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

	if !config.JSONOutput {
		fmt.Println()
		ui.ConflictTable(allConflicts)
	}

	needsResolution := 0
	for _, c := range allConflicts {
		if c.Status == "Needs Resolution" {
			needsResolution++
		}
	}

	if !config.JSONOutput {
		ui.Summary(needsResolution, len(allConflicts))
		fmt.Println()
	}

	// If running in a terminal and there are unresolved conflicts, launch TUI BEFORE synthesis
	if !config.JSONOutput && !config.DryRun && tui.IsTerminal() && needsResolution > 0 {
		return launchConflictTUI(ctx, config, conflictingFiles, synthesesByFile, jsonResult, opType)
	}

	if !config.JSONOutput {
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

			if config.JSONOutput && jsonResult != nil {
				fileResult := output.FileResult{
					File:          file,
					ConflictCount: result.ConflictCount,
					ResolvedCount: result.AutoMergeCount,
					AllAutoMerged: result.AllAutoMerged,
					HasMarkers:    !result.AllAutoMerged && result.Success,
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
			if config.JSONOutput && jsonResult != nil {
				jsonResult.AddFileResult(output.FileResult{
					File:          file,
					ConflictCount: 1,
					HasMarkers:    true,
				})
			}
		}
	}

	if !config.JSONOutput {
		fmt.Println()
	}

	if config.DryRun {
		// Abort the operation to restore repo state
		var abortArgs []string
		switch opType {
		case OpMerge:
			abortArgs = []string{"merge", "--abort"}
		case OpRebase:
			abortArgs = []string{"rebase", "--abort"}
		case OpCherryPick:
			abortArgs = []string{"cherry-pick", "--abort"}
		}
		if err := gitExec.Run(ctx, abortArgs...); err != nil {
			logging.Debug("abort failed (may not be in operation state)", "error", err)
		}
		if config.JSONOutput && jsonResult != nil {
			jsonResult.Finalize()
			output.WriteJSONStdout(jsonResult)
		} else if !config.JSONOutput {
			ui.Info("Dry run complete - no files were modified")
		}
		return exitcode.Success
	}

	if allAutoMerged && needsResolution == 0 {
		if config.JSONOutput && jsonResult != nil {
			jsonResult.Finalize()
			output.WriteJSONStdout(jsonResult)
		} else if !config.JSONOutput {
			ui.Success("All conflicts auto-merged and staged!")
			if opType != OpMerge {
				ui.Info(fmt.Sprintf("Run 'g2 continue' to finish the %s", opType.String()))
			}
		}
		return exitcode.Success
	}

	if !config.JSONOutput {
		if filesWithMarkers > 0 {
			ui.Info(fmt.Sprintf("%d file(s) have conflict markers - resolve manually", filesWithMarkers))
		}
		ui.Info(fmt.Sprintf("Run 'g2 continue' to continue after resolving, or 'g2 abort' to cancel"))
	}
	if config.JSONOutput && jsonResult != nil {
		jsonResult.Finalize()
		output.WriteJSONStdout(jsonResult)
	}
	return exitcode.ConflictsRemain
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

// SmartRebaseWithExecutor runs smart rebase with a custom executor (for testing)
func SmartRebaseWithExecutor(args []string, exec git.Executor) int {
	oldExec := gitExec
	gitExec = exec
	semantic.SetGitExecutor(exec)
	defer func() {
		gitExec = oldExec
		semantic.SetGitExecutor(oldExec)
	}()
	return smartRebase(args)
}

// SmartCherryPickWithExecutor runs smart cherry-pick with a custom executor (for testing)
func SmartCherryPickWithExecutor(args []string, exec git.Executor) int {
	oldExec := gitExec
	gitExec = exec
	semantic.SetGitExecutor(exec)
	defer func() {
		gitExec = oldExec
		semantic.SetGitExecutor(oldExec)
	}()
	return smartCherryPick(args)
}

// launchConflictTUI launches the interactive TUI for resolving conflicts
func launchConflictTUI(ctx context.Context, config semantic.MergeConfig, conflictingFiles []string, synthesesByFile map[string]*semantic.SynthesisAnalysis, jsonResult *output.MergeResult, opType OperationType) int {
	// Build a map from (file, name) -> index in synthesis.Conflicts for applying resolutions
	type conflictKey struct {
		file string
		name string
	}
	conflictIndices := make(map[conflictKey]int)

	// Collect conflicts that need resolution
	var tuiConflicts []tui.ConflictItem

	for _, file := range conflictingFiles {
		if !semantic.IsSemanticFile(file) {
			continue
		}

		synthesis, ok := synthesesByFile[file]
		if !ok {
			continue
		}

		for i, sc := range synthesis.Conflicts {
			if sc.UIConflict.Status != "Needs Resolution" {
				continue
			}

			// Get name and kind from whichever definition exists
			var name, kind string
			if sc.Local != nil {
				name = sc.Local.Name
				kind = sc.Local.Kind
			} else if sc.Remote != nil {
				name = sc.Remote.Name
				kind = sc.Remote.Kind
			} else if sc.Base != nil {
				name = sc.Base.Name
				kind = sc.Base.Kind
			}

			// Store index for later resolution mapping
			conflictIndices[conflictKey{file: file, name: name}] = i

			// Get content bodies
			var baseBody, localBody, remoteBody string
			if sc.Base != nil {
				baseBody = sc.Base.Body
			}
			if sc.Local != nil {
				localBody = sc.Local.Body
			}
			if sc.Remote != nil {
				remoteBody = sc.Remote.Body
			}

			tuiConflicts = append(tuiConflicts, tui.ConflictItem{
				File:          file,
				Name:          name,
				Kind:          kind,
				ConflictType:  sc.UIConflict.ConflictType,
				BaseContent:   baseBody,
				LocalContent:  localBody,
				RemoteContent: remoteBody,
			})
		}
	}

	if len(tuiConflicts) == 0 {
		ui.Info("No conflicts to resolve interactively")
		// Fall back to standard synthesis
		return synthesizeAllFiles(ctx, config, conflictingFiles, synthesesByFile, jsonResult, opType)
	}

	ui.Info(fmt.Sprintf("\nLaunching interactive resolver for %d conflict(s)...\n", len(tuiConflicts)))

	// Run the TUI
	result, err := tui.RunResolver(tuiConflicts)
	if err != nil {
		ui.Error(fmt.Sprintf("TUI error: %v", err))
		return exitcode.GitError
	}

	if result.Aborted {
		ui.Warning("Resolution aborted")
		ui.Info(fmt.Sprintf("Run 'g2 continue' to try again, or 'g2 abort' to cancel the %s", opType.String()))
		return exitcode.ConflictsRemain
	}

	// Print summary
	tui.PrintSummary(result)

	// Apply user resolutions to the synthesis analysis
	resolved := 0
	manualEdits := 0
	for _, c := range result.Conflicts {
		if c.Resolution == tui.ResolutionNone {
			continue
		}

		if c.Resolution == tui.ResolutionSkip {
			manualEdits++
		} else {
			resolved++
		}

		// Find the synthesis conflict and apply resolution
		key := conflictKey{file: c.File, name: c.Name}
		idx, ok := conflictIndices[key]
		if !ok {
			continue
		}

		synthesis := synthesesByFile[c.File]
		if synthesis == nil || idx >= len(synthesis.Conflicts) {
			continue
		}

		// Map TUI resolution to semantic resolution
		switch c.Resolution {
		case tui.ResolutionLocal:
			synthesis.Conflicts[idx].UserResolution = semantic.UserResolutionLocal
		case tui.ResolutionRemote:
			synthesis.Conflicts[idx].UserResolution = semantic.UserResolutionRemote
		case tui.ResolutionBoth:
			synthesis.Conflicts[idx].UserResolution = semantic.UserResolutionBoth
		case tui.ResolutionBase:
			synthesis.Conflicts[idx].UserResolution = semantic.UserResolutionBase
		case tui.ResolutionSkip:
			synthesis.Conflicts[idx].UserResolution = semantic.UserResolutionSkip
		}
	}

	if resolved == 0 && manualEdits == 0 {
		ui.Info(fmt.Sprintf("\nRun 'g2 continue' to resolve remaining conflicts, or 'g2 abort' to cancel"))
		return exitcode.ConflictsRemain
	}

	// Now run synthesis with user resolutions applied
	ui.Step("\nSynthesizing files with your resolutions...")
	return synthesizeAllFilesWithManualCount(ctx, config, conflictingFiles, synthesesByFile, jsonResult, opType, manualEdits)
}

// synthesizeAllFiles runs synthesis on all conflicting files
func synthesizeAllFiles(ctx context.Context, config semantic.MergeConfig, conflictingFiles []string, synthesesByFile map[string]*semantic.SynthesisAnalysis, jsonResult *output.MergeResult, opType OperationType) int {
	return synthesizeAllFilesWithManualCount(ctx, config, conflictingFiles, synthesesByFile, jsonResult, opType, 0)
}

// synthesizeAllFilesWithManualCount runs synthesis with tracking of manually-edited conflicts
func synthesizeAllFilesWithManualCount(ctx context.Context, config semantic.MergeConfig, conflictingFiles []string, synthesesByFile map[string]*semantic.SynthesisAnalysis, jsonResult *output.MergeResult, opType OperationType, manualEdits int) int {
	allAutoMerged := true
	filesWithMarkers := 0

	for _, file := range conflictingFiles {
		if semantic.IsSemanticFile(file) {
			synthesis := synthesesByFile[file]
			result := semantic.SynthesizeFile(synthesis, config)

			if config.JSONOutput && jsonResult != nil {
				fileResult := output.FileResult{
					File:          file,
					ConflictCount: result.ConflictCount,
					ResolvedCount: result.AutoMergeCount,
					AllAutoMerged: result.AllAutoMerged,
					HasMarkers:    !result.AllAutoMerged && result.Success,
				}
				if result.Error != nil {
					fileResult.Error = result.Error.Error()
				}
				jsonResult.AddFileResult(fileResult)
			}

			if result.Error != nil {
				logging.Warn("failed to synthesize file", "file", file, "error", result.Error)
				ui.Warning(fmt.Sprintf("Failed to synthesize %s: %v", file, result.Error))
				allAutoMerged = false
			} else if !result.AllAutoMerged {
				allAutoMerged = false
				filesWithMarkers++
			}
		} else {
			allAutoMerged = false
			if config.JSONOutput && jsonResult != nil {
				jsonResult.AddFileResult(output.FileResult{
					File:          file,
					ConflictCount: 1,
					HasMarkers:    true,
				})
			}
		}
	}

	fmt.Println()

	if allAutoMerged {
		if config.JSONOutput && jsonResult != nil {
			jsonResult.Finalize()
			output.WriteJSONStdout(jsonResult)
		}
		ui.Success("All conflicts resolved and staged!")
		if opType != OpMerge {
			ui.Info(fmt.Sprintf("Run 'g2 continue' to finish the %s", opType.String()))
		}
		return exitcode.Success
	}

	// If user marked some for manual editing, provide helpful message
	if manualEdits > 0 {
		ui.Info(fmt.Sprintf("%d conflict(s) left for manual editing", manualEdits))
		ui.Info("Edit the files to resolve conflict markers, then run 'g2 continue'")
	} else if filesWithMarkers > 0 {
		ui.Info(fmt.Sprintf("%d file(s) have conflict markers - resolve manually", filesWithMarkers))
		ui.Info(fmt.Sprintf("Run 'g2 continue' to continue after resolving, or 'g2 abort' to cancel"))
	}

	if config.JSONOutput && jsonResult != nil {
		jsonResult.Finalize()
		output.WriteJSONStdout(jsonResult)
	}
	return exitcode.ConflictsRemain
}

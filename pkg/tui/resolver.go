package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ResolverResult contains the result of the TUI resolution session
type ResolverResult struct {
	Conflicts []ConflictItem
	Aborted   bool
}

// RunResolver starts the interactive conflict resolution TUI
func RunResolver(conflicts []ConflictItem) (*ResolverResult, error) {
	if len(conflicts) == 0 {
		return &ResolverResult{Conflicts: conflicts}, nil
	}

	// Create model
	model := NewModel(conflicts)

	// Create program with alt screen
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	// Extract result
	m := finalModel.(Model)
	return &ResolverResult{
		Conflicts: m.Conflicts,
		Aborted:   m.Aborted,
	}, nil
}

// PrintSummary prints a summary of the resolutions
func PrintSummary(result *ResolverResult) {
	if result.Aborted {
		fmt.Println(lipgloss.NewStyle().Foreground(ErrorRed).Render("\n" + IconCross + "Resolution aborted"))
		return
	}

	resolved := 0
	manual := 0
	for _, c := range result.Conflicts {
		if c.Resolution == ResolutionSkip {
			manual++
		} else if c.Resolution != ResolutionNone {
			resolved++
		}
	}

	total := len(result.Conflicts)
	if resolved == total {
		fmt.Println(lipgloss.NewStyle().Foreground(SuccessGreen).Bold(true).Render(
			fmt.Sprintf("\n%s All %d conflicts resolved!", IconCheck, resolved),
		))
	} else if resolved+manual == total {
		fmt.Println(lipgloss.NewStyle().Foreground(SuccessGreen).Bold(true).Render(
			fmt.Sprintf("\n%s %d conflicts resolved, %d marked for manual editing", IconCheck, resolved, manual),
		))
	} else {
		fmt.Println(lipgloss.NewStyle().Foreground(WarningAmber).Render(
			fmt.Sprintf("\n%s %d of %d conflicts resolved", IconWarning, resolved+manual, total),
		))
	}

	// Print resolution details
	fmt.Println()
	for _, c := range result.Conflicts {
		var status string
		switch c.Resolution {
		case ResolutionNone:
			status = lipgloss.NewStyle().Foreground(MutedText).Render("unresolved")
		case ResolutionSkip:
			status = lipgloss.NewStyle().Foreground(WarningAmber).Render(c.Resolution.String())
		default:
			status = lipgloss.NewStyle().Foreground(SuccessGreen).Render(c.Resolution.String())
		}
		fmt.Printf("  %s %s: %s\n",
			lipgloss.NewStyle().Foreground(InfoBlue).Render(c.Name),
			lipgloss.NewStyle().Foreground(DimTextColor).Render(c.File),
			status,
		)
	}
}

// ApplyResolutions applies the resolved conflicts to the file
func ApplyResolutions(conflicts []ConflictItem, fileContent []byte) ([]byte, error) {
	// Sort conflicts by byte position descending (apply from end to beginning)
	sorted := make([]ConflictItem, len(conflicts))
	copy(sorted, conflicts)

	for i := len(sorted) - 1; i >= 0; i-- {
		for j := 0; j < i; j++ {
			if sorted[j].StartByte < sorted[j+1].StartByte {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	// Apply each resolution
	result := fileContent
	for _, c := range sorted {
		if c.Resolution == ResolutionNone {
			continue
		}

		var replacement string
		switch c.Resolution {
		case ResolutionLocal:
			replacement = c.LocalContent
		case ResolutionRemote:
			replacement = c.RemoteContent
		case ResolutionBoth:
			// Keep both, with local first
			replacement = c.LocalContent
			if c.RemoteContent != "" {
				if replacement != "" {
					replacement += "\n\n"
				}
				replacement += c.RemoteContent
			}
		case ResolutionBase:
			replacement = c.BaseContent
		}

		// Replace the byte range
		if c.EndByte <= uint32(len(result)) {
			result = replaceBytes(result, c.StartByte, c.EndByte, []byte(replacement))
		}
	}

	return result, nil
}

func replaceBytes(content []byte, start, end uint32, replacement []byte) []byte {
	if start > uint32(len(content)) {
		start = uint32(len(content))
	}
	if end > uint32(len(content)) {
		end = uint32(len(content))
	}
	if start > end {
		start = end
	}

	result := make([]byte, 0, len(content)-int(end-start)+len(replacement))
	result = append(result, content[:start]...)
	result = append(result, replacement...)
	result = append(result, content[end:]...)
	return result
}

// IsTerminal checks if we're running in an interactive terminal
func IsTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// ConvertFromSynthesis converts SynthesisConflict to ConflictItem for the TUI
// This is a helper that can be called from main.go
func ConvertConflicts(file string, conflicts []struct {
	Name         string
	Kind         string
	ConflictType string
	Status       string
	BaseBody     string
	LocalBody    string
	RemoteBody   string
	StartByte    uint32
	EndByte      uint32
}) []ConflictItem {
	var items []ConflictItem

	for _, c := range conflicts {
		// Only include conflicts that need resolution
		if c.Status != "Needs Resolution" {
			continue
		}

		items = append(items, ConflictItem{
			File:          file,
			Name:          c.Name,
			Kind:          c.Kind,
			ConflictType:  c.ConflictType,
			BaseContent:   c.BaseBody,
			LocalContent:  c.LocalBody,
			RemoteContent: c.RemoteBody,
			Resolution:    ResolutionNone,
			StartByte:     c.StartByte,
			EndByte:       c.EndByte,
		})
	}

	return items
}

// FormatDiff creates a simple diff view between two strings
func FormatDiff(local, remote string) string {
	localLines := strings.Split(local, "\n")
	remoteLines := strings.Split(remote, "\n")

	var result strings.Builder

	// Simple side-by-side diff (not a real diff algorithm)
	maxLines := len(localLines)
	if len(remoteLines) > maxLines {
		maxLines = len(remoteLines)
	}

	for i := 0; i < maxLines; i++ {
		var localLine, remoteLine string
		if i < len(localLines) {
			localLine = localLines[i]
		}
		if i < len(remoteLines) {
			remoteLine = remoteLines[i]
		}

		if localLine == remoteLine {
			result.WriteString("  " + localLine + "\n")
		} else {
			if localLine != "" {
				result.WriteString(RemovedLineStyle.Render("- " + localLine) + "\n")
			}
			if remoteLine != "" {
				result.WriteString(AddedLineStyle.Render("+ " + remoteLine) + "\n")
			}
		}
	}

	return result.String()
}

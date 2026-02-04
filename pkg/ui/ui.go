package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette - Modern Minimalist
var (
	InfoBlue     = lipgloss.Color("#6C9BCF")
	SuccessGreen = lipgloss.Color("#7CB486")
	ErrorRed     = lipgloss.Color("#E07A7A")
	WarningAmber = lipgloss.Color("#D9A648")
	BorderGray   = lipgloss.Color("#4A5568")
	MutedText    = lipgloss.Color("#718096")
	DimText      = lipgloss.Color("#A0AEC0")
	BGAccent     = lipgloss.Color("#2D3748")
)

// Nerd Font icons
const (
	IconInfo     = ""
	IconSuccess  = ""
	IconError    = ""
	IconWarning  = ""
	IconMerge    = ""
	IconFile     = ""
	IconStep     = ""
	IconFunction = "󰊕"
	IconClass    = ""
)

// Conflict represents a merge conflict entry
type Conflict struct {
	File         string
	ConflictType string
	Status       string
}

// Styles
var (
	infoStyle = lipgloss.NewStyle().
			Foreground(InfoBlue).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(SuccessGreen).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(ErrorRed).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(WarningAmber).
			Bold(true)

	stepStyle = lipgloss.NewStyle().
			Foreground(DimText)

	mutedStyle = lipgloss.NewStyle().
			Foreground(MutedText)

	headerStyle = lipgloss.NewStyle().
			Foreground(InfoBlue).
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderGray)

	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(InfoBlue).
				Bold(true)

	tableCellStyle = lipgloss.NewStyle().
			Foreground(DimText)

	conflictCellStyle = lipgloss.NewStyle().
				Foreground(ErrorRed)

	autoMergeCellStyle = lipgloss.NewStyle().
				Foreground(SuccessGreen)
)

// Info prints an info message with blue icon
func Info(msg string) {
	fmt.Printf("%s %s\n", infoStyle.Render(IconInfo), msg)
}

// Success prints a success message with green checkmark
func Success(msg string) {
	fmt.Printf("%s %s\n", successStyle.Render(IconSuccess), msg)
}

// Error prints an error message with red X
func Error(msg string) {
	fmt.Printf("%s %s\n", errorStyle.Render(IconError), msg)
}

// Warning prints a warning message with amber triangle
func Warning(msg string) {
	fmt.Printf("%s %s\n", warningStyle.Render(IconWarning), msg)
}

// Step prints a step indicator with arrow
func Step(msg string) {
	fmt.Printf("%s %s\n", stepStyle.Render(IconStep), mutedStyle.Render(msg))
}

// Header prints a styled header box with merge icon
func Header(title string) {
	content := fmt.Sprintf("%s %s", IconMerge, title)
	fmt.Println(headerStyle.Render(content))
	fmt.Println()
}

// ConflictTable renders a beautiful table of conflicts
func ConflictTable(conflicts []Conflict) {
	if len(conflicts) == 0 {
		return
	}

	// Calculate column widths
	fileWidth := 4 // "FILE"
	typeWidth := 13 // "CONFLICT TYPE"
	statusWidth := 6 // "STATUS"

	for _, c := range conflicts {
		if len(c.File) > fileWidth {
			fileWidth = len(c.File)
		}
		if len(c.ConflictType) > typeWidth {
			typeWidth = len(c.ConflictType)
		}
		if len(c.Status) > statusWidth {
			statusWidth = len(c.Status)
		}
	}

	// Add padding
	fileWidth += 2
	typeWidth += 2
	statusWidth += 2

	// Border characters
	topLeft := "╭"
	topRight := "╮"
	bottomLeft := "╰"
	bottomRight := "╯"
	horizontal := "─"
	vertical := "│"
	topT := "┬"
	bottomT := "┴"
	leftT := "├"
	rightT := "┤"
	cross := "┼"

	borderStyle := lipgloss.NewStyle().Foreground(BorderGray)

	// Helper to create horizontal line
	hLine := func(left, mid, right string) string {
		return borderStyle.Render(
			left +
				strings.Repeat(horizontal, fileWidth) +
				mid +
				strings.Repeat(horizontal, typeWidth) +
				mid +
				strings.Repeat(horizontal, statusWidth) +
				right,
		)
	}

	// Helper to pad string
	pad := func(s string, width int) string {
		return " " + s + strings.Repeat(" ", width-len(s)-1)
	}

	// Print top border
	fmt.Println(hLine(topLeft, topT, topRight))

	// Print header row
	fmt.Printf("%s%s%s%s%s%s%s\n",
		borderStyle.Render(vertical),
		tableHeaderStyle.Render(pad(IconFile+" FILE", fileWidth)),
		borderStyle.Render(vertical),
		tableHeaderStyle.Render(pad("CONFLICT TYPE", typeWidth)),
		borderStyle.Render(vertical),
		tableHeaderStyle.Render(pad("STATUS", statusWidth)),
		borderStyle.Render(vertical),
	)

	// Print header separator
	fmt.Println(hLine(leftT, cross, rightT))

	// Print data rows
	for _, c := range conflicts {
		statusStyle := tableCellStyle
		if c.Status == "Needs Resolution" {
			statusStyle = conflictCellStyle
		} else if c.Status == "Can Auto-merge" {
			statusStyle = autoMergeCellStyle
		}

		fmt.Printf("%s%s%s%s%s%s%s\n",
			borderStyle.Render(vertical),
			tableCellStyle.Render(pad(c.File, fileWidth)),
			borderStyle.Render(vertical),
			tableCellStyle.Render(pad(c.ConflictType, typeWidth)),
			borderStyle.Render(vertical),
			statusStyle.Render(pad(c.Status, statusWidth)),
			borderStyle.Render(vertical),
		)
	}

	// Print bottom border
	fmt.Println(hLine(bottomLeft, bottomT, bottomRight))
}

// Summary prints a conflict summary
func Summary(needsResolution, total int) {
	fmt.Println()
	if needsResolution > 0 {
		Warning(fmt.Sprintf("%d of %d conflicts need manual resolution", needsResolution, total))
	} else {
		Success(fmt.Sprintf("All %d conflicts can be auto-merged", total))
	}
}

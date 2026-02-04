package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model
func (m Model) View() string {
	if m.Width == 0 {
		return "Loading..."
	}

	var content string

	switch m.Mode {
	case ViewList:
		content = m.renderListView()
	case ViewDetail:
		content = m.renderDetailView()
	case ViewResolving:
		content = m.renderResolvingView()
	}

	return AppStyle.Render(content)
}

func (m Model) renderListView() string {
	var b strings.Builder

	// Header
	header := HeaderStyle.Render(fmt.Sprintf("%s Conflict Resolution", IconMerge))
	b.WriteString(header)
	b.WriteString("\n\n")

	// Progress
	resolved := m.resolvedCount()
	total := len(m.Conflicts)
	progressText := fmt.Sprintf("Progress: %d/%d resolved", resolved, total)
	if resolved == total {
		progressText = StatusResolved.Render(fmt.Sprintf("%s All conflicts resolved!", IconCheck))
	}
	b.WriteString(progressText)
	b.WriteString("\n\n")

	// Conflict list
	listContent := m.renderConflictList()
	listPanel := PanelStyle.Width(m.Width - 6).Render(listContent)
	b.WriteString(listPanel)
	b.WriteString("\n")

	// Help
	b.WriteString(m.renderListHelp())

	return b.String()
}

func (m Model) renderConflictList() string {
	var b strings.Builder

	// Calculate visible range
	visibleHeight := m.Height - 16
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	start := 0
	if m.CurrentIndex >= visibleHeight {
		start = m.CurrentIndex - visibleHeight + 1
	}
	end := start + visibleHeight
	if end > len(m.Conflicts) {
		end = len(m.Conflicts)
	}

	for i := start; i < end; i++ {
		c := m.Conflicts[i]
		isSelected := i == m.CurrentIndex

		// Build the line
		var line strings.Builder

		// Selection indicator
		if isSelected {
			line.WriteString(IconArrowRight)
		} else {
			line.WriteString("  ")
		}

		// Icon based on kind
		switch c.Kind {
		case "function", "method":
			line.WriteString(IconFunction)
		case "class", "struct", "interface":
			line.WriteString(IconClass)
		default:
			line.WriteString(IconFile)
		}

		// Name and file
		nameText := fmt.Sprintf("%-20s", truncate(c.Name, 20))
		fileText := fmt.Sprintf("%-25s", truncate(c.File, 25))

		// Status
		var statusText string
		if c.Resolution != ResolutionNone {
			statusText = StatusResolved.Render(fmt.Sprintf("%s %s", IconCheck, c.Resolution.String()))
		} else {
			statusText = StatusNeedsResolution.Render(fmt.Sprintf("%s Needs Resolution", IconWarning))
		}

		line.WriteString(nameText)
		line.WriteString(" ")
		line.WriteString(FilePathStyle.Render(fileText))
		line.WriteString(" ")
		line.WriteString(statusText)

		// Apply style
		lineStr := line.String()
		if isSelected {
			lineStr = SelectedItemStyle.Width(m.Width - 10).Render(lineStr)
		} else {
			lineStr = ItemStyle.Render(lineStr)
		}

		b.WriteString(lineStr)
		b.WriteString("\n")
	}

	// Scrollbar indicator
	if len(m.Conflicts) > visibleHeight {
		scrollInfo := fmt.Sprintf("\n  %d-%d of %d", start+1, end, len(m.Conflicts))
		b.WriteString(DimTextStyle.Render(scrollInfo))
	}

	return b.String()
}

func (m Model) renderListHelp() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%s navigate", HelpKeyStyle.Render("↑↓/jk")))
	parts = append(parts, fmt.Sprintf("%s view details", HelpKeyStyle.Render("enter")))
	parts = append(parts, fmt.Sprintf("%s local", HelpKeyStyle.Render("1")))
	parts = append(parts, fmt.Sprintf("%s remote", HelpKeyStyle.Render("2")))
	parts = append(parts, fmt.Sprintf("%s both", HelpKeyStyle.Render("3")))
	parts = append(parts, fmt.Sprintf("%s manual", HelpKeyStyle.Render("s")))

	if m.allResolved() {
		parts = append(parts, fmt.Sprintf("%s apply all", HelpKeyStyle.Render("a")))
	}
	parts = append(parts, fmt.Sprintf("%s quit", HelpKeyStyle.Render("q")))

	return HelpStyle.Render(strings.Join(parts, "  "))
}

func (m Model) renderDetailView() string {
	var b strings.Builder

	c := m.Conflicts[m.CurrentIndex]

	// Header with conflict info
	header := HeaderStyle.Render(fmt.Sprintf("%s %s: %s", IconMerge, c.Kind, c.Name))
	b.WriteString(header)
	b.WriteString("\n")

	// File path and conflict type
	b.WriteString(FilePathStyle.Render(c.File))
	b.WriteString("  ")
	b.WriteString(ConflictTypeStyle.Render(c.ConflictType))
	b.WriteString("\n\n")

	// Three-panel view
	panelWidth := (m.Width - 12) / 3
	panelHeight := m.Height - 16
	if panelHeight < 5 {
		panelHeight = 5
	}

	// Update viewport sizes
	m.BaseViewport.Width = panelWidth - 2
	m.BaseViewport.Height = panelHeight
	m.LocalViewport.Width = panelWidth - 2
	m.LocalViewport.Height = panelHeight
	m.RemoteViewport.Width = panelWidth - 2
	m.RemoteViewport.Height = panelHeight

	// Render panels
	basePanel := m.renderPanel("BASE", c.BaseContent, PanelBase, panelWidth, panelHeight)
	localPanel := m.renderPanel("LOCAL (ours)", c.LocalContent, PanelLocal, panelWidth, panelHeight)
	remotePanel := m.renderPanel("REMOTE (theirs)", c.RemoteContent, PanelRemote, panelWidth, panelHeight)

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, basePanel, "  ", localPanel, "  ", remotePanel)
	b.WriteString(panels)
	b.WriteString("\n")

	// Current resolution status
	if c.Resolution != ResolutionNone {
		b.WriteString("\n")
		b.WriteString(StatusResolved.Render(fmt.Sprintf("%s Resolution: %s", IconCheck, c.Resolution.String())))
	}

	// Help
	b.WriteString("\n")
	b.WriteString(m.renderDetailHelp())

	return b.String()
}

func (m Model) renderPanel(title, content string, panel Panel, width, height int) string {
	// Title
	titleStyle := PanelTitleStyle.Width(width - 2)
	var icon string
	switch panel {
	case PanelBase:
		icon = IconBase
	case PanelLocal:
		icon = IconLocal
	case PanelRemote:
		icon = IconRemote
	}
	titleText := titleStyle.Render(icon + title)

	// Content
	if content == "" {
		content = DimTextStyle.Render("(no content)")
	} else {
		content = formatCodeForPanel(content, width-4, height-2)
	}

	// Panel style based on focus
	var style lipgloss.Style
	if panel == m.FocusedPanel {
		style = FocusedPanelStyle.Width(width).Height(height + 2)
	} else {
		style = PanelStyle.Width(width).Height(height + 2)
	}

	panelContent := titleText + "\n" + content
	return style.Render(panelContent)
}

func (m Model) renderDetailHelp() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%s switch panel", HelpKeyStyle.Render("tab/←→")))
	parts = append(parts, fmt.Sprintf("%s scroll", HelpKeyStyle.Render("↑↓/jk")))
	parts = append(parts, fmt.Sprintf("%s local", HelpKeyStyle.Render("1")))
	parts = append(parts, fmt.Sprintf("%s remote", HelpKeyStyle.Render("2")))
	parts = append(parts, fmt.Sprintf("%s both", HelpKeyStyle.Render("3")))
	if m.Conflicts[m.CurrentIndex].BaseContent != "" {
		parts = append(parts, fmt.Sprintf("%s base", HelpKeyStyle.Render("4")))
	}
	parts = append(parts, fmt.Sprintf("%s manual", HelpKeyStyle.Render("s")))
	parts = append(parts, fmt.Sprintf("%s back", HelpKeyStyle.Render("esc")))

	return HelpStyle.Render(strings.Join(parts, "  "))
}

func (m Model) renderResolvingView() string {
	var b strings.Builder

	c := m.Conflicts[m.CurrentIndex]

	// Header
	header := HeaderStyle.Render(fmt.Sprintf("%s Choose Resolution", IconMerge))
	b.WriteString(header)
	b.WriteString("\n\n")

	// Conflict info
	b.WriteString(fmt.Sprintf("%s %s: ", c.Kind, c.Name))
	b.WriteString(FilePathStyle.Render(c.File))
	b.WriteString("\n\n")

	// Resolution options
	options := m.getResolutionOptions()
	var optionViews []string

	for i, opt := range options {
		var icon, label string
		switch opt {
		case ResolutionLocal:
			icon = IconLocal
			label = "Keep Local"
		case ResolutionRemote:
			icon = IconRemote
			label = "Keep Remote"
		case ResolutionBoth:
			icon = IconBoth
			label = "Keep Both"
		case ResolutionBase:
			icon = IconBase
			label = "Keep Base"
		case ResolutionSkip:
			icon = IconEdit
			label = "Edit Manually"
		}

		text := icon + label
		if i == m.ResolutionCursor {
			optionViews = append(optionViews, SelectedOptionStyle.Render(text))
		} else {
			optionViews = append(optionViews, OptionStyle.Render(text))
		}
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, optionViews...))
	b.WriteString("\n\n")

	// Preview
	selected := options[m.ResolutionCursor]
	b.WriteString(DimTextStyle.Render("Preview:"))
	b.WriteString("\n")

	var preview string
	switch selected {
	case ResolutionLocal:
		preview = c.LocalContent
	case ResolutionRemote:
		preview = c.RemoteContent
	case ResolutionBoth:
		preview = c.LocalContent + "\n\n" + c.RemoteContent
	case ResolutionBase:
		preview = c.BaseContent
	case ResolutionSkip:
		preview = "<<<<<<< LOCAL\n" + c.LocalContent + "\n=======\n" + c.RemoteContent + "\n>>>>>>> REMOTE"
	}

	previewFormatted := formatCodeForPanel(preview, m.Width-10, 10)
	previewPanel := PanelStyle.Width(m.Width - 6).Render(previewFormatted)
	b.WriteString(previewPanel)
	b.WriteString("\n")

	// Help
	b.WriteString(m.renderResolvingHelp())

	return b.String()
}

func (m Model) renderResolvingHelp() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("%s select", HelpKeyStyle.Render("←→")))
	parts = append(parts, fmt.Sprintf("%s confirm", HelpKeyStyle.Render("enter")))
	parts = append(parts, fmt.Sprintf("%s cancel", HelpKeyStyle.Render("esc")))

	return HelpStyle.Render(strings.Join(parts, "  "))
}

// formatCodeForPanel formats code with line numbers for a panel
func formatCodeForPanel(content string, width, maxLines int) string {
	if content == "" {
		return DimTextStyle.Render("(empty)")
	}

	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	var result strings.Builder
	lineNumWidth := len(fmt.Sprintf("%d", len(lines)))

	for i, line := range lines {
		lineNum := fmt.Sprintf("%*d", lineNumWidth, i+1)
		lineNumStyled := LineNumberStyle.Render(lineNum)

		// Truncate line if too long
		if len(line) > width-lineNumWidth-3 {
			line = line[:width-lineNumWidth-6] + "..."
		}

		result.WriteString(lineNumStyled)
		result.WriteString(" ")
		result.WriteString(CodeStyle.Render(line))
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	if len(strings.Split(content, "\n")) > maxLines {
		result.WriteString("\n")
		result.WriteString(DimTextStyle.Render(fmt.Sprintf("... +%d more lines", len(strings.Split(content, "\n"))-maxLines)))
	}

	return result.String()
}

// truncate truncates a string to maxLen
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

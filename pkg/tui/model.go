package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Resolution represents how a conflict was resolved
type Resolution int

const (
	ResolutionNone Resolution = iota
	ResolutionLocal
	ResolutionRemote
	ResolutionBoth
	ResolutionBase
	ResolutionSkip // Leave for manual editing
)

func (r Resolution) String() string {
	switch r {
	case ResolutionLocal:
		return "Keep Local"
	case ResolutionRemote:
		return "Keep Remote"
	case ResolutionBoth:
		return "Keep Both"
	case ResolutionBase:
		return "Keep Base"
	case ResolutionSkip:
		return "Edit Manually"
	default:
		return "Unresolved"
	}
}

// ConflictItem represents a single conflict to resolve
type ConflictItem struct {
	File         string
	Name         string // Definition name (e.g., function name)
	Kind         string // "function", "class", etc.
	ConflictType string
	BaseContent  string
	LocalContent string
	RemoteContent string
	Resolution   Resolution
	StartByte    uint32
	EndByte      uint32
}

// View mode
type ViewMode int

const (
	ViewList ViewMode = iota
	ViewDetail
	ViewResolving
)

// Panel focus in detail view
type Panel int

const (
	PanelBase Panel = iota
	PanelLocal
	PanelRemote
)

// Model is the main TUI model
type Model struct {
	// Data
	Conflicts    []ConflictItem
	CurrentIndex int

	// View state
	Mode         ViewMode
	FocusedPanel Panel
	Width        int
	Height       int

	// Viewports for scrolling
	ListViewport   viewport.Model
	BaseViewport   viewport.Model
	LocalViewport  viewport.Model
	RemoteViewport viewport.Model

	// Resolution selection
	ResolutionCursor int

	// Result
	Quit     bool
	Aborted  bool
}

// KeyMap defines keyboard shortcuts
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Tab      key.Binding
	Escape   key.Binding
	Quit     key.Binding
	Help     key.Binding
	Local    key.Binding
	Remote   key.Binding
	Both     key.Binding
	Base     key.Binding
	Skip     key.Binding
	Apply    key.Binding
}

var keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select/confirm"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next panel"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Local: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "keep local"),
	),
	Remote: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "keep remote"),
	),
	Both: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "keep both"),
	),
	Base: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "keep base"),
	),
	Skip: key.NewBinding(
		key.WithKeys("s", "5"),
		key.WithHelp("s/5", "edit manually"),
	),
	Apply: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "apply all"),
	),
}

// NewModel creates a new TUI model
func NewModel(conflicts []ConflictItem) Model {
	m := Model{
		Conflicts:    conflicts,
		CurrentIndex: 0,
		Mode:         ViewList,
		FocusedPanel: PanelLocal,
		Width:        80,
		Height:       24,
	}

	// Initialize viewports
	m.ListViewport = viewport.New(80, 20)
	m.BaseViewport = viewport.New(25, 15)
	m.LocalViewport = viewport.New(25, 15)
	m.RemoteViewport = viewport.New(25, 15)

	return m
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.updateViewportSizes()
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	// Update viewports
	var cmd tea.Cmd
	switch m.Mode {
	case ViewList:
		m.ListViewport, cmd = m.ListViewport.Update(msg)
	case ViewDetail:
		switch m.FocusedPanel {
		case PanelBase:
			m.BaseViewport, cmd = m.BaseViewport.Update(msg)
		case PanelLocal:
			m.LocalViewport, cmd = m.LocalViewport.Update(msg)
		case PanelRemote:
			m.RemoteViewport, cmd = m.RemoteViewport.Update(msg)
		}
	}

	return m, cmd
}

func (m *Model) updateViewportSizes() {
	// List view takes full width
	m.ListViewport.Width = m.Width - 4
	m.ListViewport.Height = m.Height - 10

	// Detail view splits into three panels
	panelWidth := (m.Width - 8) / 3
	panelHeight := m.Height - 14

	m.BaseViewport.Width = panelWidth
	m.BaseViewport.Height = panelHeight
	m.LocalViewport.Width = panelWidth
	m.LocalViewport.Height = panelHeight
	m.RemoteViewport.Width = panelWidth
	m.RemoteViewport.Height = panelHeight
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.Mode {
	case ViewList:
		return m.handleListKeys(msg)
	case ViewDetail:
		return m.handleDetailKeys(msg)
	case ViewResolving:
		return m.handleResolvingKeys(msg)
	}
	return m, nil
}

func (m Model) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.Quit = true
		m.Aborted = true
		return m, tea.Quit

	case key.Matches(msg, keys.Up):
		if m.CurrentIndex > 0 {
			m.CurrentIndex--
		}

	case key.Matches(msg, keys.Down):
		if m.CurrentIndex < len(m.Conflicts)-1 {
			m.CurrentIndex++
		}

	case key.Matches(msg, keys.Enter):
		m.Mode = ViewDetail
		m.updateDetailViewports()

	case key.Matches(msg, keys.Apply):
		// Apply all resolutions and quit
		if m.allResolved() {
			m.Quit = true
			return m, tea.Quit
		}

	case key.Matches(msg, keys.Local):
		m.Conflicts[m.CurrentIndex].Resolution = ResolutionLocal
		m.moveToNextUnresolved()

	case key.Matches(msg, keys.Remote):
		m.Conflicts[m.CurrentIndex].Resolution = ResolutionRemote
		m.moveToNextUnresolved()

	case key.Matches(msg, keys.Both):
		m.Conflicts[m.CurrentIndex].Resolution = ResolutionBoth
		m.moveToNextUnresolved()

	case key.Matches(msg, keys.Base):
		if m.Conflicts[m.CurrentIndex].BaseContent != "" {
			m.Conflicts[m.CurrentIndex].Resolution = ResolutionBase
			m.moveToNextUnresolved()
		}

	case key.Matches(msg, keys.Skip):
		m.Conflicts[m.CurrentIndex].Resolution = ResolutionSkip
		m.moveToNextUnresolved()
	}

	return m, nil
}

func (m Model) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.Mode = ViewList

	case key.Matches(msg, keys.Quit):
		m.Quit = true
		m.Aborted = true
		return m, tea.Quit

	case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Right):
		m.FocusedPanel = (m.FocusedPanel + 1) % 3

	case key.Matches(msg, keys.Left):
		m.FocusedPanel = (m.FocusedPanel + 2) % 3

	case key.Matches(msg, keys.Up):
		switch m.FocusedPanel {
		case PanelBase:
			m.BaseViewport.LineUp(1)
		case PanelLocal:
			m.LocalViewport.LineUp(1)
		case PanelRemote:
			m.RemoteViewport.LineUp(1)
		}

	case key.Matches(msg, keys.Down):
		switch m.FocusedPanel {
		case PanelBase:
			m.BaseViewport.LineDown(1)
		case PanelLocal:
			m.LocalViewport.LineDown(1)
		case PanelRemote:
			m.RemoteViewport.LineDown(1)
		}

	case key.Matches(msg, keys.Enter):
		m.Mode = ViewResolving
		m.ResolutionCursor = 0

	case key.Matches(msg, keys.Local):
		m.Conflicts[m.CurrentIndex].Resolution = ResolutionLocal
		m.Mode = ViewList
		m.moveToNextUnresolved()

	case key.Matches(msg, keys.Remote):
		m.Conflicts[m.CurrentIndex].Resolution = ResolutionRemote
		m.Mode = ViewList
		m.moveToNextUnresolved()

	case key.Matches(msg, keys.Both):
		m.Conflicts[m.CurrentIndex].Resolution = ResolutionBoth
		m.Mode = ViewList
		m.moveToNextUnresolved()

	case key.Matches(msg, keys.Base):
		if m.Conflicts[m.CurrentIndex].BaseContent != "" {
			m.Conflicts[m.CurrentIndex].Resolution = ResolutionBase
			m.Mode = ViewList
			m.moveToNextUnresolved()
		}

	case key.Matches(msg, keys.Skip):
		m.Conflicts[m.CurrentIndex].Resolution = ResolutionSkip
		m.Mode = ViewList
		m.moveToNextUnresolved()
	}

	return m, nil
}

func (m Model) handleResolvingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	options := m.getResolutionOptions()

	switch {
	case key.Matches(msg, keys.Escape):
		m.Mode = ViewDetail

	case key.Matches(msg, keys.Quit):
		m.Quit = true
		m.Aborted = true
		return m, tea.Quit

	case key.Matches(msg, keys.Left):
		if m.ResolutionCursor > 0 {
			m.ResolutionCursor--
		}

	case key.Matches(msg, keys.Right):
		if m.ResolutionCursor < len(options)-1 {
			m.ResolutionCursor++
		}

	case key.Matches(msg, keys.Enter):
		m.Conflicts[m.CurrentIndex].Resolution = options[m.ResolutionCursor]
		m.Mode = ViewList
		m.moveToNextUnresolved()
	}

	return m, nil
}

func (m *Model) updateDetailViewports() {
	if m.CurrentIndex >= len(m.Conflicts) {
		return
	}
	c := m.Conflicts[m.CurrentIndex]

	m.BaseViewport.SetContent(formatCode(c.BaseContent))
	m.LocalViewport.SetContent(formatCode(c.LocalContent))
	m.RemoteViewport.SetContent(formatCode(c.RemoteContent))

	m.BaseViewport.GotoTop()
	m.LocalViewport.GotoTop()
	m.RemoteViewport.GotoTop()
}

func (m *Model) moveToNextUnresolved() {
	// Find next unresolved conflict
	for i := m.CurrentIndex + 1; i < len(m.Conflicts); i++ {
		if m.Conflicts[i].Resolution == ResolutionNone {
			m.CurrentIndex = i
			return
		}
	}
	// Wrap around
	for i := 0; i < m.CurrentIndex; i++ {
		if m.Conflicts[i].Resolution == ResolutionNone {
			m.CurrentIndex = i
			return
		}
	}
}

func (m Model) allResolved() bool {
	for _, c := range m.Conflicts {
		if c.Resolution == ResolutionNone {
			return false
		}
	}
	return true
}

// hasManualEdits returns true if any conflicts are marked for manual editing
func (m Model) hasManualEdits() bool {
	for _, c := range m.Conflicts {
		if c.Resolution == ResolutionSkip {
			return true
		}
	}
	return false
}

func (m Model) getResolutionOptions() []Resolution {
	c := m.Conflicts[m.CurrentIndex]
	options := []Resolution{ResolutionLocal, ResolutionRemote, ResolutionBoth}
	if c.BaseContent != "" {
		options = append(options, ResolutionBase)
	}
	options = append(options, ResolutionSkip)
	return options
}

func (m Model) resolvedCount() int {
	count := 0
	for _, c := range m.Conflicts {
		if c.Resolution != ResolutionNone {
			count++
		}
	}
	return count
}

// formatCode adds line numbers and formatting to code content
func formatCode(content string) string {
	if content == "" {
		return DimTextStyle.Render("(empty)")
	}

	lines := strings.Split(content, "\n")
	var result strings.Builder

	for i, line := range lines {
		lineNum := LineNumberStyle.Render(lipgloss.NewStyle().Width(4).Align(lipgloss.Right).Render(
			strings.Repeat(" ", 3-len(string(rune('0'+((i+1)/100)%10)))) + string(rune('0'+((i+1)/100)%10)) +
				string(rune('0'+((i+1)/10)%10)) +
				string(rune('0'+(i+1)%10)),
		))
		result.WriteString(lineNum)
		result.WriteString(" ")
		result.WriteString(CodeStyle.Render(line))
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

package tui

import "github.com/charmbracelet/lipgloss"

// Color palette - matching the existing ui package
var (
	// Primary colors
	InfoBlue     = lipgloss.Color("#6C9BCF")
	SuccessGreen = lipgloss.Color("#7CB486")
	ErrorRed     = lipgloss.Color("#E07A7A")
	WarningAmber = lipgloss.Color("#D9A648")

	// UI colors
	BorderGray   = lipgloss.Color("#4A5568")
	MutedText    = lipgloss.Color("#718096")
	DimTextColor = lipgloss.Color("#A0AEC0")
	BGAccent     = lipgloss.Color("#2D3748")
	BGHighlight  = lipgloss.Color("#3D4A5C")
	BrightWhite  = lipgloss.Color("#F7FAFC")

	// Diff colors
	AddedGreen   = lipgloss.Color("#48BB78")
	RemovedRed   = lipgloss.Color("#FC8181")
	ChangedBlue  = lipgloss.Color("#63B3ED")
)

// Base styles
var (
	// Container styles
	AppStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrightWhite).
			Background(InfoBlue).
			Padding(0, 2).
			MarginBottom(1)

	// Panel styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderGray).
			Padding(0, 1)

	FocusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(InfoBlue).
				Padding(0, 1)

	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrightWhite).
			Background(BGAccent).
			Padding(0, 1)

	// List item styles
	ItemStyle = lipgloss.NewStyle().
			Foreground(MutedText).
			PaddingLeft(2)

	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(BrightWhite).
				Background(BGHighlight).
				Bold(true).
				PaddingLeft(1)

	// Status badges
	StatusNeedsResolution = lipgloss.NewStyle().
				Foreground(ErrorRed).
				Bold(true)

	StatusAutoMerge = lipgloss.NewStyle().
			Foreground(SuccessGreen)

	StatusResolved = lipgloss.NewStyle().
			Foreground(SuccessGreen).
			Bold(true)

	// Code styles
	CodeStyle = lipgloss.NewStyle().
			Foreground(DimTextColor)

	LineNumberStyle = lipgloss.NewStyle().
			Foreground(MutedText).
			Width(4).
			Align(lipgloss.Right).
			MarginRight(1)

	AddedLineStyle = lipgloss.NewStyle().
			Foreground(AddedGreen)

	RemovedLineStyle = lipgloss.NewStyle().
			Foreground(RemovedRed)

	// Help bar
	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedText).
			MarginTop(1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(InfoBlue).
			Bold(true)

	// Footer
	FooterStyle = lipgloss.NewStyle().
			Foreground(MutedText).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(BorderGray).
			PaddingTop(1).
			MarginTop(1)

	// Resolution option styles
	OptionStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Margin(0, 1)

	SelectedOptionStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Margin(0, 1).
				Background(InfoBlue).
				Foreground(BrightWhite).
				Bold(true)

	// File path style
	FilePathStyle = lipgloss.NewStyle().
			Foreground(InfoBlue).
			Bold(true)

	// Conflict type style
	ConflictTypeStyle = lipgloss.NewStyle().
				Foreground(WarningAmber)

	// Dim text style
	DimTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0AEC0"))
)

// Icons (Nerd Font)
const (
	IconFile       = "󰈙 "
	IconFunction   = "󰊕 "
	IconClass      = "󰌗 "
	IconCheck      = "✔ "
	IconCross      = "✘ "
	IconWarning    = "⚠ "
	IconArrowRight = "➜ "
	IconLocal      = "󰊢 "
	IconRemote     = "󰊣 "
	IconBase       = "󰊠 "
	IconBoth       = "󰄬 "
	IconEdit       = "󰏫 "
	IconMerge      = "⎇ "
)

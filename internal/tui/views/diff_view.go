package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ramsrib/kluster-compare/internal/diff"
	"github.com/ramsrib/kluster-compare/internal/model"
)

type DiffViewModel struct {
	pair     model.ResourcePair
	resType  model.ResourceType
	viewport viewport.Model
	ready    bool
	width    int
	height   int
	leftCtx  string
	rightCtx string
}

type DiffViewBackMsg struct{}

func (m DiffViewModel) Pair() model.ResourcePair {
	return m.pair
}

func (m DiffViewModel) TypeResource() string {
	return m.resType.Resource
}

func NewDiffViewModel(pair model.ResourcePair, resType model.ResourceType, width, height int, leftCtx, rightCtx string) DiffViewModel {
	m := DiffViewModel{
		pair:     pair,
		resType:  resType,
		width:    width,
		height:   height,
		leftCtx:  leftCtx,
		rightCtx: rightCtx,
	}
	m.initViewport()
	return m
}

func (m *DiffViewModel) initViewport() {
	headerHeight := 3
	footerHeight := 2

	vpHeight := m.height - headerHeight - footerHeight
	if vpHeight < 5 {
		vpHeight = 20
	}
	vpWidth := m.width
	if vpWidth < 40 {
		vpWidth = 80
	}

	m.viewport = viewport.New(vpWidth, vpHeight)
	m.viewport.Style = lipgloss.NewStyle().PaddingLeft(2)
	m.viewport.SetContent(m.renderContent())
	m.ready = true
}

func (m DiffViewModel) renderContent() string {
	switch m.pair.Status {
	case model.StatusOnlyInLeft:
		return m.renderOneSided(m.pair.Left, fmt.Sprintf("Only exists in %s", m.leftCtx), "-", "1")
	case model.StatusOnlyInRight:
		return m.renderOneSided(m.pair.Right, fmt.Sprintf("Only exists in %s", m.rightCtx), "+", "2")
	case model.StatusEqual:
		return m.renderEqual()
	case model.StatusChanged:
		return m.renderSideBySide()
	}
	return ""
}

func (m DiffViewModel) renderOneSided(res *model.Resource, title, prefix, color string) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(title))
	b.WriteString("\n\n")
	if res != nil {
		for _, line := range strings.Split(res.Normalized, "\n") {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(prefix + " " + line))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m DiffViewModel) renderEqual() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Resources are identical"))
	b.WriteString("\n\n")
	if m.pair.Left != nil {
		b.WriteString(m.pair.Left.Normalized)
	}
	return b.String()
}

func (m DiffViewModel) renderSideBySide() string {
	lines := diff.SideBySide(m.pair.DiffText)
	if len(lines) == 0 {
		return "No differences"
	}

	// Each side gets half the width, minus gutter (3 chars: " │ ")
	panelWidth := (m.width - 3) / 2
	if panelWidth < 30 {
		panelWidth = 30
	}
	lineNoWidth := 4

	contentWidth := panelWidth - lineNoWidth - 1 // 1 for space after line no

	gutter := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(" │ ")

	// Column headers
	leftName := m.leftCtx
	if leftName == "" {
		leftName = "LEFT"
	}
	rightName := m.rightCtx
	if rightName == "" {
		rightName = "RIGHT"
	}
	leftHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")).Render(pad(leftName, panelWidth))
	rightHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")).Render(pad(rightName, panelWidth))

	var b strings.Builder
	b.WriteString(leftHeader)
	b.WriteString(gutter)
	b.WriteString(rightHeader)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", panelWidth))
	b.WriteString("─┼─")
	b.WriteString(strings.Repeat("─", panelWidth))
	b.WriteString("\n")

	styleRemoved := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleAdded := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleContext := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleHunk := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	styleLineNo := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	bgRemoved := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#5c3040"))
	bgAdded := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#2a5030"))

	for _, dl := range lines {
		switch dl.Kind {
		case LineHunk:
			hunkText := pad(dl.Left, m.width)
			b.WriteString(styleHunk.Render(hunkText))
			b.WriteString("\n")

		case LineEqual:
			lno := styleLineNo.Render(fmt.Sprintf("%*d ", lineNoWidth, dl.LeftNo))
			rno := styleLineNo.Render(fmt.Sprintf("%*d ", lineNoWidth, dl.RightNo))
			lt := styleContext.Render(padTrunc(dl.Left, contentWidth))
			rt := styleContext.Render(padTrunc(dl.Right, contentWidth))
			b.WriteString(lno)
			b.WriteString(lt)
			b.WriteString(gutter)
			b.WriteString(rno)
			b.WriteString(rt)
			b.WriteString("\n")

		case LineChanged:
			lno := styleLineNo.Render(fmt.Sprintf("%*d ", lineNoWidth, dl.LeftNo))
			rno := styleLineNo.Render(fmt.Sprintf("%*d ", lineNoWidth, dl.RightNo))
			lt := bgRemoved.Render(padTrunc(dl.Left, contentWidth))
			rt := bgAdded.Render(padTrunc(dl.Right, contentWidth))
			b.WriteString(lno)
			b.WriteString(lt)
			b.WriteString(gutter)
			b.WriteString(rno)
			b.WriteString(rt)
			b.WriteString("\n")

		case LineRemoved:
			lno := styleLineNo.Render(fmt.Sprintf("%*d ", lineNoWidth, dl.LeftNo))
			lt := styleRemoved.Render(padTrunc(dl.Left, contentWidth))
			empty := strings.Repeat(" ", panelWidth)
			b.WriteString(lno)
			b.WriteString(lt)
			b.WriteString(gutter)
			b.WriteString(empty)
			b.WriteString("\n")

		case LineAdded:
			empty := strings.Repeat(" ", panelWidth)
			rno := styleLineNo.Render(fmt.Sprintf("%*d ", lineNoWidth, dl.RightNo))
			rt := styleAdded.Render(padTrunc(dl.Right, contentWidth))
			b.WriteString(empty)
			b.WriteString(gutter)
			b.WriteString(rno)
			b.WriteString(rt)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// pad right-pads a string to exactly width.
func pad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// padTrunc pads or truncates to exactly width.
func padTrunc(s string, width int) string {
	if width <= 0 {
		return ""
	}
	// Use rune-aware length for proper handling
	runes := []rune(s)
	if len(runes) > width {
		if width > 1 {
			return string(runes[:width-1]) + "…"
		}
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func (m DiffViewModel) Update(msg tea.Msg) (DiffViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width - 4
		m.height = msg.Height - 7
		m.initViewport()
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			return m, func() tea.Msg { return DiffViewBackMsg{} }
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m DiffViewModel) headerView() string {
	name := m.pair.Key
	ns := ""
	if parts := strings.SplitN(m.pair.Key, "/", 2); len(parts) == 2 {
		ns = parts[0]
		name = parts[1]
	}

	title := fmt.Sprintf("  %s: %s", m.resType.Name, name)
	if ns != "" {
		title += fmt.Sprintf(" (%s)", ns)
	}

	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(title)
}

func (m DiffViewModel) View() string {
	var b strings.Builder

	b.WriteString(m.headerView())
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Status: %s", m.pair.Status.String()))
	b.WriteString("\n")

	lineWidth := m.width
	if lineWidth < 60 {
		lineWidth = 60
	}
	b.WriteString("  " + strings.Repeat("─", lineWidth-4))
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	scrollPct := fmt.Sprintf("  %3.f%%", m.viewport.ScrollPercent()*100)
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(scrollPct))

	return b.String()
}

// Re-export LineKind constants so the view can reference diff.LineKind
type LineKind = diff.LineKind

const (
	LineEqual   = diff.LineEqual
	LineChanged = diff.LineChanged
	LineRemoved = diff.LineRemoved
	LineAdded   = diff.LineAdded
	LineHunk    = diff.LineHunk
)

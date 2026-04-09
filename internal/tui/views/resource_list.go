package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ramsrib/kluster-compare/internal/model"
)

type ResourceListModel struct {
	result    model.ComparisonResult
	filtered  []int // indices into result.Pairs
	cursor    int
	offset    int
	width     int
	height    int
	leftCtx   string
	rightCtx  string
	searching bool
	search    textinput.Model
}

func NewResourceListModel(result model.ComparisonResult, width, height int, leftCtx, rightCtx string) ResourceListModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 64
	ti.Prompt = "/ "

	m := ResourceListModel{
		result:   result,
		cursor:   0,
		width:    width,
		height:   height,
		leftCtx:  leftCtx,
		rightCtx: rightCtx,
		search:   ti,
	}
	m.applyFilter()
	return m
}

func (m *ResourceListModel) applyFilter() {
	query := strings.ToLower(m.search.Value())
	m.filtered = m.filtered[:0]
	for i, pair := range m.result.Pairs {
		if query == "" || strings.Contains(strings.ToLower(pair.Key), query) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	m.offset = 0
}

func (m ResourceListModel) IsSearching() bool {
	return m.searching
}

func (m ResourceListModel) SearchView() string {
	if !m.searching {
		return ""
	}
	return "  " + m.search.View()
}

type ResourceSelectedMsg struct {
	Pair model.ResourcePair
	Type model.ResourceType
}

type ResourceListBackMsg struct{}

func (m ResourceListModel) Update(msg tea.Msg) (ResourceListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width - 4
		m.height = msg.Height - 7

	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "esc":
				if m.search.Value() != "" {
					// First esc clears the filter
					m.search.SetValue("")
					m.applyFilter()
				} else {
					// Second esc exits search mode
					m.searching = false
					m.search.Blur()
				}
				return m, nil
			case "enter":
				m.searching = false
				m.search.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.applyFilter()
				return m, cmd
			}
		}

		// Normal mode
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
			m.searching = true
			m.search.Focus()
			return m, textinput.Blink
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				maxVisible := m.visibleRows()
				if m.cursor >= m.offset+maxVisible {
					m.offset = m.cursor - maxVisible + 1
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if m.cursor >= 0 && m.cursor < len(m.filtered) {
				pair := m.result.Pairs[m.filtered[m.cursor]]
				return m, func() tea.Msg {
					return ResourceSelectedMsg{Pair: pair, Type: m.result.Type}
				}
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			return m, func() tea.Msg { return ResourceListBackMsg{} }
		}
	}
	return m, nil
}

func (m ResourceListModel) visibleRows() int {
	extra := 5 // title + context + header + separator + padding
	rows := m.height - extra
	if rows < 5 {
		return 20
	}
	return rows
}

func (m ResourceListModel) View() string {
	var b strings.Builder

	totalShown := len(m.filtered)
	total := len(m.result.Pairs)
	if m.search.Value() != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).
			Render(fmt.Sprintf("  %s (%d/%d)", m.result.Type.Name, totalShown, total)))
	} else {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).
			Render(fmt.Sprintf("  %s (%d total)", m.result.Type.Name, total)))
	}
	b.WriteString("\n")

	leftStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	rightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	b.WriteString("  " + leftStyle.Render("Left: "+m.leftCtx) + "  " + rightStyle.Render("Right: "+m.rightCtx))
	b.WriteString("\n")

	b.WriteString("\n")

	// Compute column widths
	statusCol := 20
	nsCol := 25
	nameCol := m.width - statusCol - nsCol - 6
	if nameCol < 20 {
		nameCol = 20
	}
	if nameCol > 60 {
		nameCol = 60
	}
	lineWidth := m.width
	if lineWidth < nameCol+nsCol+statusCol+6 {
		lineWidth = nameCol + nsCol + statusCol + 6
	}

	headerFmt := fmt.Sprintf("  %%-%ds %%-%ds %%-%ds", nameCol, nsCol, statusCol)
	header := fmt.Sprintf(headerFmt, "Name", "Namespace", "Status")
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render(header))
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("─", lineWidth-4))
	b.WriteString("\n")

	maxVisible := m.visibleRows()
	end := m.offset + maxVisible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	rowFmt := fmt.Sprintf("  %%-%ds %%-%ds %%-%ds", nameCol, nsCol, statusCol)

	for vi := m.offset; vi < end; vi++ {
		pair := m.result.Pairs[m.filtered[vi]]

		name := pair.Key
		ns := ""
		if parts := strings.SplitN(pair.Key, "/", 2); len(parts) == 2 {
			ns = parts[0]
			name = parts[1]
		}

		statusStr := m.statusLabel(pair.Status)
		line := fmt.Sprintf(rowFmt, truncate(name, nameCol), truncate(ns, nsCol), statusStr)

		visible := lipgloss.Width(line)
		if visible < lineWidth {
			line += strings.Repeat(" ", lineWidth-visible)
		}

		if vi == m.cursor {
			line = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("12")).
				Render(line)
		} else {
			var style lipgloss.Style
			switch pair.Status {
			case model.StatusEqual:
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
			case model.StatusChanged:
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
			case model.StatusOnlyInLeft:
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
			case model.StatusOnlyInRight:
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
			}
			line = style.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.filtered) > maxVisible {
		info := fmt.Sprintf("  showing %d-%d of %d", m.offset+1, end, len(m.filtered))
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(info))
		b.WriteString("\n")
	}

	return b.String()
}

func shortCtx(ctx string, maxLen int) string {
	if len(ctx) <= maxLen {
		return ctx
	}
	return ".." + ctx[len(ctx)-maxLen+2:]
}

func (m ResourceListModel) statusLabel(s model.DiffStatus) string {
	switch s {
	case model.StatusEqual:
		return "EQUAL"
	case model.StatusChanged:
		return "CHANGED"
	case model.StatusOnlyInLeft:
		return "ONLY " + shortCtx(m.leftCtx, 14)
	case model.StatusOnlyInRight:
		return "ONLY " + shortCtx(m.rightCtx, 14)
	default:
		return "UNKNOWN"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

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

type ResourceTypeListModel struct {
	results      []model.ComparisonResult
	filtered     []int // indices into results
	coreCount    int   // number of core (static) types — discovered types start after this
	cursor       int
	offset       int
	width        int
	height       int
	leftCtx      string
	rightCtx     string
	hideEmpty    bool
	searching    bool
	search       textinput.Model
}

func NewResourceTypeListModel(results []model.ComparisonResult, width, height int, leftCtx, rightCtx string) ResourceTypeListModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 64
	ti.Prompt = "/ "

	m := ResourceTypeListModel{
		results:   results,
		coreCount: len(results),
		cursor:    0,
		width:     width,
		height:    height,
		leftCtx:   leftCtx,
		rightCtx:  rightCtx,
		hideEmpty: true,
		search:    ti,
	}
	m.applyFilter()
	return m
}

func (m *ResourceTypeListModel) applyFilter() {
	query := strings.ToLower(m.search.Value())
	m.filtered = m.filtered[:0]

	// Core types first (indices 0..coreCount-1)
	for i := 0; i < m.coreCount && i < len(m.results); i++ {
		if m.shouldShow(m.results[i], query) {
			m.filtered = append(m.filtered, i)
		}
	}

	// Discovered types after (indices coreCount..)
	if m.coreCount < len(m.results) {
		for i := m.coreCount; i < len(m.results); i++ {
			if m.shouldShow(m.results[i], query) {
				m.filtered = append(m.filtered, i)
			}
		}
	}

	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m *ResourceTypeListModel) shouldShow(r model.ComparisonResult, query string) bool {
	if r.Loading {
		return false
	}
	if m.hideEmpty && r.LeftCount == 0 && r.RightCount == 0 && r.Error == "" {
		return false
	}
	if query != "" && !strings.Contains(strings.ToLower(r.Type.Name), query) {
		return false
	}
	return true
}

// UpdateResult updates a single result in-place and reapplies the filter.
func (m *ResourceTypeListModel) UpdateResult(index int, result model.ComparisonResult) {
	if index >= 0 && index < len(m.results) {
		m.results[index] = result
		m.applyFilter()
	}
}

// AppendResults adds new results (from discovery) to the list.
func (m *ResourceTypeListModel) AppendResults(results []model.ComparisonResult) {
	m.results = append(m.results, results...)
	m.applyFilter()
}

func (m ResourceTypeListModel) IsSearching() bool {
	return m.searching
}

func (m ResourceTypeListModel) SearchView() string {
	if !m.searching {
		return ""
	}
	return "  " + m.search.View()
}

type ResourceTypeSelectedMsg struct {
	Result model.ComparisonResult
}

func (m ResourceTypeListModel) visibleRows() int {
	rows := m.height - 4 // context line + header + separator + padding
	if rows < 5 {
		return 20
	}
	return rows
}

func (m ResourceTypeListModel) Update(msg tea.Msg) (ResourceTypeListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width - 4
		m.height = msg.Height - 7

	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "esc":
				if m.search.Value() != "" {
					m.search.SetValue("")
					m.applyFilter()
				} else {
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

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
			m.searching = true
			m.search.Focus()
			return m, textinput.Blink
		case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
			m.hideEmpty = !m.hideEmpty
			m.applyFilter()
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
				return m, func() tea.Msg {
					return ResourceTypeSelectedMsg{Result: m.results[m.filtered[m.cursor]]}
				}
			}
		}
	}
	return m, nil
}

func (m ResourceTypeListModel) View() string {
	var b strings.Builder

	// Cluster context names
	leftStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	rightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	b.WriteString("  " + leftStyle.Render(m.leftCtx) + "  vs  " + rightStyle.Render(m.rightCtx))
	b.WriteString("\n\n")

	// Column widths
	nameCol := 28
	numCol := 8
	tableWidth := nameCol + numCol*5 + 6
	if m.width > tableWidth+10 {
		nameCol = m.width - numCol*5 - 6
		if nameCol > 35 {
			nameCol = 35
		}
	}
	lineWidth := m.width
	if lineWidth < tableWidth {
		lineWidth = tableWidth
	}

	// Table header
	fmtStr := fmt.Sprintf("  %%-%ds %%%dd %%%dd %%%dd %%%dd %%%dd", nameCol, numCol, numCol, numCol, numCol, numCol)
	headerFmt := fmt.Sprintf("  %%-%ds %%%ds %%%ds %%%ds %%%ds %%%ds", nameCol, numCol, numCol, numCol, numCol, numCol)

	// Count loading vs done
	loadingCount := 0
	for _, r := range m.results {
		if r.Loading {
			loadingCount++
		}
	}

	totalLabel := ""
	if loadingCount > 0 {
		totalLabel = fmt.Sprintf("loading %d/%d types...", len(m.results)-loadingCount, len(m.results))
	} else {
		totalLabel = fmt.Sprintf("%d types", len(m.filtered))
		if m.hideEmpty {
			totalLabel += " (e: show empty)"
		} else {
			totalLabel += " (e: hide empty)"
		}
	}
	b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(totalLabel))
	b.WriteString("\n")

	header := fmt.Sprintf(headerFmt, "Resource Type", "Left", "Right", "Diff", "Only-L", "Only-R")
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render(header))
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("─", lineWidth-4))
	b.WriteString("\n")

	maxVisible := m.visibleRows()
	end := m.offset + maxVisible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	shownDiscoveredHeader := false
	for vi := m.offset; vi < end; vi++ {
		idx := m.filtered[vi]

		// Show separator when entering discovered section
		if !shownDiscoveredHeader && idx >= m.coreCount && m.coreCount < len(m.results) {
			shownDiscoveredHeader = true
			sep := "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("── Discovered CRDs ──")
			b.WriteString(sep)
			b.WriteString("\n")
		}

		result := m.results[idx]
		_, changed, onlyLeft, onlyRight := result.Counts()

		line := fmt.Sprintf(fmtStr,
			result.Type.Name,
			result.LeftCount,
			result.RightCount,
			changed,
			onlyLeft,
			onlyRight,
		)

		if result.Error != "" {
			line = fmt.Sprintf("  %-*s %s", nameCol, result.Type.Name,
				lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("ERROR: "+result.Error))
		}

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
		} else if result.Error != "" {
			// already styled
		} else if result.Loading {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(line)
		} else if changed > 0 || onlyLeft > 0 || onlyRight > 0 {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(line)
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

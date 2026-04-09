package views

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LoadingModel struct {
	spinner spinner.Model
	message string
}

func NewLoadingModel() LoadingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	return LoadingModel{
		spinner: s,
		message: "Fetching resources from both clusters...",
	}
}

func (m LoadingModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m LoadingModel) Update(msg tea.Msg) (LoadingModel, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m LoadingModel) View() string {
	return fmt.Sprintf("  %s %s", m.spinner.View(), m.message)
}

func (m *LoadingModel) SetMessage(msg string) {
	m.message = msg
}

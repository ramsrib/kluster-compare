package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/client-go/dynamic"
	"github.com/ramsrib/kluster-compare/internal/kube"
	"github.com/ramsrib/kluster-compare/internal/model"
	"github.com/ramsrib/kluster-compare/internal/tui/views"
)

type viewID int

const (
	viewLoading viewID = iota
	viewResourceTypes
	viewResourceList
	viewDiff
)

const headerLines = 2 // title + blank line before content

type fetchCompleteMsg struct {
	results []model.ComparisonResult
	err     error
}

type fetchOneResultMsg struct {
	index  int
	result model.ComparisonResult
}

type fetchAllDoneMsg struct{}

type clearCopiedMsg struct{}

type discoveryCompleteMsg struct {
	newResults []model.ComparisonResult
	newTypes   []model.ResourceType
}

type App struct {
	activeView viewID

	// Views
	loadingView      views.LoadingModel
	typeListView     views.ResourceTypeListModel
	resourceListView views.ResourceListModel
	diffView         views.DiffViewModel

	// Config
	leftContext     string
	rightContext    string
	leftClient     dynamic.Interface
	rightClient    dynamic.Interface
	registry       *model.Registry
	namespaces     []string
	kubeconfigPath string

	// State
	results       []model.ComparisonResult
	resultCh      chan tea.Msg
	doneCount     int
	totalTypes    int
	width         int
	height        int
	ready         bool // true after first WindowSizeMsg
	discovering   bool // true while CRD discovery is in progress
	discovered    bool // true after discovery has been done
	copiedMsg     string
	err           error
}

func NewApp(
	leftCtx, rightCtx string,
	leftClient, rightClient dynamic.Interface,
	registry *model.Registry,
	namespaces []string,
	kubeconfigPath string,
) App {
	types := registry.All()
	results := make([]model.ComparisonResult, len(types))
	for i, rt := range types {
		results[i] = model.ComparisonResult{Type: rt, Loading: true}
	}

	resultCh := make(chan tea.Msg, len(types)+1)

	return App{
		activeView:     viewResourceTypes,
		loadingView:    views.NewLoadingModel(),
		leftContext:     leftCtx,
		rightContext:    rightCtx,
		leftClient:     leftClient,
		rightClient:    rightClient,
		registry:       registry,
		namespaces:     namespaces,
		kubeconfigPath: kubeconfigPath,
		results:        results,
		resultCh:       resultCh,
		totalTypes:     len(types),
		typeListView:   views.NewResourceTypeListModel(results, 80, 30, leftCtx, rightCtx),
	}
}

func (a App) Init() tea.Cmd {
	// Start fetching in background
	ch := a.resultCh
	leftClient := a.leftClient
	rightClient := a.rightClient
	registry := a.registry
	namespaces := a.namespaces

	go func() {
		kube.FetchAndCompareStreaming(
			context.Background(),
			leftClient,
			rightClient,
			registry,
			namespaces,
			func(index int, result model.ComparisonResult) {
				ch <- fetchOneResultMsg{index: index, result: result}
			},
		)
		ch <- fetchAllDoneMsg{}
	}()

	return waitForResult(ch)
}

func waitForResult(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (a App) discoverCustomTypes() tea.Cmd {
	return func() tea.Msg {
		discovered, err := kube.DiscoverResourceTypes(a.kubeconfigPath, a.leftContext, a.rightContext)
		if err != nil {
			return discoveryCompleteMsg{} // silently fail, keep static types
		}

		// Find types not already in our registry
		existing := make(map[string]bool)
		for _, rt := range a.registry.All() {
			existing[rt.Group+"/"+rt.Version+"/"+rt.Resource] = true
		}

		var newTypes []model.ResourceType
		var newResults []model.ComparisonResult
		for _, rt := range discovered.All() {
			k := rt.Group + "/" + rt.Version + "/" + rt.Resource
			if !existing[k] {
				newTypes = append(newTypes, rt)
				newResults = append(newResults, model.ComparisonResult{Type: rt, Loading: true})
			}
		}

		return discoveryCompleteMsg{newResults: newResults, newTypes: newTypes}
	}
}

// contentWidth returns usable width for child views (minus left/right padding).
func (a App) contentWidth() int {
	w := a.width - 4 // 2 padding each side
	if w < 40 {
		return 80
	}
	return w
}

// contentHeight returns usable height for child views (minus header, status bar, padding).
func (a App) contentHeight() int {
	h := a.height - headerLines - 3 // header + status bar + padding
	if h < 10 {
		return 30
	}
	return h
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		// Forward to active view
		return a, a.forwardToActiveView(msg)

	case tea.KeyMsg:
		// Don't quit when typing in search
		isSearching := a.resourceListView.IsSearching() || a.typeListView.IsSearching()
		if key.Matches(msg, keys.Quit) && !isSearching {
			return a, tea.Quit
		}
		// Discover custom resource types on demand
		if key.Matches(msg, key.NewBinding(key.WithKeys("d"))) &&
			a.activeView == viewResourceTypes && !isSearching && !a.discovering && !a.discovered {
			a.discovering = true
			return a, a.discoverCustomTypes()
		}
		// Copy CLI diff command to clipboard
		if key.Matches(msg, key.NewBinding(key.WithKeys("c"))) && !isSearching {
			var cmd string
			switch a.activeView {
			case viewResourceList:
				cmd = a.cliDiffCmd()
			case viewDiff:
				cmd = a.cliDiffCmdForDiffView()
			}
			if cmd != "" {
				if copyToClipboard(cmd) == nil {
					a.copiedMsg = "Copied!"
					return a, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
						return clearCopiedMsg{}
					})
				}
			}
		}

	case clearCopiedMsg:
		a.copiedMsg = ""
		return a, nil

	case discoveryCompleteMsg:
		a.discovering = false
		a.discovered = true
		// Append new types to results
		baseLen := len(a.results)
		a.results = append(a.results, msg.newResults...)
		a.totalTypes = len(a.results)
		// Create a new channel for the new fetches
		a.resultCh = make(chan tea.Msg, len(msg.newResults)+1)
		a.typeListView.AppendResults(msg.newResults)
		// Start fetching the new types
		go func() {
			newReg := &model.Registry{}
			for _, rt := range msg.newTypes {
				newReg.Register(rt)
			}
			kube.FetchAndCompareStreaming(
				context.Background(),
				a.leftClient, a.rightClient,
				newReg, a.namespaces,
				func(index int, result model.ComparisonResult) {
					a.resultCh <- fetchOneResultMsg{index: baseLen + index, result: result}
				},
			)
			a.resultCh <- fetchAllDoneMsg{}
		}()
		return a, waitForResult(a.resultCh)

	case fetchOneResultMsg:
		a.results[msg.index] = msg.result
		a.doneCount++
		a.typeListView.UpdateResult(msg.index, msg.result)
		// Keep listening for more
		return a, waitForResult(a.resultCh)

	case fetchAllDoneMsg:
		// All done, no more listening needed
		return a, nil

	case fetchCompleteMsg:
		// Legacy path for non-streaming (summary mode)
		if msg.err != nil {
			a.err = msg.err
			return a, tea.Quit
		}
		a.results = msg.results
		a.typeListView = views.NewResourceTypeListModel(a.results, a.contentWidth(), a.contentHeight(), a.leftContext, a.rightContext)
		a.activeView = viewResourceTypes
		return a, nil

	case views.ResourceTypeSelectedMsg:
		a.resourceListView = views.NewResourceListModel(msg.Result, a.contentWidth(), a.contentHeight(), a.leftContext, a.rightContext)
		a.activeView = viewResourceList
		return a, nil

	case views.ResourceSelectedMsg:
		a.diffView = views.NewDiffViewModel(msg.Pair, msg.Type, a.contentWidth(), a.contentHeight(), a.leftContext, a.rightContext)
		a.activeView = viewDiff
		return a, nil

	case views.ResourceListBackMsg:
		a.activeView = viewResourceTypes
		return a, nil

	case views.DiffViewBackMsg:
		a.activeView = viewResourceList
		return a, nil
	}

	// Delegate to active view
	cmd := a.forwardToActiveView(msg)
	return a, cmd
}

func (a *App) forwardToActiveView(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch a.activeView {
	case viewLoading:
		a.loadingView, cmd = a.loadingView.Update(msg)
	case viewResourceTypes:
		a.typeListView, cmd = a.typeListView.Update(msg)
	case viewResourceList:
		a.resourceListView, cmd = a.resourceListView.Update(msg)
	case viewDiff:
		a.diffView, cmd = a.diffView.Update(msg)
	}
	return cmd
}

func (a App) View() string {
	// Don't render until we know the terminal size
	if !a.ready {
		return ""
	}

	header := lipgloss.NewStyle().Bold(true).Render("kluster-compare")

	var content string
	if a.err != nil {
		content = StyleError.Render(fmt.Sprintf("Error: %v", a.err))
	} else {
		switch a.activeView {
		case viewLoading:
			content = a.loadingView.View()
		case viewResourceTypes:
			content = a.typeListView.View()
		case viewResourceList:
			content = a.resourceListView.View()
		case viewDiff:
			content = a.diffView.View()
		}
	}

	inner := header + "\n\n" + content

	// Search bar sits just above the status bar
	searchBar := ""
	switch a.activeView {
	case viewResourceTypes:
		searchBar = a.typeListView.SearchView()
	case viewResourceList:
		searchBar = a.resourceListView.SearchView()
	}

	help := a.helpText()
	bar := StyleStatusBar.Width(a.width).Render(help)

	bottomLines := 1
	if searchBar != "" {
		bottomLines = 2
	}

	mainHeight := a.height - bottomLines
	main := lipgloss.Place(
		a.width, mainHeight,
		lipgloss.Left, lipgloss.Top,
		lipgloss.NewStyle().Padding(0, 2).Render(inner),
	)

	if searchBar != "" {
		return main + "\n" + searchBar + "\n" + bar
	}
	return main + "\n" + bar
}

func (a App) buildDiffCmd(pair *model.ResourcePair, typeName string) string {
	if pair == nil {
		return ""
	}

	var leftName, rightName, ns string
	if pair.Left != nil {
		leftName = pair.Left.Name
		ns = pair.Left.Namespace
	}
	if pair.Right != nil {
		rightName = pair.Right.Name
		if ns == "" {
			ns = pair.Right.Namespace
		}
	}
	if leftName == "" {
		leftName = rightName
	}
	if rightName == "" {
		rightName = leftName
	}

	resArg := typeName + "/" + leftName
	if leftName != rightName {
		resArg = typeName + "/" + leftName + ":" + rightName
	}

	cmd := fmt.Sprintf("kluster-compare %s %s %s", a.leftContext, a.rightContext, resArg)
	if ns != "" {
		cmd += " -n " + ns
	}
	return cmd
}

func (a App) cliDiffCmd() string {
	pair := a.resourceListView.SelectedPair()
	if pair == nil || pair.Status == model.StatusEqual {
		return ""
	}
	return a.buildDiffCmd(pair, a.resourceListView.TypeName())
}

func (a App) cliDiffCmdForDiffView() string {
	pair := a.diffView.Pair()
	return a.buildDiffCmd(&pair, a.diffView.TypeResource())
}

func (a App) formatCmdLine(cmd string) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	if a.copiedMsg != "" {
		return "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(a.copiedMsg) + " " + dimStyle.Render(cmd)
	}
	return "  " + dimStyle.Render("c: copy  "+cmd)
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return fmt.Errorf("unsupported OS")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (a App) helpText() string {
	switch a.activeView {
	case viewResourceTypes:
		help := " j/k: navigate  /: filter  e: toggle empty  enter: select  q: quit"
		if a.discovering {
			help += "  (discovering...)"
		} else if !a.discovered {
			help += "  d: discover CRDs"
		}
		return help
	case viewResourceList:
		help := " j/k: navigate  /: filter  enter: view diff  esc: back  q: quit"
		if a.copiedMsg != "" {
			help += "  Copied!"
		} else {
			help += "  c: copy cmd"
		}
		return help
	case viewDiff:
		help := " j/k: scroll  esc: back  q: quit"
		if a.copiedMsg != "" {
			help += "  Copied!"
		} else {
			help += "  c: copy cmd"
		}
		return help
	default:
		return ""
	}
}


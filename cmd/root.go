package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"github.com/ramsrib/kluster-compare/internal/kube"
	"github.com/ramsrib/kluster-compare/internal/model"
	"github.com/ramsrib/kluster-compare/internal/normalize"
	"github.com/ramsrib/kluster-compare/internal/tui"

	pkgdiff "github.com/ramsrib/kluster-compare/internal/diff"
)

var (
	kubeconfig    string
	namespace     string
	resourceTypes []string
	summaryFlag   bool
	discoverTypes bool
)

var rootCmd = &cobra.Command{
	Use:   "kluster-compare <context-a> <context-b> [type/name]",
	Short: "Compare Kubernetes resources between two clusters",
	Long: `Compare Kubernetes resources between two clusters and view diffs.

Modes:
  Interactive TUI (default):
    kluster-compare ctx-a ctx-b

  Non-interactive summary:
    kluster-compare ctx-a ctx-b -s

  Specific resource diff (kubectl-style):
    kluster-compare ctx-a ctx-b deployment/api-server -n app-namespace
    kluster-compare ctx-a ctx-b namespace/kube-system
    kluster-compare ctx-a ctx-b service/my-svc -n default

  Filter by types or namespaces:
    kluster-compare ctx-a ctx-b -t deployments,services -n app-namespace`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctxA, ctxB := args[0], args[1]

		clientA, err := kube.NewDynamicClient(kubeconfig, ctxA)
		if err != nil {
			return fmt.Errorf("failed to create client for %s: %w", ctxA, err)
		}
		clientB, err := kube.NewDynamicClient(kubeconfig, ctxB)
		if err != nil {
			return fmt.Errorf("failed to create client for %s: %w", ctxB, err)
		}

		// Specific resource diff: kluster-compare ctx-a ctx-b type/name
		if len(args) == 3 {
			return runDiff(ctxA, ctxB, args[2], clientA, clientB)
		}

		registry := model.DefaultRegistry()
		if len(resourceTypes) > 0 {
			registry = registry.Filter(resourceTypes)
		}

		// Non-interactive summary
		if summaryFlag {
			if discoverTypes {
				fmt.Fprintln(os.Stderr, "Discovering resource types...")
				discovered, err := kube.DiscoverResourceTypes(kubeconfig, ctxA, ctxB)
				if err == nil {
					registry = discovered
				}
				if len(resourceTypes) > 0 {
					registry = registry.Filter(resourceTypes)
				}
			}
			return runSummary(ctxA, ctxB, clientA, clientB, registry)
		}

		// Interactive TUI — pass kubeconfig so it can discover on demand
		namespaces := splitNamespaces(namespace)
		app := tui.NewApp(ctxA, ctxB, clientA, clientB, registry, namespaces, kubeconfig)
		p := tea.NewProgram(app, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	defaultKubeconfig := filepath.Join(homeDir(), ".kube", "config")
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig, "path to kubeconfig file")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace to filter (or namespace of specific resource)")
	rootCmd.Flags().StringSliceVarP(&resourceTypes, "types", "t", nil, "resource types to compare (default: all)")
	rootCmd.Flags().BoolVarP(&summaryFlag, "summary", "s", false, "non-interactive summary table")
	rootCmd.Flags().BoolVar(&discoverTypes, "discover", false, "auto-discover all resource types including CRDs")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}

// --- Specific resource diff ---

func runDiff(ctxA, ctxB, resourceArg string, clientA, clientB dynamic.Interface) error {
	typeName, resName, err := parseResourceArg(resourceArg)
	if err != nil {
		return err
	}

	// Try static registry first, then discovery
	resType := resolveResourceType(typeName, model.DefaultRegistry())
	if resType == nil {
		discovered, err := kube.DiscoverResourceTypes(kubeconfig, ctxA, ctxB)
		if err == nil {
			resType = resolveResourceType(typeName, discovered)
		}
	}
	if resType == nil {
		return fmt.Errorf("unknown resource type %q -- run 'kluster-compare types' to see available types", typeName)
	}

	if resType.Namespaced && namespace == "" {
		return fmt.Errorf("%s is namespaced -- use -n to specify namespace", resType.Name)
	}

	fetcher := kube.NewDynamicFetcher(*resType)
	ctx := context.Background()

	var ns []string
	if namespace != "" {
		ns = []string{namespace}
	}

	leftResources, errL := fetcher.Fetch(ctx, clientA, ns)
	rightResources, errR := fetcher.Fetch(ctx, clientB, ns)

	left := findByName(leftResources, resName)
	right := findByName(rightResources, resName)

	key := resName
	if namespace != "" {
		key = namespace + "/" + resName
	}

	if left == nil && right == nil {
		msg := fmt.Sprintf("%s %q not found in either cluster", resType.Name, key)
		if errL != nil {
			msg += fmt.Sprintf("\n  left error: %v", errL)
		}
		if errR != nil {
			msg += fmt.Sprintf("\n  right error: %v", errR)
		}
		return fmt.Errorf("%s", msg)
	}

	if left == nil {
		fmt.Printf("\033[1m%s %s\033[0m\n", resType.Name, key)
		fmt.Printf("  \033[31mLEFT  (%s): not found\033[0m\n", ctxA)
		fmt.Printf("  \033[32mRIGHT (%s): exists\033[0m\n\n", ctxB)
		printPrefixed(right.Normalized, "+", "32")
		return nil
	}

	if right == nil {
		fmt.Printf("\033[1m%s %s\033[0m\n", resType.Name, key)
		fmt.Printf("  \033[32mLEFT  (%s): exists\033[0m\n", ctxA)
		fmt.Printf("  \033[31mRIGHT (%s): not found\033[0m\n\n", ctxB)
		printPrefixed(left.Normalized, "-", "31")
		return nil
	}

	leftNorm := normalize.Normalize(left.Raw)
	rightNorm := normalize.Normalize(right.Raw)

	d := pkgdiff.Compute("left/"+key, "right/"+key, leftNorm, rightNorm)
	if d == "" {
		fmt.Printf("\033[1m%s %s\033[0m\n", resType.Name, key)
		fmt.Printf("  LEFT:  %s\n  RIGHT: %s\n\n", ctxA, ctxB)
		fmt.Println("\033[32mResources are identical.\033[0m")
		return nil
	}

	fmt.Printf("\033[1m%s %s\033[0m\n", resType.Name, key)
	fmt.Printf("  \033[31mLEFT:  %s\033[0m\n  \033[32mRIGHT: %s\033[0m\n\n", ctxA, ctxB)
	printColorDiff(d)
	return nil
}

// --- Non-interactive summary ---

func runSummary(ctxA, ctxB string, clientA, clientB dynamic.Interface, registry *model.Registry) error {
	fmt.Printf("\033[1mkluster-compare\033[0m\n")
	fmt.Printf("  \033[31mLeft:  %s\033[0m\n", ctxA)
	fmt.Printf("  \033[32mRight: %s\033[0m\n\n", ctxB)

	fmt.Println("Fetching resources...")
	namespaces := splitNamespaces(namespace)
	results := kube.FetchAndCompare(context.Background(), clientA, clientB, registry, namespaces)

	fmt.Printf("\n%-20s %6s %6s %6s %8s %8s\n", "Resource Type", "Left", "Right", "Diff", "Only-L", "Only-R")
	fmt.Println(strings.Repeat("─", 62))

	totalChanged, totalOnlyL, totalOnlyR := 0, 0, 0
	for _, r := range results {
		if r.Error != "" {
			fmt.Printf("%-20s \033[31mERROR: %s\033[0m\n", r.Type.Name, r.Error)
			continue
		}
		_, changed, onlyLeft, onlyRight := r.Counts()
		totalChanged += changed
		totalOnlyL += onlyLeft
		totalOnlyR += onlyRight

		color, reset := "", ""
		if changed > 0 || onlyLeft > 0 || onlyRight > 0 {
			color = "\033[33m"
			reset = "\033[0m"
		}
		fmt.Printf("%s%-20s %6d %6d %6d %8d %8d%s\n",
			color, r.Type.Name, r.LeftCount, r.RightCount, changed, onlyLeft, onlyRight, reset)
	}

	fmt.Println(strings.Repeat("─", 62))
	if totalChanged == 0 && totalOnlyL == 0 && totalOnlyR == 0 {
		fmt.Println("\033[32mClusters are in sync.\033[0m")
	} else {
		fmt.Printf("\033[33m%d changed, %d only in left, %d only in right\033[0m\n", totalChanged, totalOnlyL, totalOnlyR)
	}

	for _, r := range results {
		if r.Error != "" {
			continue
		}
		for _, p := range r.Pairs {
			switch p.Status {
			case model.StatusChanged:
				fmt.Printf("\n  \033[33mCHANGED\033[0m  %s/%s", r.Type.Name, p.Key)
			case model.StatusOnlyInLeft:
				fmt.Printf("\n  \033[31mONLY-L\033[0m   %s/%s", r.Type.Name, p.Key)
			case model.StatusOnlyInRight:
				fmt.Printf("\n  \033[32mONLY-R\033[0m   %s/%s", r.Type.Name, p.Key)
			}
		}
	}
	fmt.Println()

	return nil
}

// --- Helpers ---

// parseResourceArg parses "type/name" into (type, name).
func parseResourceArg(arg string) (string, string, error) {
	parts := strings.SplitN(arg, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("resource must be in type/name format (e.g. deployment/api-server), got %q", arg)
	}
	return parts[0], parts[1], nil
}

// resolveResourceType finds a ResourceType by name, plural, or singular (case-insensitive).
func resolveResourceType(name string, registry *model.Registry) *model.ResourceType {
	lower := strings.ToLower(name)
	for _, rt := range registry.All() {
		if strings.ToLower(rt.Name) == lower ||
			strings.ToLower(rt.Resource) == lower ||
			strings.TrimSuffix(strings.ToLower(rt.Resource), "s") == lower {
			t := rt
			return &t
		}
	}
	return nil
}

func findByName(resources []model.Resource, name string) *model.Resource {
	for i := range resources {
		if resources[i].Name == name {
			return &resources[i]
		}
	}
	return nil
}

func splitNamespaces(ns string) []string {
	if ns == "" {
		return nil
	}
	parts := strings.Split(ns, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func printColorDiff(d string) {
	for _, line := range strings.Split(d, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			fmt.Printf("\033[1m%s\033[0m\n", line)
		case strings.HasPrefix(line, "@@"):
			fmt.Printf("\033[36m%s\033[0m\n", line)
		case strings.HasPrefix(line, "+"):
			fmt.Printf("\033[32m%s\033[0m\n", line)
		case strings.HasPrefix(line, "-"):
			fmt.Printf("\033[31m%s\033[0m\n", line)
		default:
			fmt.Println(line)
		}
	}
}

func printPrefixed(yaml, prefix, colorCode string) {
	for _, line := range strings.Split(yaml, "\n") {
		fmt.Printf("\033[%sm%s %s\033[0m\n", colorCode, prefix, line)
	}
}

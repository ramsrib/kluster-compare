package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ramsrib/kluster-compare/internal/model"
)

var typesCmd = &cobra.Command{
	Use:   "types",
	Short: "List supported resource types",
	Run: func(cmd *cobra.Command, args []string) {
		registry := model.DefaultRegistry()
		fmt.Println("Supported resource types:")
		fmt.Println()
		for _, rt := range registry.All() {
			group := rt.Group
			if group == "" {
				group = "core"
			}
			scope := "namespaced"
			if !rt.Namespaced {
				scope = "cluster"
			}
			fmt.Printf("  %-20s %s/%s (%s)\n", rt.Name, group, rt.Version, scope)
		}
		fmt.Printf("\nUse --resource-types / -t to filter: kluster-compare compare ctx-a ctx-b -t Deployments,Services\n")
	},
}

func init() {
	rootCmd.AddCommand(typesCmd)
}

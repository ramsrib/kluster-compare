package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var contextsCmd = &cobra.Command{
	Use:   "contexts",
	Short: "List available Kubernetes contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := clientcmd.LoadFromFile(kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to load kubeconfig: %w", err)
		}

		current := config.CurrentContext
		for name := range config.Contexts {
			marker := "  "
			if name == current {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, name)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(contextsCmd)
}

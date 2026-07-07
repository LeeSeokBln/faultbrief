package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	root := &cobra.Command{
		Use:           "faultbrief",
		Short:         "Turn Linux logs into an incident brief",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("faultbrief", version)
		},
	})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "faultbrief:", err)
		os.Exit(2)
	}
}

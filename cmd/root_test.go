package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveFormat(t *testing.T) {
	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{}
		cmd.Flags().StringP("format", "f", "table", "")
		cmd.Flags().Bool("json", false, "")
		return cmd
	}

	tests := []struct {
		name   string
		setup  func(*cobra.Command)
		want   string
	}{
		{
			"default is table",
			func(cmd *cobra.Command) {},
			"table",
		},
		{
			"format flag json",
			func(cmd *cobra.Command) { cmd.Flags().Set("format", "json") },
			"json",
		},
		{
			"format flag csv",
			func(cmd *cobra.Command) { cmd.Flags().Set("format", "csv") },
			"csv",
		},
		{
			"json flag overrides format",
			func(cmd *cobra.Command) {
				cmd.Flags().Set("format", "csv")
				cmd.Flags().Set("json", "true")
			},
			"json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCmd()
			tt.setup(cmd)
			got := resolveFormat(cmd)
			if got != tt.want {
				t.Errorf("resolveFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

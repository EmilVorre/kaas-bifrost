package commands

import (
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var logger *zap.Logger

var rootCmd = &cobra.Command{
	Use:   "bifrost",
	Short: "KaaS Bifrost - Kubernetes multi-tenancy platform CLI",
	Long: `
██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗
██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝
██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║
██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║
██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║
╚═════╝ ╚═╝╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚══════╝   ╚═╝
 
KaaS Bifrost — The bridge between your infrastructure and your tenants.
Manage cluster provisioning, tenant isolation, secrets, and observability.`,
	SilenceUsage: true,
}

// Execute is the entry point called from main.go
func Execute() {
	var err error

	logger, err = zap.NewProduction()
	if err != nil {
		panic("failed to initialise logger: " + err.Error())
	}
	defer func() {
		_ = logger.Sync()
	}()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(tenantCmd)
	rootCmd.AddCommand(statusCmd)
}

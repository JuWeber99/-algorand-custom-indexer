package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/algorand/indexer/cmd/conduit/internal/initialize"
	"github.com/algorand/indexer/cmd/conduit/internal/list"
	"github.com/algorand/indexer/conduit"
	"github.com/algorand/indexer/conduit/loggers"
	"github.com/algorand/indexer/conduit/pipeline"
	_ "github.com/algorand/indexer/conduit/plugins/exporters/all"
	_ "github.com/algorand/indexer/conduit/plugins/importers/all"
	_ "github.com/algorand/indexer/conduit/plugins/processors/all"
	"github.com/algorand/indexer/version"
)

var (
	logger     *log.Logger
	conduitCmd = makeConduitCmd()
	//go:embed banner.txt
	banner string
)

// init() function for main package
func init() {
	conduitCmd.AddCommand(initialize.InitCommand)
	conduitCmd.AddCommand(list.Command)
}

// runConduitCmdWithConfig run the main logic with a supplied conduit config
func runConduitCmdWithConfig(args *conduit.Args) error {
	defer pipeline.HandlePanic(logger)

	if args.ConduitDataDir == "" {
		args.ConduitDataDir = os.Getenv("CONDUIT_DATA_DIR")
	}

	pCfg, err := pipeline.MakePipelineConfig(args)
	if err != nil {
		return err
	}

	// Initialize logger
	level, err := log.ParseLevel(pCfg.PipelineLogLevel)
	if err != nil {
		return fmt.Errorf("runConduitCmdWithConfig(): invalid log level: %s", err)
	}

	logger, err = loggers.MakeThreadSafeLogger(level, pCfg.LogFile)
	if err != nil {
		return fmt.Errorf("runConduitCmdWithConfig(): failed to create logger: %w", err)
	}

	logger.Infof("Using data directory: %s", args.ConduitDataDir)
	logger.Info("Conduit configuration is valid")

	if !pCfg.HideBanner {
		fmt.Printf(banner)
	}

	if pCfg.LogFile != "" {
		fmt.Printf("Writing logs to file: %s\n", pCfg.LogFile)
	} else {
		fmt.Println("Writing logs to console.")
	}

	ctx := context.Background()
	pipeline, err := pipeline.MakePipeline(ctx, pCfg, logger)
	if err != nil {
		err = fmt.Errorf("pipeline creation error: %w", err)

		// Make sure the error is written to stdout once.
		fmt.Println(err)
		if pCfg.LogFile != "" {
			logger.Error(err)
		}
		return err
	}

	err = pipeline.Init()
	if err != nil {
		return fmt.Errorf("pipeline init error: %w", err)
	}
	pipeline.Start()
	defer pipeline.Stop()
	pipeline.Wait()
	return pipeline.Error()
}

// makeConduitCmd creates the main cobra command, initializes flags
func makeConduitCmd() *cobra.Command {
	cfg := &conduit.Args{}
	var vFlag bool
	cmd := &cobra.Command{
		Use:   "conduit",
		Short: "run the conduit framework",
		Long:  "run the conduit framework",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConduitCmdWithConfig(cfg)
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if vFlag {
				fmt.Println("Conduit Pre-Release")
				fmt.Printf("%s\n", version.LongVersion())
				os.Exit(0)
			}
		},
		SilenceUsage: true,
		// Silence errors because our logger will catch and print any errors
		SilenceErrors: true,
	}
	cmd.Flags().StringVarP(&cfg.ConduitDataDir, "data-dir", "d", "", "set the data directory for the conduit binary")
	cmd.Flags().Uint64VarP(&cfg.NextRoundOverride, "next-round-override", "r", 0, "set the starting round. Overrides next-round in metadata.json")
	cmd.Flags().BoolVarP(&vFlag, "version", "v", false, "print the conduit version")

	return cmd
}

func main() {
	// Hidden command to generate docs in a given directory
	// conduit generate-docs [path]
	if len(os.Args) == 3 && os.Args[1] == "generate-docs" {
		err := doc.GenMarkdownTree(conduitCmd, os.Args[2])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := conduitCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

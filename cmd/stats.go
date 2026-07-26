package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vieolo/contour/internal/config"
	"github.com/vieolo/contour/internal/render"
	"github.com/vieolo/contour/internal/store"
	"github.com/vieolo/contour/internal/usage"
	"github.com/vieolo/termange"
)

var (
	statsProject string
	statsDays    int
	statsClear   bool
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show how agents have used the store",
	Long: "Summarise the usage logs: what agents searched for and could not " +
		"find (gaps), which items are never fetched (prune candidates), and " +
		"which are fetched most.\n\n" +
		"Usage is recorded only when an agent drives contour over MCP, and only " +
		"while usage_logging is on. Nothing ever leaves this machine.\n\n" +
		"Use --project to scope to one project, --days to a recent window, or " +
		"--clear to delete all recorded usage.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if statsClear {
			return clearUsage()
		}

		opts := usage.Options{ProjectSubstr: statsProject}
		if statsDays > 0 {
			opts.Since = time.Now().Add(-time.Duration(statsDays) * 24 * time.Hour)
		}

		report, err := usage.Aggregate(opts)
		if err != nil {
			return err
		}
		neverFetched, err := neverFetchedItems(report)
		if err != nil {
			return err
		}

		render.UsageReport(describeScope(statsProject, statsDays), report, neverFetched)
		if report.Sessions == 0 {
			hintWhyEmpty()
		}
		return nil
	},
}

func init() {
	statsCmd.Flags().StringVar(&statsProject, "project", "", "limit to projects whose path contains this string")
	statsCmd.Flags().IntVar(&statsDays, "days", 0, "limit to the last N days (0 = all time)")
	statsCmd.Flags().BoolVar(&statsClear, "clear", false, "delete all recorded usage and exit")
	rootCmd.AddCommand(statsCmd)
}

// neverFetchedItems returns the store items the report shows no fetch for. With
// no sessions there is nothing to compare against, so it returns nothing rather
// than list the whole store as "unused".
func neverFetchedItems(report *usage.Report) ([]store.Item, error) {
	if report.Sessions == 0 {
		return nil, nil
	}
	home, err := resolveStore()
	if err != nil {
		return nil, err
	}
	// Central store only, deliberately not layered: "never fetched" is about
	// curating the store you maintain, and stats aggregates across every
	// project, so no single project's overlay belongs here.
	st, err := store.Load(home.Path)
	if err != nil {
		return nil, err
	}

	fetched := report.FetchedKeys()
	var never []store.Item
	for _, it := range st.All() {
		if !fetched[it.ID] {
			never = append(never, it)
		}
	}
	return never, nil
}

func describeScope(project string, days int) string {
	scope := "all projects"
	if strings.TrimSpace(project) != "" {
		scope = "projects matching " + project
	}
	if days > 0 {
		return fmt.Sprintf("%s, last %d days", scope, days)
	}
	return scope + ", all time"
}

func hintWhyEmpty() {
	if enabled, err := config.UsageLoggingEnabled(); err == nil && !enabled {
		termange.PrintWarningln("\nusage_logging is off in your config — set it to true to record usage.")
		return
	}
	termange.PrintInfoln("\nRun some agent sessions over MCP, then check back.")
}

func clearUsage() error {
	dir, err := config.UsageDir()
	if err != nil {
		return err
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.jsonl"))
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}
	termange.PrintSuccessf("Cleared %d usage session %s from %s\n",
		len(files), pluralFiles(len(files)), dir)
	return nil
}

func pluralFiles(n int) string {
	if n == 1 {
		return "file"
	}
	return "files"
}

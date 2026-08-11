package main

import (
	"os"

	"github.com/spf13/cobra"
)

// Global flags shared by all subcommands. Populated from env defaults on the
// root's persistent flags; every subcommand reads them through the package-level
// vars rather than threading a config struct.
var (
	flagURL    string
	flagKey    string
	flagToken  string
	flagFields string
	flagPretty bool
	flagQuiet  bool
)

const (
	defaultKey = "gexfakercli" // any non-empty token authenticates; seeds a per-key cursor
)

// defaultURL resolves the API base from env, falling back to the compose
// HOST_BIND/HOST_PORT convention, then loopback:8080.
func defaultURL() string {
	if v := os.Getenv("GEXFAKER_URL"); v != "" {
		return v
	}
	host := os.Getenv("HOST_BIND")
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	port := os.Getenv("HOST_PORT")
	if port == "" {
		port = "8080"
	}
	return "http://" + host + ":" + port
}

func defaultKeyEnv() string {
	if v := os.Getenv("GEXFAKER_KEY"); v != "" {
		return v
	}
	return defaultKey
}

// defaultTokenEnv resolves the Studio/control auth token from env. Empty by default
// (open, local dev); required only when the faker sets STUDIO_AUTH_TOKEN and thus
// gates the mutating control routes (load-range, load, reset).
func defaultTokenEnv() string {
	return os.Getenv("GEXFAKER_TOKEN")
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "gexfakercli",
		Short: "LLM-first CLI for the GEX Faker API (client · skill · setup)",
		Long: "gexfakercli is a JSON-first client over the GEX Faker REST API, made for LLM agents.\n" +
			"Run `gexfakercli setup` to bring a faker to a ready state, then `gexfakercli describe`\n" +
			"to learn the full command surface. Data pulls advance a per-key playback cursor;\n" +
			"use `reset` to replay from the start.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.PersistentFlags().StringVar(&flagURL, "url", defaultURL(),
		"faker base URL (env GEXFAKER_URL, else HOST_BIND/HOST_PORT)")
	root.PersistentFlags().StringVar(&flagKey, "key", defaultKeyEnv(),
		"playback key sent on data routes (env GEXFAKER_KEY; any non-empty token works)")
	root.PersistentFlags().StringVar(&flagToken, "token", defaultTokenEnv(),
		"Studio/control auth token for mutating control routes (env GEXFAKER_TOKEN; needed only when the faker sets STUDIO_AUTH_TOKEN)")
	root.PersistentFlags().StringVar(&flagFields, "fields", "",
		"comma-separated top-level keys to keep in the output (token thrift)")
	root.PersistentFlags().BoolVar(&flagPretty, "pretty", false, "pretty-print JSON output")
	root.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "suppress progress lines on stderr")

	root.AddCommand(
		discoverCmds()...,
	)
	root.AddCommand(dataCmds()...)
	root.AddCommand(controlCmds()...)
	root.AddCommand(describeCmd())
	root.AddCommand(setupCmd())
	root.AddCommand(skillCmd())

	return root
}

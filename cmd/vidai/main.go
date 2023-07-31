package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"time"

	"github.com/igolaizola/vidai"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

// Build flags
var version = ""
var commit = ""
var date = ""

func main() {
	// Create signal based context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Launch command
	cmd := newCommand()
	if err := cmd.ParseAndRun(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func newCommand() *ffcli.Command {
	fs := flag.NewFlagSet("vidai", flag.ExitOnError)

	return &ffcli.Command{
		ShortUsage: "vidai [flags] <subcommand>",
		FlagSet:    fs,
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
		Subcommands: []*ffcli.Command{
			newVersionCommand(),
			newGenerateCommand(),
			newExtendCommand(),
			newLoopCommand(),
		},
	}
}

func newVersionCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "vidai version",
		ShortHelp:  "print version",
		Exec: func(ctx context.Context, args []string) error {
			v := version
			if v == "" {
				if buildInfo, ok := debug.ReadBuildInfo(); ok {
					v = buildInfo.Main.Version
				}
			}
			if v == "" {
				v = "dev"
			}
			versionFields := []string{v}
			if commit != "" {
				versionFields = append(versionFields, commit)
			}
			if date != "" {
				versionFields = append(versionFields, date)
			}
			fmt.Println(strings.Join(versionFields, " "))
			return nil
		},
	}
}

func newGenerateCommand() *ffcli.Command {
	cmd := "generate"
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	_ = fs.String("config", "", "config file (optional)")

	var cfg vidai.Config
	fs.BoolVar(&cfg.Debug, "debug", false, "debug mode")
	fs.DurationVar(&cfg.Wait, "wait", 2*time.Second, "wait time between requests")
	fs.StringVar(&cfg.Token, "token", "", "runway token")
	image := fs.String("image", "", "source image")
	text := fs.String("text", "", "source text")
	output := fs.String("output", "", "output file (optional, if omitted it won't be saved)")
	extend := fs.Int("extend", 0, "extend the video by this many times (optional)")
	interpolate := fs.Bool("interpolate", true, "interpolate frames (optional)")
	upscale := fs.Bool("upscale", false, "upscale frames (optional)")
	watermark := fs.Bool("watermark", false, "add watermark (optional)")

	return &ffcli.Command{
		Name:       cmd,
		ShortUsage: fmt.Sprintf("vidai %s [flags] <key> <value data...>", cmd),
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ff.PlainParser),
			ff.WithEnvVarPrefix("VIDAI"),
		},
		ShortHelp: fmt.Sprintf("vidai %s command", cmd),
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			if cfg.Token == "" {
				return fmt.Errorf("token is required")
			}
			if *image == "" && *text == "" {
				return fmt.Errorf("image or text is required")
			}
			c := vidai.New(&cfg)
			urls, err := c.Generate(ctx, *image, *text, *output, *extend,
				*interpolate, *upscale, *watermark)
			if err != nil {
				return err
			}
			if len(urls) == 1 {
				fmt.Printf("Video URL: %s\n", urls[0])
			} else {
				for i, u := range urls {
					fmt.Printf("Video URL %d: %s\n", i+1, u)
				}
			}
			return nil
		},
	}
}

func newExtendCommand() *ffcli.Command {
	cmd := "extend"
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	_ = fs.String("config", "", "config file (optional)")

	var cfg vidai.Config
	fs.BoolVar(&cfg.Debug, "debug", false, "debug mode")
	fs.DurationVar(&cfg.Wait, "wait", 2*time.Second, "wait time between requests")
	fs.StringVar(&cfg.Token, "token", "", "runway token")
	input := fs.String("input", "", "input video")
	output := fs.String("output", "", "output file (optional, if omitted it won't be saved)")
	n := fs.Int("n", 1, "extend the video by this many times")
	interpolate := fs.Bool("interpolate", true, "interpolate frames (optional)")
	upscale := fs.Bool("upscale", false, "upscale frames (optional)")
	watermark := fs.Bool("watermark", false, "add watermark (optional)")

	return &ffcli.Command{
		Name:       cmd,
		ShortUsage: fmt.Sprintf("vidai %s [flags] <key> <value data...>", cmd),
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ff.PlainParser),
			ff.WithEnvVarPrefix("VIDAI"),
		},
		ShortHelp: fmt.Sprintf("vidai %s command", cmd),
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			if cfg.Token == "" {
				return fmt.Errorf("token is required")
			}
			if *input == "" {
				return fmt.Errorf("input is required")
			}
			if *n < 1 {
				return fmt.Errorf("n must be greater than 0")
			}

			c := vidai.New(&cfg)
			urls, err := c.Extend(ctx, *input, *output, *n, *interpolate, *upscale, *watermark)
			if err != nil {
				return err
			}
			for i, u := range urls {
				fmt.Printf("Video URL %d: %s\n", i+1, u)
			}
			return nil
		},
	}
}

func newLoopCommand() *ffcli.Command {
	cmd := "loop"
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	_ = fs.String("config", "", "config file (optional)")

	input := fs.String("input", "", "input video")
	output := fs.String("output", "", "output file")

	return &ffcli.Command{
		Name:       cmd,
		ShortUsage: fmt.Sprintf("vidai %s [flags] <key> <value data...>", cmd),
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ff.PlainParser),
			ff.WithEnvVarPrefix("VIDAI"),
		},
		ShortHelp: fmt.Sprintf("vidai %s command", cmd),
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			if *input == "" {
				return fmt.Errorf("input is required")
			}
			if *output == "" {
				return fmt.Errorf("output is required")
			}
			if err := vidai.Loop(ctx, *input, *output); err != nil {
				return err
			}
			return nil
		},
	}
}

package cli

import (
	"context"
	"flag"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/igopr/vidai/pkg/cmd/extend"
	"github.com/igopr/vidai/pkg/cmd/generate"
	"github.com/igopr/vidai/pkg/cmd/loop"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/peterbourgon/ff/v3/ffyaml"
)

func NewCommand(version, commit, date string) *ffcli.Command {
	fs := flag.NewFlagSet("vidai", flag.ExitOnError)

	return &ffcli.Command{
		ShortUsage: "vidai [flags] <subcommand>",
		FlagSet:    fs,
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
		Subcommands: []*ffcli.Command{
			newVersionCommand(version, commit, date),
			newGenerateCommand(),
			newExtendCommand(),
			newLoopCommand(),
		},
	}
}

func newVersionCommand(version, commit, date string) *ffcli.Command {
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

	var cfg generate.Config
	fs.BoolVar(&cfg.Debug, "debug", false, "debug mode")
	fs.DurationVar(&cfg.Wait, "wait", 2*time.Second, "wait time between requests")
	fs.StringVar(&cfg.Token, "token", "", "runway token")

	fs.StringVar(&cfg.Model, "model", "gen2", "model to use (gen2 or gen3)")
	fs.StringVar(&cfg.Folder, "folder", "", "runway folder to store assets (optional)")
	fs.StringVar(&cfg.Image, "image", "", "source image")
	fs.StringVar(&cfg.Text, "text", "", "source text")
	fs.StringVar(&cfg.Output, "output", "", "output file (optional, if omitted it won't be saved)")
	fs.IntVar(&cfg.Extend, "extend", 0, "extend the video by this many times (optional)")
	fs.BoolVar(&cfg.Interpolate, "interpolate", true, "interpolate frames (optional)")
	fs.BoolVar(&cfg.Upscale, "upscale", false, "upscale frames (optional)")
	fs.BoolVar(&cfg.Watermark, "watermark", false, "add watermark (optional)")
	fs.IntVar(&cfg.Width, "width", 0, "output video width (optional)")
	fs.IntVar(&cfg.Height, "height", 0, "output video height (optional)")
	fs.BoolVar(&cfg.Explore, "explore", false, "explore mode (optional)")

	return &ffcli.Command{
		Name:       cmd,
		ShortUsage: fmt.Sprintf("vidai %s [flags] <key> <value data...>", cmd),
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ffyaml.Parser),
			ff.WithEnvVarPrefix("VIDAI"),
		},
		ShortHelp: fmt.Sprintf("vidai %s command", cmd),
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			return generate.Run(ctx, &cfg)
		},
	}
}

func newExtendCommand() *ffcli.Command {
	cmd := "extend"
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	_ = fs.String("config", "", "config file (optional)")

	var cfg extend.Config
	fs.BoolVar(&cfg.Debug, "debug", false, "debug mode")
	fs.DurationVar(&cfg.Wait, "wait", 2*time.Second, "wait time between requests")
	fs.StringVar(&cfg.Token, "token", "", "runway token")
	fs.StringVar(&cfg.Input, "input", "", "input video")
	fs.StringVar(&cfg.Output, "output", "", "output file (optional, if omitted it won't be saved)")
	fs.IntVar(&cfg.N, "n", 1, "extend the video by this many times")
	fs.StringVar(&cfg.Model, "model", "gen2", "model to use (gen2 or gen3)")
	fs.StringVar(&cfg.Folder, "folder", "", "runway folder to store assets (optional)")
	fs.BoolVar(&cfg.Interpolate, "interpolate", true, "interpolate frames (optional)")
	fs.BoolVar(&cfg.Upscale, "upscale", false, "upscale frames (optional)")
	fs.BoolVar(&cfg.Watermark, "watermark", false, "add watermark (optional)")
	fs.BoolVar(&cfg.Explore, "explore", false, "explore mode (optional)")

	return &ffcli.Command{
		Name:       cmd,
		ShortUsage: fmt.Sprintf("vidai %s [flags] <key> <value data...>", cmd),
		Options: []ff.Option{
			ff.WithConfigFileFlag("config"),
			ff.WithConfigFileParser(ffyaml.Parser),
			ff.WithEnvVarPrefix("VIDAI"),
		},
		ShortHelp: fmt.Sprintf("vidai %s command", cmd),
		FlagSet:   fs,
		Exec: func(ctx context.Context, args []string) error {
			return extend.Run(ctx, &cfg)
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
			ff.WithConfigFileParser(ffyaml.Parser),
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
			if err := loop.Run(ctx, *input, *output); err != nil {
				return err
			}
			return nil
		},
	}
}

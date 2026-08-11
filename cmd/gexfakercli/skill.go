package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dgnsrekt/gexbot-downloader/cmd/gexfakercli/skill"
)

const skillName = "gexfakercli"

// skillTarget names an agent's skills directory and how to locate its root.
type skillTarget struct {
	name string // "claude" | "codex"
	sub  string // e.g. ".claude/skills"
}

var skillTargets = []skillTarget{
	{name: "claude", sub: filepath.Join(".claude", "skills")},
	{name: "codex", sub: filepath.Join(".codex", "skills")},
}

func skillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install or remove the gexfakercli agent skill",
	}
	cmd.AddCommand(skillInstallCmd(), skillUninstallCmd())
	return cmd
}

// resolveDests returns the skills directories to act on given the flags. With no
// --claude/--codex/--dir, it targets every agent whose parent dir already exists
// (so we don't create a .codex tree for a user who only runs Claude).
func resolveDests(claudeOnly, codexOnly bool, dir string) ([]string, error) {
	if dir != "" {
		return []string{filepath.Join(dir, skillName)}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, &apiError{Msg: "cannot resolve home dir: " + err.Error()}
	}
	pick := func(t skillTarget) bool {
		switch {
		case claudeOnly && !codexOnly:
			return t.name == "claude"
		case codexOnly && !claudeOnly:
			return t.name == "codex"
		case claudeOnly && codexOnly:
			return true
		default:
			// auto: only where the agent's config root (e.g. ~/.claude) exists
			parent := filepath.Join(home, filepath.Dir(t.sub))
			_, statErr := os.Stat(parent)
			return statErr == nil
		}
	}
	var dests []string
	for _, t := range skillTargets {
		if pick(t) {
			dests = append(dests, filepath.Join(home, t.sub, skillName))
		}
	}
	if len(dests) == 0 {
		return nil, &apiError{Msg: "no target skills dir found; pass --claude, --codex, or --dir"}
	}
	return dests, nil
}

func skillInstallCmd() *cobra.Command {
	var claude, codex bool
	var dir string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Write the embedded SKILL.md into the Claude/Codex skills directories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dests, err := resolveDests(claude, codex, dir)
			if err != nil {
				return fail(err)
			}
			installed := make([]string, 0, len(dests))
			for _, d := range dests {
				if err := writeSkill(d); err != nil {
					return fail(&apiError{Msg: fmt.Sprintf("installing to %s: %v", d, err)})
				}
				installed = append(installed, d)
			}
			out, _ := json.Marshal(map[string]any{"installed": installed, "skill": skillName})
			return emit(out)
		},
	}
	cmd.Flags().BoolVar(&claude, "claude", false, "install into ~/.claude/skills")
	cmd.Flags().BoolVar(&codex, "codex", false, "install into ~/.codex/skills")
	cmd.Flags().StringVar(&dir, "dir", "", "install into a custom skills dir (writes <dir>/gexfakercli)")
	return cmd
}

func skillUninstallCmd() *cobra.Command {
	var claude, codex bool
	var dir string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the installed gexfakercli skill directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dests, err := resolveDests(claude, codex, dir)
			if err != nil {
				return fail(err)
			}
			removed := make([]string, 0, len(dests))
			for _, d := range dests {
				if _, statErr := os.Stat(d); statErr != nil {
					continue // nothing installed there
				}
				if err := os.RemoveAll(d); err != nil {
					return fail(&apiError{Msg: fmt.Sprintf("removing %s: %v", d, err)})
				}
				removed = append(removed, d)
			}
			out, _ := json.Marshal(map[string]any{"removed": removed, "skill": skillName})
			return emit(out)
		},
	}
	cmd.Flags().BoolVar(&claude, "claude", false, "remove from ~/.claude/skills")
	cmd.Flags().BoolVar(&codex, "codex", false, "remove from ~/.codex/skills")
	cmd.Flags().StringVar(&dir, "dir", "", "remove from a custom skills dir")
	return cmd
}

// writeSkill copies the embedded skill bundle into dest, creating parent dirs.
func writeSkill(dest string) error {
	return fs.WalkDir(skill.Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := skill.Files.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

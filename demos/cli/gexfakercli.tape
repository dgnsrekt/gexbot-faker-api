# gexfakercli — the agent golden flow, rendered by charmbracelet VHS.
#
# Regenerate:  just demos-render   (needs a faker on :8080; see demos/README.md)
# Output is committed at website/public/demos/gexfakercli.gif and embedded in the
# guides landing + README.

Output website/public/demos/gexfakercli.gif

# On-brand terminal: the GEX Faker Studio near-black palette + green accent,
# JetBrains Mono (matches the guides).
Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 17
Set Width 1180
Set Height 720
Set Padding 26
Set TypingSpeed 55ms
Set Theme { "name": "GEX Faker", "background": "#0d0e10", "foreground": "#e7e8ea", "cursor": "#6fcf97", "selection": "#1f242a", "black": "#0d0e10", "red": "#e06c75", "green": "#6fcf97", "yellow": "#e0b164", "blue": "#61afef", "magenta": "#c678dd", "cyan": "#56b6c2", "white": "#d6d8db", "brightBlack": "#5c616a", "brightRed": "#e06c75", "brightGreen": "#7fd1a8", "brightYellow": "#e0b164", "brightBlue": "#61afef", "brightMagenta": "#c678dd", "brightCyan": "#56b6c2", "brightWhite": "#f4f5f6" }

# Clean green prompt (matches the CLI's ❯ brand glyph) + a known loaded day, set
# off-camera so the tape is self-contained (deterministic regardless of render
# order or the faker's current state).
Hide
Type "export PS1='\[\033[38;2;111;207;151m\]❯\[\033[0m\] '"
Enter
Type "gexfakercli load 2026-08-10 >/dev/null"
Enter
Sleep 2s
Type "clear"
Enter
Show

Sleep 800ms

# Zero → ready: bring a faker to a verified, ready state for an agent.
Type "gexfakercli setup --quiet --pretty"
Enter
Sleep 3.5s

# Is it up, and which day is it replaying?
Type "gexfakercli status"
Enter
Sleep 2.5s

# Pull a snapshot. Each call advances this key's playback cursor…
Type "gexfakercli classic QQQ gex_zero --fields timestamp,spot,zero_gamma"
Enter
Sleep 2.5s

# …so the next pull returns the next snapshot (note the timestamp).
Type "gexfakercli classic QQQ gex_zero --fields timestamp,spot"
Enter
Sleep 2.5s

# Rewind to replay the day from the start.
Type "gexfakercli reset"
Enter
Sleep 3s

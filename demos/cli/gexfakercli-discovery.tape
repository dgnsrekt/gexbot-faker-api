# gexfakercli — discovery & switching days, rendered by charmbracelet VHS.
#
# Regenerate:  just demos-render   (needs a faker on :8080; see demos/README.md)
# Output is committed at website/public/demos/gexfakercli-discovery.gif.

Output website/public/demos/gexfakercli-discovery.gif

# Same on-brand terminal as gexfakercli.tape (near-black + green, JetBrains Mono).
Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 17
Set Width 1180
Set Height 720
Set Padding 26
Set TypingSpeed 55ms
Set Theme { "name": "GEX Faker", "background": "#0d0e10", "foreground": "#e7e8ea", "cursor": "#6fcf97", "selection": "#1f242a", "black": "#0d0e10", "red": "#e06c75", "green": "#6fcf97", "yellow": "#e0b164", "blue": "#61afef", "magenta": "#c678dd", "cyan": "#56b6c2", "white": "#d6d8db", "brightBlack": "#5c616a", "brightRed": "#e06c75", "brightGreen": "#7fd1a8", "brightYellow": "#e0b164", "brightBlue": "#61afef", "brightMagenta": "#c678dd", "brightCyan": "#56b6c2", "brightWhite": "#f4f5f6" }

# Green prompt + a known starting day, off-camera (self-contained + deterministic).
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

# Which market days can I replay?
Type "gexfakercli dates"
Enter
Sleep 2.5s

# Hot-swap to a different day — no restart (reloading a full day takes a moment).
Type "gexfakercli load 2026-07-17"
Enter
Sleep 6s

# What's on that day?
Type "gexfakercli available 2026-07-17 --fields summary"
Enter
Sleep 2.5s

# Pull a greek profile and orderflow for a ticker on that day.
Type "gexfakercli state SPX gamma_zero --fields timestamp,ticker,spot"
Enter
Sleep 2.5s
Type "gexfakercli orderflow SPX --fields timestamp,ticker,net_dex"
Enter
Sleep 3s

# Restore the default day, off-camera.
Hide
Type "gexfakercli load 2026-08-10 >/dev/null"
Enter
Show

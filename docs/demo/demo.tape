# Regenerate from the repo root: vhs docs/demo/demo.tape
# Runs against a throwaway cache seeded from seed.sql; the invalid token keeps it off any real account.

Output docs/demo.gif

Require go
Require sqlite3

Set Shell zsh
Set FontSize 15
Set Width 1400
Set Height 460
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type "export DEMO=$(mktemp -d) XDG_DATA_HOME=$(mktemp -d) TODOIST_TOKEN=invalid PS1='$ ' RPROMPT='' && go build -o $DEMO/tdc ./cmd/todoist-cli && export PATH=$DEMO:$PATH && tdc projects >/dev/null && sqlite3 $XDG_DATA_HOME/todoist-cli/todoist-cli.db < docs/demo/seed.sql && clear"
Enter
Sleep 4s
Show

Type "tdc ls"
Enter
Sleep 2.5s

Type "tdc cd 'Website Relaunch'"
Enter
Sleep 1s

Type "tdc ls -b"
Enter
Sleep 3.5s

Hide
Type "clear"
Enter
Show

Type "tdc ls -l urgent -g"
Enter
Sleep 2.5s

Type "tdc stats"
Enter
Sleep 3s

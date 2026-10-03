package mcpsetup

import "strings"

func uvCommandOperand(args []string, needsRun bool) string {
	valueFlags := map[string]bool{
		"--directory": true, "--project": true, "--config-file": true,
		"--cache-dir": true, "--color": true, "--allow-insecure-host": true,
		"-p": true, "--python": true, "--from": true,
		"-w": true, "--with": true, "--with-requirements": true,
	}
	options := true
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if options && arg == "--" {
			options = false
			continue
		}
		if options && strings.HasPrefix(arg, "-") {
			if valueFlags[arg] {
				i++
			}
			continue
		}
		if arg == "" {
			continue
		}
		if needsRun {
			if arg != "run" {
				return ""
			}
			needsRun = false
			continue
		}
		return arg
	}
	return ""
}

package mcpsetup

import "strings"

func dockerRunOperand(args []string) string {
	valueFlags := map[string]bool{
		"-e": true, "--env": true, "--env-file": true,
		"-v": true, "--volume": true, "--mount": true,
		"-u": true, "--user": true, "-w": true, "--workdir": true,
		"--name": true, "--network": true, "--platform": true,
		"--entrypoint": true, "--pull": true,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
		key, _, inline := strings.Cut(arg, "=")
		switch key {
		case "--rm", "--interactive", "--tty", "--detach", "--quiet",
			"--init", "--read-only", "--sig-proxy", "--no-healthcheck":
			continue
		}
		if !strings.HasPrefix(arg, "--") && strings.TrimLeft(arg[1:], "itdqP") == "" {
			continue
		}
		if valueFlags[key] {
			if !inline {
				i++
			}
			continue
		}
		if !strings.HasPrefix(arg, "--") && len(arg) > 2 && valueFlags[arg[:2]] {
			continue
		}
		return ""
	}
	return ""
}

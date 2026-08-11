package provider

import "strings"

// extractEnvValue finds a key in an array of "KEY=VALUE" strings and returns its value.
func extractEnvValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				return parts[1], true
			}
		}
	}
	return "", false
}

// extractFlagValue finds a flag in a command array and returns the value immediately following it.
func extractFlagValue(cmd []string, flag string) (string, bool) {
	for i, arg := range cmd {
		if arg == flag && i+1 < len(cmd) {
			return cmd[i+1], true
		}
	}
	return "", false
}

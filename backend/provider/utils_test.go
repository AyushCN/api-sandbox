package provider

import "testing"

func TestExtractEnvValue(t *testing.T) {
	tests := []struct {
		name     string
		env      []string
		key      string
		expected string
		found    bool
	}{
		{"Found", []string{"FOO=bar", "POSTGRES_PASSWORD=secret", "BAZ=qux"}, "POSTGRES_PASSWORD", "secret", true},
		{"Not Found", []string{"FOO=bar", "BAZ=qux"}, "POSTGRES_PASSWORD", "", false},
		{"Empty Env", []string{}, "POSTGRES_PASSWORD", "", false},
		{"Malformed Entry", []string{"POSTGRES_PASSWORD"}, "POSTGRES_PASSWORD", "", false},
		{"Contains Equals", []string{"POSTGRES_PASSWORD=my=secret=password"}, "POSTGRES_PASSWORD", "my=secret=password", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, found := extractEnvValue(tt.env, tt.key)
			if val != tt.expected || found != tt.found {
				t.Errorf("extractEnvValue() = (%v, %v), want (%v, %v)", val, found, tt.expected, tt.found)
			}
		})
	}
}

func TestExtractFlagValue(t *testing.T) {
	tests := []struct {
		name     string
		cmd      []string
		flag     string
		expected string
		found    bool
	}{
		{"Found", []string{"redis-server", "--requirepass", "secret"}, "--requirepass", "secret", true},
		{"Not Found", []string{"redis-server", "--port", "6379"}, "--requirepass", "", false},
		{"Empty Cmd", []string{}, "--requirepass", "", false},
		{"Flag At End", []string{"redis-server", "--requirepass"}, "--requirepass", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, found := extractFlagValue(tt.cmd, tt.flag)
			if val != tt.expected || found != tt.found {
				t.Errorf("extractFlagValue() = (%v, %v), want (%v, %v)", val, found, tt.expected, tt.found)
			}
		})
	}
}

package provider

import "testing"

func TestTraefikRuntimeConfigUsesKVPaths(t *testing.T) {
	for _, tt := range []struct {
		name       string
		domain     string
		entrypoint string
		wantTLS    bool
	}{
		{name: "localhost preview", domain: "localhost", entrypoint: "web"},
		{name: "hosted preview", domain: "sandbox.example.test", entrypoint: "websecure", wantTLS: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := traefikRuntimeConfig("env-1", tt.domain, "172.20.0.2", "3000")
			want := map[string]string{
				"traefik/http/routers/env-env-1/rule":                        "Host(`env-1." + tt.domain + "`)",
				"traefik/http/routers/env-env-1/service":                     "env-env-1",
				"traefik/http/routers/env-env-1/entrypoints/0":               tt.entrypoint,
				"traefik/http/services/env-env-1/loadbalancer/servers/0/url": "http://172.20.0.2:3000",
			}
			if tt.wantTLS {
				want["traefik/http/routers/env-env-1/tls/certresolver"] = "myresolver"
			}
			for key, value := range want {
				if config[key] != value {
					t.Errorf("config[%q] = %q, want %q", key, config[key], value)
				}
			}
			if _, legacy := config["traefik/http/routers/env-env-1"]; legacy {
				t.Fatal("runtime route must be written as individual KV paths, not a Redis hash")
			}
		})
	}
}

func TestTraefikRuntimeConfigKeysIncludeAllFields(t *testing.T) {
	want := map[string]bool{
		"traefik/http/routers/env-env-1":                             false,
		"traefik/http/routers/env-env-1/rule":                        false,
		"traefik/http/routers/env-env-1/service":                     false,
		"traefik/http/routers/env-env-1/entrypoints/0":               false,
		"traefik/http/routers/env-env-1/tls/certresolver":            false,
		"traefik/http/services/env-env-1/loadbalancer/servers/0":     false,
		"traefik/http/services/env-env-1/loadbalancer/servers/0/url": false,
	}
	for _, key := range traefikRuntimeConfigKeys("env-1") {
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected cleanup key %q", key)
			continue
		}
		want[key] = true
	}
	for key, found := range want {
		if !found {
			t.Errorf("cleanup keys do not include %q", key)
		}
	}
}

package config

import "testing"

func TestNormalizeServicesDefaultsAudienceToKey(t *testing.T) {
	cfg := &Config{
		Environment: "development",
		Services: []ServiceEntry{
			{Key: "squirrel", UIExchangeURL: "http://localhost:5174/impersonate/exchange", UIOrigin: "http://localhost:5174"},
		},
	}
	if err := normalizeServices(cfg); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if cfg.Services[0].Audience != "squirrel" {
		t.Errorf("expected audience defaulted to key, got %q", cfg.Services[0].Audience)
	}
}

func TestNormalizeServicesRejectsDuplicates(t *testing.T) {
	cfg := &Config{
		Environment: "development",
		Services: []ServiceEntry{
			{Key: "a", UIExchangeURL: "http://localhost:1/x", UIOrigin: "http://localhost:1"},
			{Key: "a", UIExchangeURL: "http://localhost:2/x", UIOrigin: "http://localhost:2"},
		},
	}
	if err := normalizeServices(cfg); err == nil {
		t.Error("expected duplicate KEY to be rejected")
	}

	cfg2 := &Config{
		Environment: "development",
		Services: []ServiceEntry{
			{Key: "a", UIExchangeURL: "http://localhost:1/x", UIOrigin: "http://localhost:1"},
			{Key: "b", UIExchangeURL: "http://localhost:2/x", UIOrigin: "http://localhost:1"},
		},
	}
	if err := normalizeServices(cfg2); err == nil {
		t.Error("expected duplicate UI_ORIGIN to be rejected")
	}
}

func TestNormalizeServicesRequiresFields(t *testing.T) {
	cases := []ServiceEntry{
		{Key: "", UIExchangeURL: "http://localhost/x", UIOrigin: "http://localhost"},
		{Key: "a", UIExchangeURL: "", UIOrigin: "http://localhost"},
		{Key: "a", UIExchangeURL: "http://localhost/x", UIOrigin: ""},
		{Key: "a", UIExchangeURL: "not-a-url", UIOrigin: "http://localhost"},
		{Key: "a", UIExchangeURL: "ftp://localhost/x", UIOrigin: "http://localhost"},
	}
	for i, c := range cases {
		cfg := &Config{Environment: "development", Services: []ServiceEntry{c}}
		if err := normalizeServices(cfg); err == nil {
			t.Errorf("case %d: expected error for %+v", i, c)
		}
	}
}

func TestNormalizeServicesProductionRequiresHTTPS(t *testing.T) {
	cfg := &Config{
		Environment: "production",
		Services: []ServiceEntry{
			{Key: "a", UIExchangeURL: "http://insecure/x", UIOrigin: "http://insecure"},
		},
	}
	if err := normalizeServices(cfg); err == nil {
		t.Error("expected http URL to be rejected in production")
	}

	ok := &Config{
		Environment: "production",
		Services: []ServiceEntry{
			{Key: "a", UIExchangeURL: "https://secure.example/x", UIOrigin: "https://secure.example"},
		},
	}
	if err := normalizeServices(ok); err != nil {
		t.Errorf("expected https URLs to pass in production, got %v", err)
	}
}

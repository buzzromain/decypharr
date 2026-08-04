package config

import "testing"

// AllDebrid's ~5000-torrent cap is a real, fixed constraint (not a default),
// so `limit` must never silently do nothing when a user sets it there —
// validateDebrids rejects it outright instead.
func TestValidateDebrids_RejectsLimitForAllDebrid(t *testing.T) {
	err := validateDebrids([]Debrid{{
		APIKey:   "key",
		Provider: "alldebrid",
		Limit:    1000,
	}})
	if err == nil {
		t.Fatal("expected an error for limit set on an alldebrid provider, got nil")
	}
}

func TestValidateDebrids_AllowsLimitForOtherProviders(t *testing.T) {
	err := validateDebrids([]Debrid{{
		APIKey:   "key",
		Provider: "realdebrid",
		Limit:    1000,
	}})
	if err != nil {
		t.Errorf("limit on a non-alldebrid provider should be allowed, got: %v", err)
	}
}

func TestValidateDebrids_AllDebridWithoutLimitIsFine(t *testing.T) {
	err := validateDebrids([]Debrid{{
		APIKey:       "key",
		Provider:     "alldebrid",
		SlotStrategy: "remove_oldest",
	}})
	if err != nil {
		t.Errorf("alldebrid without limit set should be valid, got: %v", err)
	}
}

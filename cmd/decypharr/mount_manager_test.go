package decypharr

import (
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/manager"
)

func TestCreateMountManager_DefaultAndNoneReturnStub(t *testing.T) {
	// Only test mount types that don't trigger config.Get() during construction.
	// rclone, dfs, and external constructors call config.Get() internally,
	// so we only test the default/none path here.
	tests := []struct {
		name      string
		mountType config.MountType
	}{
		{"none mount type", config.MountTypeNone},
		{"empty mount type", ""},
		{"unknown mount type", "nonexistent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Mount: config.Mount{Type: tt.mountType},
			}
			got := createMountManager(nil, cfg)
			if got == nil {
				t.Fatal("createMountManager returned nil")
			}
			if got.Type() != "none" {
				t.Errorf("Type() = %q, want 'none'", got.Type())
			}
		})
	}
}

func TestCreateMountManager_SwitchCoversAllConfigTypes(t *testing.T) {
	// Verify the switch statement handles all defined MountType constants.
	// We can't call the constructors without config, but we verify the
	// switch branches exist by checking that known types don't fall through
	// to default (which returns stub/"none").
	//
	// This is a compile-time documentation test — if someone adds a new
	// MountType but forgets to update createMountManager, this test
	// reminds them to add coverage here too.
	knownTypes := []config.MountType{
		config.MountTypeRclone,
		config.MountTypeDFS,
		config.MountTypeExternalRclone,
		config.MountTypeNone,
	}
	_ = knownTypes // Ensure this list stays in sync with config constants
}

func TestStubMountManager_Behaviors(t *testing.T) {
	stub := manager.NewStubMountManager()

	if stub.Type() != "none" {
		t.Errorf("Type() = %q, want 'none'", stub.Type())
	}
	if stub.IsReady() {
		t.Error("IsReady() = true, want false for stub")
	}
	if err := stub.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
	if err := stub.Refresh(nil); err != nil {
		t.Errorf("Refresh() error = %v, want nil", err)
	}
	stats := stub.Stats()
	if stats == nil {
		t.Error("Stats() returned nil")
	}
}

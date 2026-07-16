package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestBuildCommandEffectsRejectsInvalidMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
		wantPanic   string
	}{
		{
			name:        "unknown_kind",
			annotations: map[string]string{effectsAnnotation: "shell_execution"},
			wantPanic:   "unknown effect kind",
		},
		{
			name:        "empty_token",
			annotations: map[string]string{effectsAnnotation: effectKindNetworkAccess + ","},
			wantPanic:   "empty introspect/effects token",
		},
		{
			name:        "empty_flag",
			annotations: map[string]string{effectsAnnotation: effectKindLocalFilesystemWrite + "@"},
			wantPanic:   "with an empty flag",
		},
		{
			name:        "multiple_at_signs",
			annotations: map[string]string{effectsAnnotation: effectKindLocalFilesystemWrite + "@force@again"},
			wantPanic:   "invalid introspect/effects token",
		},
		{
			name:        "missing_flag",
			annotations: map[string]string{effectsAnnotation: effectKindLocalFilesystemWrite + "@missing"},
			wantPanic:   "references unavailable flag",
		},
		{
			name:        "unknown_suppression",
			annotations: map[string]string{suppressGlobalFlagEffectsAnnotation: "missing"},
			wantPanic:   "suppresses unknown global flag effect",
		},
		{
			name:        "uninherited_suppression",
			annotations: map[string]string{suppressGlobalFlagEffectsAnnotation: "output"},
			wantPanic:   "does not inherit",
		},
		{
			name:        "effectless_suppression",
			annotations: map[string]string{suppressGlobalFlagEffectsAnnotation: "format"},
			wantPanic:   "has no declared effects",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				got := recover()
				if got == nil {
					t.Errorf("buildCommandEffects() did not panic, want panic containing %q", test.wantPanic)
					return
				}
				if message := fmt.Sprint(got); !strings.Contains(message, test.wantPanic) {
					t.Errorf("buildCommandEffects() panic = %q, want substring %q", message, test.wantPanic)
				}
			}()

			cmd := &cobra.Command{Use: "test", Annotations: test.annotations}
			buildCommandEffects(cmd, nil)
		})
	}
}

func TestBuildCommandEffectsAcceptsConfigurationDependentEffect(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{
		Use: "test",
		Annotations: map[string]string{
			effectsAnnotation: effectKindProcessExecution + "@" + effectWhenConfigurationDependent,
		},
	}
	want := []EffectDoc{{Kind: effectKindProcessExecution, When: effectWhenConfigurationDependent}}
	got := buildCommandEffects(cmd, nil)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("buildCommandEffects() = %#v, want %#v", got, want)
	}
}

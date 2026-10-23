package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// TestNewReaderNamesImplicitProfileAmongMany pins the notice that makes a read
// against the config file's default profile visible when several exist. It is
// written before credentials are resolved, so it also precedes any failure.
func TestNewReaderNamesImplicitProfileAmongMany(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cfg  config.Config
		want string
	}{
		{
			name: "default_profile among two",
			cfg:  config.Config{Profile: "prod", ProfileSource: config.ProfileFromDefaultProfile, ProfileCount: 2},
			want: "zscalerctl: using profile \"prod\" (default_profile; 2 profiles configured); pass --profile to choose\n",
		},
		{
			name: "builtin default among three",
			cfg:  config.Config{Profile: "default", ProfileSource: config.ProfileFromBuiltinDefault, ProfileCount: 3},
			want: "zscalerctl: using profile \"default\" (no default_profile set; 3 profiles configured); pass --profile to choose\n",
		},
		{
			name: "flag among two",
			cfg:  config.Config{Profile: "dev", ProfileSource: config.ProfileFromFlag, ProfileCount: 2},
		},
		{
			name: "env among two",
			cfg:  config.Config{Profile: "dev", ProfileSource: config.ProfileFromEnv, ProfileCount: 2},
		},
		{
			name: "default_profile alone",
			cfg:  config.Config{Profile: "prod", ProfileSource: config.ProfileFromDefaultProfile, ProfileCount: 1},
		},
		{
			name: "env-only configuration",
			cfg:  config.Config{Profile: "default"},
		},
	} {
		var notices bytes.Buffer
		cfg := tc.cfg
		_, _ = newReaderFromConfig(context.Background(), &cfg, Options{
			Notices: &notices,
			newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) {
				return nil, nil
			},
		})
		if got := notices.String(); got != tc.want {
			t.Errorf("%s: notices = %q, want %q", tc.name, got, tc.want)
		}
	}

	// Without a Notices writer nothing is written and nothing panics.
	cfg := config.Config{Profile: "prod", ProfileSource: config.ProfileFromDefaultProfile, ProfileCount: 2}
	_, _ = newReaderFromConfig(context.Background(), &cfg, Options{
		newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) { return nil, nil },
	})
}

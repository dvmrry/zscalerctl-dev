package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/dvmrry/zscalerctl/internal/fileperm"
	"github.com/dvmrry/zscalerctl/internal/secretref"
	"gopkg.in/yaml.v3"
)

const maxConfigFileBytes = 1 << 20

type profileFile struct {
	DefaultProfile string                 `yaml:"default_profile"`
	Profiles       map[string]profileData `yaml:"profiles"`
}

type profileData struct {
	AuthMode         string               `yaml:"auth_mode"`
	VanityDomain     string               `yaml:"vanity_domain"`
	Cloud            string               `yaml:"cloud"`
	ClientID         string               `yaml:"client_id"`
	ClientSecretRef  *secretref.SecretRef `yaml:"client_secret_ref"`
	ZPACustomerID    string               `yaml:"zpa_customer_id"`
	ZPAMicrotenantID string               `yaml:"zpa_microtenant_id"`
	ZIAUsername      string               `yaml:"zia_username"`
	ZIAPasswordRef   *secretref.SecretRef `yaml:"zia_password_ref"`
	ZIAAPIKeyRef     *secretref.SecretRef `yaml:"zia_api_key_ref"`
	ZIACloud         string               `yaml:"zia_cloud"`
	Redaction        string               `yaml:"redaction"`
	NoCache          *bool                `yaml:"no_cache"`
}

type loadedProfile struct {
	name string
	data profileData
	// count is the number of profiles in the file; defaultSource says how
	// name was chosen when no profile was requested.
	count         int
	defaultSource ProfileSource
}

func loadProfileFile(path, requestedProfile string, explicit bool) (loadedProfile, bool, error) {
	file, err := fileperm.OpenOwnerOnly(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if explicit {
				return loadedProfile{}, false, fmt.Errorf("%w: config file not found", ErrInvalidConfig)
			}
			return loadedProfile{}, false, nil
		}
		if errors.Is(err, fileperm.ErrInsecurePermissions) {
			return loadedProfile{}, false, fmt.Errorf("%w: config file permissions: %w", ErrInvalidConfig, err)
		}
		return loadedProfile{}, false, fmt.Errorf("%w: read config file: %w", ErrInvalidConfig, err)
	}
	defer file.Close()

	body, err := readBoundedConfig(file)
	if err != nil {
		return loadedProfile{}, false, fmt.Errorf("%w: read config file: %w", ErrInvalidConfig, err)
	}
	var model profileFile
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&model); err != nil {
		return loadedProfile{}, false, fmt.Errorf("%w: parse config file: %w", ErrInvalidConfig, err)
	}
	if len(model.Profiles) == 0 {
		return loadedProfile{}, false, fmt.Errorf("%w: config file has no profiles", ErrInvalidConfig)
	}
	name := requestedProfile
	defaultSource := ProfileFromDefaultProfile
	if name == "" {
		name = model.DefaultProfile
	}
	if name == "" {
		name = "default"
		defaultSource = ProfileFromBuiltinDefault
	}
	data, ok := model.Profiles[name]
	if !ok {
		return loadedProfile{}, false, fmt.Errorf("%w: profile %q not found", ErrInvalidConfig, name)
	}
	return loadedProfile{name: name, data: data, count: len(model.Profiles), defaultSource: defaultSource}, true, nil
}

func readBoundedConfig(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxConfigFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxConfigFileBytes {
		return nil, fmt.Errorf("config file exceeds %d bytes", maxConfigFileBytes)
	}
	return body, nil
}

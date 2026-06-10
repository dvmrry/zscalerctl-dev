//go:build semgrepfixtures

package tests

import "github.com/dvmrry/zscalerctl/internal/secret"

type sdkConfiguration struct {
	ClientID     string
	ClientSecret string
}

type readerConfig struct {
	ClientID     secret.Secret
	ClientSecret secret.Secret
}

func newSDKConfiguration(cfg readerConfig) *sdkConfiguration {
	return &sdkConfiguration{
		ClientID:     cfg.ClientID.Reveal(),
		ClientSecret: cfg.ClientSecret.Reveal(),
	}
}

type legacyZIAConfig struct {
	Username string
	Password string
	APIKey   string
}

type legacyZIAReaderConfig struct {
	ZIALegacy struct {
		Username secret.Secret
		Password secret.Secret
		APIKey   secret.Secret
	}
}

func newLegacyZIAConfiguration(cfg legacyZIAReaderConfig) (*legacyZIAConfig, error) {
	ziaCfg := &legacyZIAConfig{
		Username: cfg.ZIALegacy.Username.Reveal(),
		Password: cfg.ZIALegacy.Password.Reveal(),
		APIKey:   cfg.ZIALegacy.APIKey.Reveal(),
	}
	return ziaCfg, nil
}

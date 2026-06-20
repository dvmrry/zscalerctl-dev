package cli

// configInitTemplate is the commented starter config written by `config init`.
// It parses as a valid config (every uncommented field is a known profileData
// field; secret refs are commented, so credentials simply stay unconfigured)
// and contains no secret values. Update profile.go's YAML tags and this
// template together.
const configInitTemplate = `# zscalerctl configuration
#
# Owner-only file created by "zscalerctl config init". Environment variables
# (ZSCALERCTL_*) always take precedence over anything set here. Secret VALUES
# never belong in this file: reference them with a secret ref scheme instead.

default_profile: prod

profiles:
  prod:
    # Non-secret tenant metadata. Replace the placeholders below.
    vanity_domain: example          # your Zscaler vanity domain
    cloud: PRODUCTION               # OneAPI cloud, e.g. PRODUCTION
    client_id: REPLACE_WITH_CLIENT_ID
    # zpa_customer_id: REPLACE_WITH_ZPA_CUSTOMER_ID   # only for ZPA resources
    # zpa_microtenant_id: REPLACE_WITH_MICROTENANT_ID

    # Client secret: pick ONE secret ref scheme and uncomment it. Do not paste
    # the secret value here.
    #
    #   env    — read from an environment variable (highest-precedence path):
    # client_secret_ref: env:ZSCALERCTL_CLIENT_SECRET
    #
    #   file   — read from an owner-only file (absolute path):
    # client_secret_ref: file:C:\Users\you\AppData\Local\zscalerctl\client-secret
    #
    #   keyring — read from the OS keyring as service/key:
    # client_secret_ref: keyring:zscalerctl/prod-client-secret
    #
    #   cmd    — run a local command (no shell; 10s default timeout):
    # client_secret_ref:
    #   cmd:
    #     argv: ["/usr/local/bin/zscaler-secret", "prod", "client-secret"]
    #     timeout: 5s

    # ZIA legacy auth (read-only ZIA when OneAPI is unavailable). Uncomment
    # auth_mode and the matching refs to use it.
    # auth_mode: zia-legacy
    # zia_username: REPLACE_WITH_ZIA_USERNAME
    # zia_password_ref: env:ZSCALERCTL_ZIA_PASSWORD
    # zia_api_key_ref: env:ZSCALERCTL_ZIA_API_KEY
    # zia_cloud: REPLACE_WITH_ZIA_CLOUD

    # Defaults.
    # redaction: standard            # standard | share | paranoid
    # no_cache: false
`

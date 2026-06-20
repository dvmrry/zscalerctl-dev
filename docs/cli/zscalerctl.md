# zscalerctl CLI Reference

This reference is generated from the live Cobra command tree.
Do not edit by hand — run `go run ./scripts/gen-cli-docs.go` to regenerate.

## Global Flags

These flags are accepted by every command:

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--color` | `string` | `auto` | color output: auto, always, never |
| `--config` | `string` | `—` | config file path |
| `--fields` | `string` | `—` | comma-separated output fields to keep (narrows the sanitized output) |
| `--filter` | `stringArray` | `—` | narrow list results: key=value (exact) or key~value (substring); repeatable, all must match |
| `--format` | `string` | `auto` | output format: auto, table, json, ndjson, pretty |
| `--log-level` | `string` | `off` | diagnostic logging to stderr: off, error, warn, info, debug |
| `--no-cache` | `bool` | `false` | bypass API cache where supported |
| `--no-color` | `bool` | `false` | disable color output |
| `--output` | `string` | `—` | output path |
| `--profile` | `string` | `—` | profile name |
| `--redaction` | `string` | `—` | redaction mode: standard, share, paranoid |
| `--search` | `string` | `—` | narrow list results to records whose rendered values contain term (case-insensitive) |
| `--timeout` | `duration` | `30s` | request timeout |

## Commands

### auth

inspect authentication configuration

```
zscalerctl auth
```

#### auth status

show authentication status for the active profile

```
zscalerctl auth status
```

### config

manage zscalerctl configuration

```
zscalerctl config
```

#### config init

write a starter config file with owner-only permissions

```
zscalerctl config init
```

**Flags:**

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--force` | `bool` | `false` | overwrite an existing config file |

#### config show

show the active configuration (redacted)

```
zscalerctl config show
```

### diff

compare two dump directories and report configuration drift

```
zscalerctl diff <old-dump-dir> <new-dump-dir>
```

**Flags:**

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--allow-partial` | `bool` | `false` | compare partial dumps instead of rejecting them |
| `--detail` | `bool` | `false` | include record-level table details |
| `--fail-on-drift` | `bool` | `false` | exit 7 when drift is detected |
| `--ignore-operational` | `bool` | `false` | ignore operational metadata on keyed and singleton resources |
| `--products` | `string` | `—` | comma-separated products: zia,zpa |
| `--resources` | `string` | `—` | comma-separated resources: locations or zia/locations |

### doctor

check configuration, credentials, and connectivity

```
zscalerctl doctor
```

### dump

write a full or filtered resource dump to a directory

```
zscalerctl dump
```

**Flags:**

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--continue-on-error` | `bool` | `false` | write a partial dump when individual resources fail |
| `--force` | `bool` | `false` | replace an existing zscalerctl dump directory |
| `--out` | `string` | `—` | dump output directory |
| `--products` | `string` | `—` | comma-separated products: zia,zpa |
| `--resources` | `string` | `—` | comma-separated resources: locations or zia/locations |

### schema

inspect the resource catalog schema

```
zscalerctl schema
```

#### schema list

list all catalog resources and their supported operations

```
zscalerctl schema list
```

### version

print version, commit, build date, and runtime info

```
zscalerctl version
```

### zcc

read zcc resources

```
zscalerctl zcc
```

### zia

read zia resources

```
zscalerctl zia
```

#### zia url-lookup

look up URL categories for one or more URLs

```
zscalerctl zia url-lookup <url> [url...]
```

### zidentity

read zidentity resources

```
zscalerctl zidentity
```

### zpa

read zpa resources

```
zscalerctl zpa
```

### ztw

read ztw resources

```
zscalerctl ztw
```


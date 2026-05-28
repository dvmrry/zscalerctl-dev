package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

var (
	completionCommands  = []string{"doctor", "auth", "config", "schema", "dump", "completion", "version", "zia", "zpa", "help"}
	completionFlags     = []string{"--profile", "--format", "--output", "--timeout", "--redaction", "--color", "--no-color", "--no-cache"}
	completionFormats   = []string{"table", "json", "yaml", "ndjson"}
	completionRedaction = []string{"standard", "share", "paranoid"}
	completionColors    = []string{"auto", "always", "never"}
	completionProducts  = []string{"zia", "zpa", "zia,zpa"}
	completionShells    = []string{"bash", "zsh", "fish"}
)

func (a *App) runCompletion(args []string) error {
	if len(args) != 1 {
		return UsageError{Message: "usage: zscalerctl completion bash|zsh|fish"}
	}
	body, err := completionScript(args[0])
	if err != nil {
		return err
	}
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	return renderer.WriteText(a.out, output.NewSafeText(body))
}

func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletion(), nil
	case "zsh":
		return zshCompletion(), nil
	case "fish":
		return fishCompletion(), nil
	default:
		return "", UsageError{Message: "usage: zscalerctl completion bash|zsh|fish"}
	}
}

func bashCompletion() string {
	return fmt.Sprintf(`# bash completion for zscalerctl
_zscalerctl()
{
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  case "$prev" in
    --format) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    --redaction) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    --color) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    --products) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    --resources) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    completion) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    auth) COMPREPLY=( $(compgen -W "status" -- "$cur") ); return ;;
    config) COMPREPLY=( $(compgen -W "show" -- "$cur") ); return ;;
    schema) COMPREPLY=( $(compgen -W "list" -- "$cur") ); return ;;
    dump) COMPREPLY=( $(compgen -W "--out --products --resources --continue-on-error" -- "$cur") ); return ;;
    zia) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    zpa) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
    %s) COMPREPLY=( $(compgen -W "%s" -- "$cur") ); return ;;
  esac

  COMPREPLY=( $(compgen -W "%s %s" -- "$cur") )
}
complete -F _zscalerctl zscalerctl
`,
		words(completionFormats),
		words(completionRedaction),
		words(completionColors),
		words(completionProducts),
		words(dumpResourceNames()),
		words(completionShells),
		words(resourceNames(resources.ProductZIA)),
		words(resourceNames(resources.ProductZPA)),
		bashCasePatterns(allResourceNames()),
		words(operationNames()),
		words(completionFlags),
		words(completionCommands),
	)
}

func zshCompletion() string {
	return fmt.Sprintf(`#compdef zscalerctl

_zscalerctl() {
  local -a commands flags formats redactions colors products dump_resources shells zia_resources zpa_resources operations dump_flags
  commands=(%s)
  flags=(%s)
  formats=(%s)
  redactions=(%s)
  colors=(%s)
  products=(%s)
  dump_resources=(%s)
  shells=(%s)
  zia_resources=(%s)
  zpa_resources=(%s)
  operations=(%s)
  dump_flags=(--out --products --resources --continue-on-error)

  case ${words[CURRENT-1]} in
    --format) compadd -- "${formats[@]}"; return ;;
    --redaction) compadd -- "${redactions[@]}"; return ;;
    --color) compadd -- "${colors[@]}"; return ;;
    --products) compadd -- "${products[@]}"; return ;;
    --resources) compadd -- "${dump_resources[@]}"; return ;;
    completion) compadd -- "${shells[@]}"; return ;;
    auth) compadd -- status; return ;;
    config) compadd -- show; return ;;
    schema) compadd -- list; return ;;
    dump) compadd -- "${dump_flags[@]}"; return ;;
    zia) compadd -- "${zia_resources[@]}"; return ;;
    zpa) compadd -- "${zpa_resources[@]}"; return ;;
    %s) compadd -- "${operations[@]}"; return ;;
  esac

  compadd -- "${flags[@]}" "${commands[@]}"
}

_zscalerctl "$@"
`,
		words(completionCommands),
		words(completionFlags),
		words(completionFormats),
		words(completionRedaction),
		words(completionColors),
		words(completionProducts),
		words(dumpResourceNames()),
		words(completionShells),
		words(resourceNames(resources.ProductZIA)),
		words(resourceNames(resources.ProductZPA)),
		words(operationNames()),
		zshCasePatterns(allResourceNames()),
	)
}

func fishCompletion() string {
	return fmt.Sprintf(`# fish completion for zscalerctl
complete -c zscalerctl -f
complete -c zscalerctl -l profile -r -d 'Profile name'
complete -c zscalerctl -l format -x -a '%s' -d 'Output format'
complete -c zscalerctl -l output -r -d 'Output path'
complete -c zscalerctl -l timeout -r -d 'Request timeout'
complete -c zscalerctl -l redaction -x -a '%s' -d 'Redaction mode'
complete -c zscalerctl -l color -x -a '%s' -d 'Color mode'
complete -c zscalerctl -l no-color -d 'Disable color output'
complete -c zscalerctl -l no-cache -d 'Bypass API cache where supported'
complete -c zscalerctl -n '__fish_use_subcommand' -a '%s'
complete -c zscalerctl -n '__fish_seen_subcommand_from completion' -a '%s'
complete -c zscalerctl -n '__fish_seen_subcommand_from auth' -a 'status'
complete -c zscalerctl -n '__fish_seen_subcommand_from config' -a 'show'
complete -c zscalerctl -n '__fish_seen_subcommand_from schema' -a 'list'
complete -c zscalerctl -n '__fish_seen_subcommand_from dump' -a '--out --products --resources --continue-on-error'
complete -c zscalerctl -n '__fish_seen_subcommand_from dump' -l products -x -a '%s'
complete -c zscalerctl -n '__fish_seen_subcommand_from dump' -l resources -x -a '%s'
complete -c zscalerctl -n '__fish_seen_subcommand_from zia' -a '%s'
complete -c zscalerctl -n '__fish_seen_subcommand_from zpa' -a '%s'
complete -c zscalerctl -n '__fish_seen_subcommand_from %s' -a '%s'
`,
		words(completionFormats),
		words(completionRedaction),
		words(completionColors),
		words(completionCommands),
		words(completionShells),
		words(completionProducts),
		words(dumpResourceNames()),
		words(resourceNames(resources.ProductZIA)),
		words(resourceNames(resources.ProductZPA)),
		words(allResourceNames()),
		words(operationNames()),
	)
}

func resourceNames(product resources.Product) []string {
	var names []string
	for _, spec := range resources.Catalog() {
		if spec.Product == product {
			names = append(names, spec.Name)
		}
	}
	sort.Strings(names)
	return names
}

func allResourceNames() []string {
	seen := map[string]struct{}{}
	for _, spec := range resources.Catalog() {
		seen[spec.Name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func dumpResourceNames() []string {
	seen := map[string]struct{}{}
	for _, spec := range resources.Catalog() {
		if !resourceSupportsDump(spec) {
			continue
		}
		seen[spec.Name] = struct{}{}
		seen[string(spec.Product)+"/"+spec.Name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func operationNames() []string {
	seen := map[string]struct{}{}
	var names []string
	for _, spec := range resources.Catalog() {
		for _, op := range spec.Operations {
			if op.Capability == resources.CapabilityRead {
				if _, ok := seen[op.Name]; ok {
					continue
				}
				seen[op.Name] = struct{}{}
				names = append(names, op.Name)
			}
		}
	}
	return names
}

func bashCasePatterns(values []string) string {
	if len(values) == 0 {
		return "__zscalerctl_no_resources__"
	}
	return strings.Join(values, "|")
}

func zshCasePatterns(values []string) string {
	return bashCasePatterns(values)
}

func words(values []string) string {
	return strings.Join(values, " ")
}

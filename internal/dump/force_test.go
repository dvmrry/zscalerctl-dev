package dump

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

func TestPrepareOutputDirPreCanceledPreservesValidDump(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)
	stalePath := filepath.Join(dir, "stale.txt")
	if err := os.WriteFile(stalePath, []byte("keep"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", stalePath, err)
	}
	manifestBefore := readFile(t, filepath.Join(dir, "manifest.json"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := PrepareOutputDir(ctx, dir, true)
	if err != context.Canceled {
		t.Fatalf("PrepareOutputDir(%q, pre-canceled) error = %v, want identity %v", dir, err, context.Canceled)
	}
	if got := readFile(t, filepath.Join(dir, "manifest.json")); got != manifestBefore {
		t.Errorf("manifest after pre-canceled PrepareOutputDir = %q, want unchanged %q", got, manifestBefore)
	}
	if got := readFile(t, stalePath); got != "keep" {
		t.Errorf("stale file after pre-canceled PrepareOutputDir = %q, want %q", got, "keep")
	}
}

func TestPrepareOutputDirForceClearsValidDump(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)

	if err := PrepareOutputDir(context.Background(), dir, true); err != nil {
		t.Fatalf("PrepareOutputDir(%q, force) error = %v, want nil", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", dir, err)
	}
	if len(entries) != 0 {
		t.Errorf("os.ReadDir(%q) = %v, want empty validated directory", dir, entries)
	}
}

func TestPrepareOutputDirRejectsForeignArtifactFile(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)
	foreignPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(foreignPath, []byte("keep"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", foreignPath, err)
	}

	err := PrepareOutputDir(context.Background(), dir, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, foreign file) error = %v, want ErrUnsafePath", dir, err)
	}
	if got := readFile(t, foreignPath); got != "keep" {
		t.Errorf("foreign file after rejected --force = %q, want %q", got, "keep")
	}
}

func TestPrepareOutputDirRejectsResourceRecordCountMismatch(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	entry := projectedDumpEntry(t, "zia", "locations", nil)
	if err := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{entry}}); err != nil {
		t.Fatalf("Write(%q) error = %v, want nil", dir, err)
	}
	resourcePath := filepath.Join(dir, "resources", "zia", "locations.json")
	if err := os.WriteFile(resourcePath, []byte("[{}]\n"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", resourcePath, err)
	}

	err := PrepareOutputDir(context.Background(), dir, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, count mismatch) error = %v, want ErrUnsafePath", dir, err)
	}
	if got := readFile(t, resourcePath); got != "[{}]\n" {
		t.Errorf("resource after rejected --force = %q, want unchanged", got)
	}
}

func TestPrepareOutputDirRejectsManifestShapeMismatch(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	entry := projectedDumpEntry(t, "zia", "locations", nil)
	if err := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{entry}}); err != nil {
		t.Fatalf("Write(%q) error = %v, want nil", dir, err)
	}
	rewriteForceManifest(t, dir, func(manifest *Manifest) {
		manifest.Resources[0].Shape = "singleton"
	})
	resourcePath := filepath.Join(dir, "resources", "zia", "locations.json")
	resourceBefore := readFile(t, resourcePath)

	err := PrepareOutputDir(context.Background(), dir, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, shape mismatch) error = %v, want ErrUnsafePath", dir, err)
	}
	if got := readFile(t, resourcePath); got != resourceBefore {
		t.Errorf("resource after rejected shape mismatch changed\n got: %s\nwant: %s", got, resourceBefore)
	}
}

func TestValidateArtifactRejectsUnknownManifestShape(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	entry := projectedDumpEntry(t, "zia", "locations", nil)
	if err := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{entry}}); err != nil {
		t.Fatalf("Write(%q) error = %v, want nil", dir, err)
	}
	rewriteForceManifest(t, dir, func(manifest *Manifest) {
		manifest.Resources[0].Shape = "attacker"
	})

	_, err := ValidateArtifactContext(context.Background(), dir)
	if !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("ValidateArtifactContext(%q, unknown shape) error = %v, want ErrInvalidArtifact", dir, err)
	}
}

func TestPrepareOutputDirRejectsPartialDump(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "partial")
	result := Result{Errors: []ResourceError{
		NewResourceError("zia", "rule-labels", "list", "live_access_failed"),
	}}
	if err := Write(dir, redact.ModeStandard, result); err != nil {
		t.Fatalf("Write(%q, partial) error = %v, want nil", dir, err)
	}
	manifestBefore := readFile(t, filepath.Join(dir, "manifest.json"))

	err := PrepareOutputDir(context.Background(), dir, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, partial) error = %v, want ErrUnsafePath", dir, err)
	}
	if got := readFile(t, filepath.Join(dir, "manifest.json")); got != manifestBefore {
		t.Errorf("partial manifest after rejected --force changed\n got: %s\nwant: %s", got, manifestBefore)
	}
	err = PublishContext(context.Background(), dir, redact.ModeShare, Result{}, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PublishContext(%q, partial force) error = %v, want ErrUnsafePath", dir, err)
	}
	if got := readFile(t, filepath.Join(dir, "manifest.json")); got != manifestBefore {
		t.Errorf("partial manifest after rejected forced publication changed\n got: %s\nwant: %s", got, manifestBefore)
	}
}

func TestPublishReplacementRejectsPostValidationInsertion(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	staging := filepath.Join(parent, "staging")
	if err := Write(target, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, target) error = %v, want nil", target, err)
	}
	if err := Write(staging, redact.ModeShare, Result{}); err != nil {
		t.Fatalf("Write(%q, staging) error = %v, want nil", staging, err)
	}
	foreignPath := filepath.Join(target, "must-survive.txt")
	var hookErr error
	err := publishReplacingDirectoryWithHooks(
		context.Background(),
		staging,
		target,
		true,
		publishReplacementHooks{
			beforeExchange: func() {
				hookErr = os.WriteFile(foreignPath, []byte("keep"), filePerm)
			},
			exchange: exchangeDirectories,
		},
	)
	if hookErr != nil {
		t.Fatalf("post-validation insertion error = %v, want nil", hookErr)
	}
	if errors.Is(err, ErrAtomicReplaceUnsupported) {
		t.Skip("directory exchange is unsupported on this filesystem")
	}
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("publishReplacingDirectoryWithHooks(post-validation insertion) error = %v, want ErrUnsafePath", err)
	}
	if got := readFile(t, foreignPath); got != "keep" {
		t.Errorf("foreign file after rejected replacement = %q, want keep", got)
	}
	var manifest Manifest
	readJSON(t, filepath.Join(target, "manifest.json"), &manifest)
	if manifest.Redaction != string(redact.ModeStandard) {
		t.Errorf("target redaction after rollback = %q, want standard", manifest.Redaction)
	}
}

func TestPublishReplacementRejectsSameNameSubstitutionBeforeCleanup(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	staging := filepath.Join(parent, "staging")
	if err := Write(target, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, target) error = %v, want nil", target, err)
	}
	if err := Write(staging, redact.ModeShare, Result{}); err != nil {
		t.Fatalf("Write(%q, staging) error = %v, want nil", staging, err)
	}
	const sentinel = "foreign replacement must survive\n"
	var hookErr error
	err := publishReplacingDirectoryWithHooks(
		context.Background(),
		staging,
		target,
		true,
		publishReplacementHooks{
			beforeQuarantineMove: func() {
				manifestPath := filepath.Join(staging, "manifest.json")
				if removeErr := os.Remove(manifestPath); removeErr != nil {
					hookErr = removeErr
					return
				}
				hookErr = os.WriteFile(manifestPath, []byte(sentinel), filePerm)
			},
			exchange: exchangeDirectories,
		},
	)
	if errors.Is(err, ErrAtomicReplaceUnsupported) {
		t.Skip("directory exchange is unsupported on this filesystem")
	}
	if hookErr != nil {
		t.Fatalf("same-name substitution error = %v, want nil", hookErr)
	}
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("publishReplacingDirectoryWithHooks(same-name substitution) error = %v, want ErrUnsafePath", err)
	}
	if got := readFile(t, filepath.Join(target, "manifest.json")); got != sentinel {
		t.Errorf("same-name foreign replacement after rollback = %q, want %q", got, sentinel)
	}
}

func TestPublishReplacementFailsClosedWithoutAtomicExchange(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	staging := filepath.Join(parent, "staging")
	if err := Write(target, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, target) error = %v, want nil", target, err)
	}
	if err := Write(staging, redact.ModeShare, Result{}); err != nil {
		t.Fatalf("Write(%q, staging) error = %v, want nil", staging, err)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", target, err)
	}
	stagingInfo, err := os.Stat(staging)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", staging, err)
	}

	err = publishReplacingDirectoryWithHooks(
		context.Background(),
		staging,
		target,
		true,
		publishReplacementHooks{
			exchange: func(string, string) (bool, error) { return false, nil },
		},
	)
	if !errors.Is(err, ErrAtomicReplaceUnsupported) {
		t.Fatalf("publishReplacingDirectoryWithHooks(unsupported exchange) error = %v, want ErrAtomicReplaceUnsupported", err)
	}
	currentTarget, targetErr := os.Stat(target)
	currentStaging, stagingErr := os.Stat(staging)
	if targetErr != nil || !os.SameFile(targetInfo, currentTarget) {
		t.Errorf("target after unsupported exchange = (%v, %v), want unchanged", currentTarget, targetErr)
	}
	if stagingErr != nil || !os.SameFile(stagingInfo, currentStaging) {
		t.Errorf("staging after unsupported exchange = (%v, %v), want unchanged", currentStaging, stagingErr)
	}
}

func TestPublishEmptyDirectoryFailsClosedWithoutAtomicExchange(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	staging := filepath.Join(parent, "staging")
	if err := os.MkdirAll(filepath.Join(target, "empty", "nested"), dirPerm); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", target, err)
	}
	if err := Write(staging, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, staging) error = %v, want nil", staging, err)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", target, err)
	}

	err = publishReplacingDirectoryWithHooks(
		context.Background(),
		staging,
		target,
		false,
		publishReplacementHooks{
			exchange: func(string, string) (bool, error) { return false, nil },
		},
	)
	if !errors.Is(err, ErrAtomicReplaceUnsupported) {
		t.Fatalf("publishReplacingDirectoryWithHooks(empty unsupported exchange) error = %v, want ErrAtomicReplaceUnsupported", err)
	}
	currentTarget, statErr := os.Stat(target)
	if statErr != nil || !os.SameFile(targetInfo, currentTarget) {
		t.Errorf("empty target after unsupported exchange = (%v, %v), want unchanged", currentTarget, statErr)
	}
	if _, err := os.Stat(filepath.Join(target, "empty", "nested")); err != nil {
		t.Errorf("nested empty target after unsupported exchange error = %v, want nil", err)
	}
}

func TestPublishContextRejectsUnwritableReplacementBeforeCommit(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	entry := projectedDumpEntry(t, "zia", "locations", nil)
	if err := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{entry}}); err != nil {
		t.Fatalf("Write(%q) error = %v, want nil", dir, err)
	}
	restrictedDir := filepath.Join(dir, "resources", "zia")
	if err := os.Chmod(restrictedDir, 0o500); err != nil {
		t.Fatalf("os.Chmod(%q, 0500) error = %v, want nil", restrictedDir, err)
	}
	defer os.Chmod(restrictedDir, dirPerm)
	oldInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", dir, err)
	}

	err = PublishContext(context.Background(), dir, redact.ModeShare, Result{}, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PublishContext(%q, unwritable target) error = %v, want ErrUnsafePath", dir, err)
	}
	newInfo, statErr := os.Stat(dir)
	if statErr != nil || !os.SameFile(oldInfo, newInfo) {
		t.Errorf("target after rejected unwritable replacement = (%v, %v), want unchanged", newInfo, statErr)
	}
}

func TestPrepareOutputDirDoesNotDeleteReplacementDirectory(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)
	validatedDir := dir + "-validated"
	replacementFile := filepath.Join(dir, "must-survive.txt")
	var hookErr error

	err := prepareOutputDir(context.Background(), dir, true, prepareOutputDirHooks{
		beforeClear: func() {
			if renameErr := os.Rename(dir, validatedDir); renameErr != nil {
				hookErr = renameErr
				return
			}
			if mkdirErr := os.Mkdir(dir, dirPerm); mkdirErr != nil {
				hookErr = mkdirErr
				return
			}
			hookErr = os.WriteFile(replacementFile, []byte("keep"), filePerm)
		},
	})
	if hookErr != nil {
		t.Fatalf("replacement boundary setup error = %v, want nil", hookErr)
	}
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("prepareOutputDir(replaced target) error = %v, want ErrUnsafePath", err)
	}
	if got := readFile(t, replacementFile); got != "keep" {
		t.Errorf("replacement file after force race = %q, want %q", got, "keep")
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		t.Errorf("replacement directory after force race = (%v, %v), want existing directory", info, statErr)
	}
}

func TestPrepareOutputDirDoesNotDeleteFileSubstitutedAfterClearing(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)
	validatedDir := dir + "-validated"
	var hookErr error

	err := prepareOutputDir(context.Background(), dir, true, prepareOutputDirHooks{
		beforeFinalCheck: func() {
			if renameErr := os.Rename(dir, validatedDir); renameErr != nil {
				hookErr = renameErr
				return
			}
			hookErr = os.WriteFile(dir, []byte("keep"), filePerm)
		},
	})
	if hookErr != nil {
		t.Fatalf("final replacement boundary setup error = %v, want nil", hookErr)
	}
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("prepareOutputDir(final replaced target) error = %v, want ErrUnsafePath", err)
	}
	if got := readFile(t, dir); got != "keep" {
		t.Errorf("replacement file after force race = %q, want %q", got, "keep")
	}
}

func TestPrepareOutputDirNoForceDoesNotInspect(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "missing", "dump")
	if err := PrepareOutputDir(context.Background(), dir, false); err != nil {
		t.Fatalf("PrepareOutputDir(%q, no force) error = %v, want nil", dir, err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", dir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := PrepareOutputDir(ctx, "", false); err != context.Canceled {
		t.Fatalf("PrepareOutputDir(empty, pre-canceled no force) error = %v, want identity %v", err, context.Canceled)
	}
}

func TestPrepareOutputDirRejectsUnownedDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "not-a-dump")
	if err := os.Mkdir(dir, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", dir, err)
	}
	notesPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notesPath, []byte("keep"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", notesPath, err)
	}

	err := PrepareOutputDir(context.Background(), dir, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, unowned) error = %v, want ErrUnsafePath", dir, err)
	}
	if got := readFile(t, notesPath); got != "keep" {
		t.Errorf("unowned directory sentinel = %q, want %q", got, "keep")
	}
}

func TestPrepareOutputDirRejectsManifestSchemaPrefixSpoof(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "not-a-dump")
	if err := os.Mkdir(dir, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", dir, err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(
		manifestPath,
		[]byte(`{"schema":"zscalerctl.dump.manifest.attacker"}`),
		filePerm,
	); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", manifestPath, err)
	}
	foreignPath := filepath.Join(dir, "must-survive.txt")
	if err := os.WriteFile(foreignPath, []byte("keep"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", foreignPath, err)
	}

	err := PrepareOutputDir(context.Background(), dir, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, prefix spoof) error = %v, want ErrUnsafePath", dir, err)
	}
	if got := readFile(t, foreignPath); got != "keep" {
		t.Errorf("foreign file after rejected prefix spoof = %q, want %q", got, "keep")
	}
}

func TestPublishContextForceReplacesValidDumpAsDirectory(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)
	oldInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", dir, err)
	}
	if err := PublishContext(context.Background(), dir, redact.ModeShare, Result{}, true); err != nil {
		if errors.Is(err, ErrAtomicReplaceUnsupported) {
			newInfo, statErr := os.Stat(dir)
			if statErr != nil || !os.SameFile(oldInfo, newInfo) {
				t.Errorf("unsupported forced replacement changed target = (%v, %v)", newInfo, statErr)
			}
			return
		}
		t.Fatalf("PublishContext(%q, force) error = %v, want nil", dir, err)
	}
	newInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%q after force) error = %v, want nil", dir, err)
	}
	if os.SameFile(oldInfo, newInfo) {
		t.Errorf("PublishContext(%q, force) directory identity unchanged, want staged replacement", dir)
	}
	var manifest Manifest
	readJSON(t, filepath.Join(dir, "manifest.json"), &manifest)
	if manifest.Redaction != string(redact.ModeShare) {
		t.Errorf("forced manifest redaction = %q, want %q", manifest.Redaction, redact.ModeShare)
	}
}

func TestPrepareOutputDirRejectsFinalSymlink(t *testing.T) {
	t.Parallel()

	target := validForceDumpDir(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("os.Symlink(%q, %q) error = %v; symlinks unavailable", target, link, err)
	}

	err := PrepareOutputDir(context.Background(), link, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, symlink) error = %v, want ErrUnsafePath", link, err)
	}
	if _, err := os.Stat(filepath.Join(target, "manifest.json")); err != nil {
		t.Errorf("target manifest after rejected symlink = %v, want nil", err)
	}
}

func TestPrepareOutputDirLeavesEmptyDirectoryAlone(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(dir, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", dir, err)
	}
	if err := PrepareOutputDir(context.Background(), dir, true); err != nil {
		t.Fatalf("PrepareOutputDir(%q, empty) error = %v, want nil", dir, err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("os.Stat(%q) = (%v, %v), want existing directory", dir, info, err)
	}
}

func validForceDumpDir(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "dump")
	if err := Write(dir, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, empty result) error = %v, want nil", dir, err)
	}
	return dir
}

func rewriteForceManifest(t *testing.T, dir string, mutate func(*Manifest)) {
	t.Helper()
	path := filepath.Join(dir, "manifest.json")
	var manifest Manifest
	readJSON(t, path, &manifest)
	mutate(&manifest)
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(manifest) error = %v, want nil", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}

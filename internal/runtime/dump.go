package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// DumpProgressFunc observes value-free dump collection progress.
type DumpProgressFunc func(done, total int, product resources.Product, resource string)

// DumpCollectOptions configures one dump collection run.
type DumpCollectOptions struct {
	ContinueOnError bool
	Progress        DumpProgressFunc
}

// DumpCollector is the trusted live dump collection facade.
type DumpCollector struct {
	reader    browser.RecordReader
	catalog   resources.ResourceCatalog
	redaction redact.Mode
}

type resourceSessionProvider interface {
	Session(context.Context, resources.Product) (zscaler.ResourceSession, error)
}

// NewDumpCollector loads runtime config, resolves credentials, constructs the
// SDK-backed read-only reader, and returns a dump collection facade.
func NewDumpCollector(ctx context.Context, opts Options) (*DumpCollector, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	env := append([]string(nil), opts.Env...)
	loadConfig := opts.loadConfig
	if loadConfig == nil {
		loadConfig = config.LoadConfig
	}
	cfg, err := loadConfig(env, config.LoadOptions{
		Profile:    opts.Profile,
		ConfigPath: opts.ConfigPath,
	})
	if err != nil {
		return nil, err
	}
	return NewDumpCollectorFromConfig(ctx, cfg, opts)
}

// NewDumpCollectorFromConfig resolves credentials from an already-loaded
// effective config, constructs the SDK-backed read-only reader, and returns a
// dump collection facade.
func NewDumpCollectorFromConfig(
	ctx context.Context,
	cfg config.Config,
	opts Options,
) (*DumpCollector, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reader, err := newReaderFromConfig(ctx, &cfg, opts)
	if err != nil {
		return nil, err
	}
	return newDumpCollectorFromReader(reader, catalogFromOptions(opts.Catalog), cfg.Defaults.Redaction), nil
}

// NewDumpCollectorFromReader constructs a dump collector around an
// already-trusted read-only record reader.
func NewDumpCollectorFromReader(
	reader browser.RecordReader,
	catalog resources.ResourceCatalog,
	mode redact.Mode,
) *DumpCollector {
	return newDumpCollectorFromReader(reader, catalog, mode)
}

// Collect reads, projects, and verifies the selected resource specs into a dump
// result. Fatal runtime failures return errors; per-resource failures are
// recorded when ContinueOnError is set.
func (c *DumpCollector) Collect(
	ctx context.Context,
	specs []resources.ResourceSpec,
	opts DumpCollectOptions,
) (dump.Result, error) {
	result := dump.Result{}
	if c == nil {
		return result, errors.New("dump collector is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := resources.AssertReadOnly(c.catalog...); err != nil {
		return result, err
	}
	selectedSpecs := copyResourceSpecs(specs)
	if err := resources.AssertReadOnly(selectedSpecs...); err != nil {
		return result, err
	}

	readers := make(map[resources.Product]browser.RecordReader)
	for i, spec := range selectedSpecs {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		reader, ok := readers[spec.Product]
		if !ok {
			var cleanup func()
			var err error
			reader, cleanup, err = c.readerForProduct(ctx, spec.Product)
			if err != nil {
				return result, err
			}
			readers[spec.Product] = reader
			defer cleanup()
		}
		if opts.Progress != nil {
			opts.Progress(i+1, len(selectedSpecs), spec.Product, spec.Name)
		}
		if spec.SupportsReadOperation("show") {
			if err := c.collectShow(ctx, reader, spec, opts.ContinueOnError, &result); err != nil {
				return result, err
			}
			continue
		}
		if err := c.collectList(ctx, reader, spec, opts.ContinueOnError, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (c *DumpCollector) readerForProduct(
	ctx context.Context,
	product resources.Product,
) (browser.RecordReader, func(), error) {
	if c.reader == nil {
		return nil, nil, browser.ErrMissingReader
	}
	provider, ok := c.reader.(resourceSessionProvider)
	if !ok {
		return c.reader, func() {}, nil
	}
	session, err := provider.Session(ctx, product)
	if err != nil {
		if errors.Is(err, zscaler.ErrUnsupportedResource) {
			return c.reader, func() {}, nil
		}
		return nil, nil, err
	}
	if session == nil {
		return nil, nil, errors.New("reader session provider returned nil session")
	}
	return session, session.Close, nil
}

func (c *DumpCollector) collectShow(
	ctx context.Context,
	reader browser.RecordReader,
	spec resources.ResourceSpec,
	continueOnError bool,
	result *dump.Result,
) error {
	record, err := reader.Show(ctx, spec.Product, spec.Name)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if continueOnError {
			result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, "show", "show_failed"))
			return nil
		}
		return fmt.Errorf("dump %s/%s show failed: %w", spec.Product, spec.Name, err)
	}
	projected, report, err := resources.ProjectRecordAndVerify(spec, c.redaction, record)
	if err != nil {
		operation, kind := dumpProjectionErrorKind(err)
		if continueOnError {
			result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, operation, kind))
			return nil
		}
		return fmt.Errorf("dump %s/%s %s failed: %w", spec.Product, spec.Name, operation, err)
	}
	result.Entries = append(result.Entries, dump.ResourceDump{
		Spec:    spec,
		Record:  &projected,
		Reports: []resources.ProjectionReport{report},
	})
	return nil
}

func (c *DumpCollector) collectList(
	ctx context.Context,
	reader browser.RecordReader,
	spec resources.ResourceSpec,
	continueOnError bool,
	result *dump.Result,
) error {
	records, err := reader.List(ctx, spec.Product, spec.Name)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if continueOnError {
			result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, "list", "list_failed"))
			return nil
		}
		return fmt.Errorf("dump %s/%s list failed: %w", spec.Product, spec.Name, err)
	}
	projected, reports, err := resources.ProjectRecordsAndVerify(spec, c.redaction, records)
	if err != nil {
		operation, kind := dumpProjectionErrorKind(err)
		if continueOnError {
			result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, operation, kind))
			return nil
		}
		return fmt.Errorf("dump %s/%s %s failed: %w", spec.Product, spec.Name, operation, err)
	}
	result.Entries = append(result.Entries, dump.ResourceDump{
		Spec:    spec,
		Records: projected,
		Reports: reports,
	})
	return nil
}

func dumpProjectionErrorKind(err error) (operation string, kind string) {
	if errors.Is(err, resources.ErrUnexpectedField) {
		return "validate", "subset_failed"
	}
	return "project", "projection_failed"
}

func newDumpCollectorFromReader(
	reader browser.RecordReader,
	catalog resources.ResourceCatalog,
	mode redact.Mode,
) *DumpCollector {
	return &DumpCollector{
		reader:    reader,
		catalog:   copyCatalog(catalog),
		redaction: redact.EffectiveMode(mode),
	}
}

func copyResourceSpecs(specs []resources.ResourceSpec) []resources.ResourceSpec {
	out := make([]resources.ResourceSpec, len(specs))
	for i, spec := range specs {
		out[i] = copyResourceSpec(spec)
	}
	return out
}

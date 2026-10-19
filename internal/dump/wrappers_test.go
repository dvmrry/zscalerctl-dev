package dump

import (
	"context"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// Context-free forms of the dump writers, for tests.

func writeJSONFile(path string, mode redact.Mode, value safeJSON) error {
	return writeJSONFileContext(context.Background(), path, mode, value)
}

func writeFileExclusive(path string, body []byte) error {
	return writeFileExclusiveContext(context.Background(), path, body)
}

func ensureDir(dir string) error {
	return ensureDirContext(context.Background(), dir)
}

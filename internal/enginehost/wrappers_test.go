package enginehost

import "errors"

func isSessionClosingError(err error) bool {
	return errors.Is(err, errSessionClosing)
}

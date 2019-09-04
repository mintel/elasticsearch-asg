package health

import (
	"time"
)

func setTestTimeout() func() {
	originalTimeout := DefaultHTTPTimeout
	DefaultHTTPTimeout = 500 * time.Millisecond

	return func() {
		DefaultHTTPTimeout = originalTimeout
	}
}

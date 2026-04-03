package crash

import (
	"os"
	"testing"
)

func TestCrash(t *testing.T) {
	os.Exit(1)
}

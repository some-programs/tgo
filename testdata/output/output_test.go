package output

import (
	"fmt"
	"testing"
)

func TestOutput(t *testing.T) {
	fmt.Println("This is some test output.")
	t.Log("This is a test log message.")
}

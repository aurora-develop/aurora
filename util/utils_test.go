package util

import (
	"fmt"
	"testing"
)

func TestRandomHexadecimalString(t *testing.T) {
	var str = RandomHexadecimalString()
	fmt.Println(str)
}

package util

import (
	"fmt"
	"testing"
)

func TestRandomHexadecimalString(t *testing.T) {
	var str = RandomHexadecimalString()
	fmt.Println(str)
}

func TestRandomIntStr(t *testing.T) {
	var str = RandomIntStr(39)
	fmt.Println(str)
}

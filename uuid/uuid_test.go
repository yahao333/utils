package uuid

import (
	"fmt"
	"testing"
)

func TestAll(t *testing.T) {
	rand := NewV4().String()
	fmt.Println(rand)
}

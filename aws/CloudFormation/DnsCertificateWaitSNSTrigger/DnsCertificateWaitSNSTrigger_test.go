package main

import (
	"fmt"
	"testing"
)

func TestResourceName(t *testing.T) {
	fmt.Println(executionName())
}

func TestRoundDecoding(t *testing.T) {
	data := map[string]interface{}{}
	round, ok := data["Round"].(int)
	if !ok {
		round = 0
	}
	fmt.Println(round)
}

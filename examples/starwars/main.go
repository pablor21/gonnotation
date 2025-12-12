package main

import (
	"encoding/json"

	"github.com/pablor21/gonnotation"
	"github.com/pablor21/gonnotation/config"
)

func main() {
	c := config.NewDefaultConfig()
	c.Scanning.Packages = []string{"./models/*.go"}

	res, err := gonnotation.ProcessWithConfig(c)
	if err != nil {
		panic(err)
	}
	bytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		panic(err)
	}
	println(string(bytes))
}

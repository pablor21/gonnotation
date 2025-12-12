package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/pablor21/gonnotation"
	"github.com/pablor21/gonnotation/config"
)

func main() {
	c := config.NewDefaultConfig()
	c.Scanning.Packages = []string{"./models/*.go", "./other/*.go"}

	res, err := gonnotation.ProcessWithConfig(c)
	if err != nil {
		panic(err)
	}
	bytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		panic(err)
	}
	str := string(bytes)
	println(str)
	// save to output.json
	err = ioutil.WriteFile("output.json", bytes, 0644)
	if err != nil {
		panic(err)
	}
}

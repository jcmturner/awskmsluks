package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type config struct {
	CMKARN           string
	Production       bool
	KeyArchiveBucket string
}

func loadConfig(path string) (config, error) {
	var c config
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("cannot read config file (%s): %v", path, err)
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return c, fmt.Errorf("configuration file (%s) could not be parsed: %v", path, err)
	}
	return c, nil
}

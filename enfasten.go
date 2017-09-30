package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"gopkg.in/yaml.v2"
)

type config struct {
	InputFolder  string
	OutputFolder string
	Widths       []int
}

func readConfig(basePath string) (conf config, err error) {
	f, err := os.OpenFile(path.Join(basePath, "enfasten.yml"), os.O_RDONLY, 0)
	if err != nil {
		return
	}

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(bytes, &conf)
	if err != nil {
		return
	}
	return
}

func main() {
	basePath := flag.String("basepath", ".", "The folder in which to search for enfasten.yml")
	flag.Parse()
	conf, err := readConfig(*basePath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v", conf)
}

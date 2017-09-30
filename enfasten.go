package main

import (
	"flag"
	"fmt"
	"github.com/bmatcuk/doublestar"
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
	basePath     string
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

func discoverImages(conf *config) {
	matches, err := doublestar.Glob(path.Join(basePath, "**/*.{png,jpg}"))
	fmt.Printf("%v", matches)
}

func main() {
	basePath := flag.String("basepath", ".", "The folder in which to search for enfasten.yml")
	flag.Parse()
	conf, err := readConfig(*basePath)
	if err != nil {
		log.Fatal(err)
	}
	conf.basePath = *basePath
	fmt.Printf("%+v", conf)
	discoverImages(&conf)
}

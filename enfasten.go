package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"github.com/bmatcuk/doublestar"
	"io"
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
	defer f.Close()
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

func hashFile(path string) (hash []byte, err error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return
	}

	hash = h.Sum(nil)
	return
}

type foundImage struct {
	Path string
	Hash []byte
}

func discoverImages(inFolder string) (results []foundImage, err error) {
	matches, err := doublestar.Glob(path.Join(inFolder, "**/*.{png,jpg}"))
	if err != nil {
		return
	}
	fmt.Printf("%v\n", matches)

	for _, path := range matches {
		var hash []byte
		hash, err = hashFile(path)
		if err != nil {
			return
		}
		results = append(results, foundImage{path, hash})
	}
	fmt.Printf("%v\n", results)
	return
}

func main() {
	basePath := flag.String("basepath", ".", "The folder in which to search for enfasten.yml")
	flag.Parse()
	conf, err := readConfig(*basePath)
	if err != nil {
		log.Fatal(err)
	}
	conf.basePath = *basePath
	fmt.Printf("%+v\n", conf)
	discoverImages(path.Join(conf.basePath, conf.InputFolder))
}

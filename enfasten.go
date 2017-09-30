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
	ImageFolder  string
	ManifestFile string
	Widths       []int
	basePath     string
}

type foundImage struct {
	Path string
	Hash []byte
}

type builtImageFile struct {
	FileName string
	Width    int
	Height   int
}

type builtImage struct {
	Width  int
	Height int
	Files  []builtImageFile
}

func readFileBytes(path string) (bytes []byte, err error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	defer f.Close()
	if err != nil {
		return
	}

	bytes, err = ioutil.ReadAll(f)
	return
}

func readConfig(basePath string) (conf config, err error) {
	conf = config{
		InputFolder:  "_site",
		OutputFolder: "_fastsite",
		ImageFolder:  "assets/images",
		ManifestFile: "enfasten_manifest.yml",
		Widths:       []int{},
	}

	bytes, err := readFileBytes(path.Join(basePath, "enfasten.yml"))
	err = yaml.Unmarshal(bytes, &conf)
	return
}

func readManifest(manifestPath string) (manifest map[string]builtImage, err error) {
	if _, statError := os.Stat(manifestPath); os.IsNotExist(statError) {
		log.Print("Can't find manifest, starting with an empty one")
		return // no manifest, starting with an empty one
	}
	bytes, err := readFileBytes(manifestPath)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(bytes, &manifest)
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

func getSlug(imagePath string, hash []byte) string {
	_, baseName := path.Split(imagePath)
	extension := path.Ext(baseName)
	fileName := baseName[0 : len(baseName)-len(extension)]
	hashFragment := hash[0:4] // 2900 images of same name gives 0.1% chance of collision
	return fmt.Sprintf("%s-%x", fileName, hashFragment)
}

func buildImage(conf *config, path string, slug string) (built builtImage, err error) {
	log.Printf("Building image %s from %s", slug, path)
	return
}

func buildNewManifest(conf *config, foundImages []foundImage, oldManifest map[string]builtImage) (newManifest map[string]builtImage, pathToSlug map[string]string, err error) {
	newManifest = map[string]builtImage{}
	pathToSlug = map[string]string{}
	for _, img := range foundImages {
		slug := getSlug(img.Path, img.Hash)
		if built, ok := oldManifest[slug]; ok {
			newManifest[slug] = built
		} else {
			var built builtImage
			built, err = buildImage(conf, img.Path, slug)
			if err != nil {
				return
			}
			newManifest[slug] = built
		}
		pathToSlug[img.Path] = slug
	}
	return
}

func buildFastSite(basePath string) (err error) {
	conf, err := readConfig(basePath)
	if err != nil {
		return
	}

	conf.basePath = basePath
	fmt.Printf("%+v\n", conf)

	foundImages, err := discoverImages(path.Join(conf.basePath, conf.InputFolder))
	if err != nil {
		return
	}

	oldManifest, err := readManifest(path.Join(conf.basePath, conf.ManifestFile))
	if err != nil {
		return
	}

	newManifest, pathToSlug, err := buildNewManifest(&conf, foundImages, oldManifest)
	if err != nil {
		return
	}

	fmt.Printf("%v\n", newManifest)
	fmt.Printf("%v\n", pathToSlug)
	return
}

func main() {
	basePath := flag.String("basepath", ".", "The folder in which to search for enfasten.yml")
	flag.Parse()
	err := buildFastSite(*basePath)
	if err != nil {
		log.Fatal("FATAL ERROR: ", err)
	}
}

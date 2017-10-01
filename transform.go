package main

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
)

// Yes I'm using a regex to parse HTML, yes everyone tells you not to do that.
// This is for personal sites so if this doesn't match your HTML, fix your HTML.
// This saves me having to do a bunch of tree traversal and serializing.
var imgRegex = regexp.MustCompile(`<img ([^>]*)src="([^"]+)"([^>]*)>`)

type transformConfig struct {
	*config
	manifest   map[string]builtImage
	pathToSlug map[string]string
}

func translatePath(conf *config, file string) string {
	inputPath := path.Join(conf.basePath, conf.InputFolder)
	relPath, err := filepath.Rel(inputPath, file)
	if err != nil {
		log.Fatalf("Can't make relative path %v", err)
	}
	return path.Join(conf.basePath, conf.OutputFolder, relPath)
}

func translateHtml(inPath string, outPath string) (err error) {
	log.Printf("Translating %s", inPath)
	bytes, err := readFileBytes(inPath)

	newBytes := imgRegex.ReplaceAllFunc(bytes, func(match []byte) []byte {
		log.Printf("Image: %s", match)
		return match
	})

	df, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer df.Close()
	df.Write(newBytes)

	return
}

func transferAndTransform(conf *transformConfig, whitelist *[]string, file string) (err error) {
	outPath := translatePath(conf.config, file)
	// log.Printf("Walked %s,      translate to %s", file, outPath)
	err = os.MkdirAll(path.Dir(outPath), os.ModePerm)
	extension := path.Ext(file)

	*whitelist = append(*whitelist, outPath)
	switch extension {
	case ".html":
		err = translateHtml(file, outPath)
	default:
		err = copyFile(file, outPath)
	}
	return
}

func transferAndTransformAll(conf *transformConfig) (whitelist []string, err error) {
	whitelist = []string{}
	walkFunk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return transferAndTransform(conf, &whitelist, path)
	}
	inputPath := path.Join(conf.basePath, conf.InputFolder)
	err = filepath.Walk(inputPath, filepath.WalkFunc(walkFunk))
	return
}

func deleteNonWhitelist(conf *config, whitelist []string) (err error) {
	outputPath := path.Join(conf.basePath, conf.OutputFolder)

	whiteMap := map[string]bool{}
	for _, item := range whitelist {
		whiteMap[item] = true

		// TODO: this is wasteful as heck
		trimmedPath := item
		for {
			trimmedPath = path.Dir(trimmedPath)
			whiteMap[trimmedPath] = true
			if trimmedPath == outputPath {
				break
			}
		}
	}

	toRemove := []string{}

	walkFunk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if _, ok := whiteMap[path]; !ok {
			toRemove = append(toRemove, path)
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	}
	err = filepath.Walk(outputPath, filepath.WalkFunc(walkFunk))

	log.Printf("Delete: %v\n", toRemove)

	for _, item := range toRemove {
		err = os.RemoveAll(item)
		if err != nil {
			return
		}
	}

	return
}

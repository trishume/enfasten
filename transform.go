package main

import (
	"bytes"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
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

func findImagePath(conf *config, fileDir string, relRef string) string {
	if relRef[0] == '/' {
		return relRef[1:len(relRef)]
	} else {
		return path.Join(fileDir, relRef)
	}
}

func nameToImagePath(conf *config, name string) string {
	return path.Join("/", conf.ImageFolder, name)
}

func rebuildImage(conf *transformConfig, relPath string, captures [][]byte) []byte {
	keyPath := findImagePath(conf.config, relPath, string(captures[2]))
	slug, ok := conf.pathToSlug[keyPath]
	if !ok {
		return captures[0]
	}
	built := conf.manifest[slug]

	var buf bytes.Buffer

	buf.WriteString("<img ")
	buf.Write(captures[1])
	buf.WriteString(`src="`)
	buf.WriteString(nameToImagePath(conf.config, built.OriginalName))
	buf.WriteString(`"`)

	// if there's only one image no point in it being responsive
	if len(built.Files) > 1 {
		buf.WriteString(` srcset="`)
		for i, builtFile := range built.Files {
			if i != 0 {
				buf.WriteString(`, `)
			}
			buf.WriteString(nameToImagePath(conf.config, builtFile.FileName))
			buf.WriteString(` `)
			buf.WriteString(strconv.Itoa(builtFile.Width))
			buf.WriteString(`w`)
		}
		if conf.SizesAttr != "" {
			buf.WriteString(`" sizes="`)
			buf.WriteString(conf.SizesAttr)
			buf.WriteString(`"`)
		}
	}

	buf.Write(captures[3])
	buf.WriteString(`>`)

	return buf.Bytes()
}

func translateHtml(conf *transformConfig, inPath string, outPath string) (err error) {
	log.Printf("Translating %s", inPath)
	bytes, err := readFileBytes(inPath)

	// set up for HTML relative paths
	inputPath := path.Join(conf.basePath, conf.InputFolder)
	dirPath := path.Dir(inPath)
	relPath, err := filepath.Rel(inputPath, dirPath)
	if err != nil {
		log.Fatalf("Can't make relative path %v", err)
	}
	log.Printf("Relative path: %s", relPath)

	newBytes := imgRegex.ReplaceAllFunc(bytes, func(match []byte) []byte {
		captures := imgRegex.FindSubmatch(match)
		log.Printf("Old Image: %s", match)
		rebuilt := rebuildImage(conf, relPath, captures)
		log.Printf("New Image: %s", rebuilt)
		return rebuilt
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
		err = translateHtml(conf, file, outPath)
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

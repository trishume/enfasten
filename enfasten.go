package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/bamiaux/rez"
	"github.com/bmatcuk/doublestar"
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

func (conf *config) ImageFolderPath() string {
	return path.Join(conf.basePath, conf.OutputFolder, conf.ImageFolder)
}

func copyFile(source string, dest string) error {
	sf, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer df.Close()
	_, err = io.Copy(df, sf)
	return err
}

func readFileBytes(path string) (bytes []byte, err error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()

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

func saveManifest(manifestPath string, manifest map[string]builtImage) (err error) {
	out, err := yaml.Marshal(manifest)
	if err != nil {
		return
	}

	df, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	defer df.Close()
	_, err = df.Write(out)

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

	for _, path := range matches {
		var hash []byte
		hash, err = hashFile(path)
		if err != nil {
			return
		}
		results = append(results, foundImage{path, hash})
	}
	return
}

func getSlug(imagePath string, hash []byte) string {
	_, baseName := path.Split(imagePath)
	extension := path.Ext(baseName)
	fileName := baseName[0 : len(baseName)-len(extension)]
	hashFragment := hash[0:4] // 2900 images of same name gives 0.1% chance of collision
	return fmt.Sprintf("%s-%x", fileName, hashFragment)
}

func downscaleImage(width int, height int, inputImage image.Image) (downscaled image.Image, err error) {
	// allocate correct buffer type
	r := image.Rect(0, 0, width, height)
	switch t := inputImage.(type) {
	case *image.YCbCr:
		downscaled = image.NewYCbCr(r, t.SubsampleRatio)
	case *image.RGBA:
		downscaled = image.NewRGBA(r)
	case *image.NRGBA:
		downscaled = image.NewNRGBA(r)
	case *image.Gray:
		downscaled = image.NewGray(r)
	default:
		err = fmt.Errorf("Unsupported image colour format %T.", inputImage)
	}

	err = rez.Convert(downscaled, inputImage, rez.NewLanczosFilter(3))
	return
}

func saveImage(outPath string, extension string, img image.Image) (err error) {
	log.Printf("Saving %s with ext %s", outPath, extension)
	// encode the output
	df, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer df.Close()

	switch extension {
	case ".png":
		err = png.Encode(df, img)
	case ".jpg":
		err = jpeg.Encode(df, img, nil)
	default:
		err = fmt.Errorf("Unrecognized extension %s", extension)
	}

	return
}

func buildImage(conf *config, imagePath string, slug string) (built builtImage, err error) {
	log.Printf("Building image %s from %s", slug, imagePath)
	extension := path.Ext(imagePath)

	// load image
	f, err := os.OpenFile(imagePath, os.O_RDONLY, 0)
	if err != nil {
		f.Close()
		return
	}

	var inputImage image.Image
	switch extension {
	case ".png":
		inputImage, err = png.Decode(f)
	case ".jpg":
		inputImage, err = jpeg.Decode(f)
	default:
		err = fmt.Errorf("Unrecognized extension %s", extension)
		return
	}

	f.Close()
	if err != nil {
		return
	}
	built.Width = inputImage.Bounds().Dx()
	built.Height = inputImage.Bounds().Dy()

	// copy-paste original file
	imageFolder := conf.ImageFolderPath()
	originalName := fmt.Sprintf("%s-original%s", slug, extension)
	originalPath := path.Join(imageFolder, originalName)

	if _, err = os.Stat(originalPath); !os.IsNotExist(err) {
		err = copyFile(imagePath, originalPath)
		if err != nil {
			return
		}
	} else {
		log.Printf("Original already copied, skipping: %s", originalPath)
	}

	builtOriginal := builtImageFile{FileName: originalName, Width: built.Width, Height: built.Height}
	built.Files = append(built.Files, builtOriginal)

	// resize to relevant sizes
	for _, w := range conf.Widths {
		if w >= built.Width {
			continue // we never want to upscale
		}

		downscaleRatio := float64(w) / float64(built.Width)
		destHeight := int(float64(built.Height) * downscaleRatio)

		log.Printf("Downscaling %s from %v to (%d,%d)", slug, inputImage.Bounds(), w, destHeight)

		outName := fmt.Sprintf("%s-%dpx%s", slug, w, extension)
		outPath := path.Join(imageFolder, outName)

		if _, err := os.Stat(outPath); !os.IsNotExist(err) {
			log.Printf("Image already exists, skipping: %s", outPath)
			continue // already exists
		}

		// do the scaling
		var downscaledImage image.Image
		downscaledImage, err = downscaleImage(w, destHeight, inputImage)
		if err != nil {
			return
		}

		err = saveImage(outPath, extension, downscaledImage)
		if err != nil {
			return
		}

		builtScaled := builtImageFile{FileName: outName, Width: w, Height: destHeight}
		built.Files = append(built.Files, builtScaled)
	}

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

	foundImages, err := discoverImages(path.Join(conf.basePath, conf.InputFolder))
	if err != nil {
		return
	}

	manifestPath := path.Join(conf.basePath, conf.ManifestFile)
	oldManifest, err := readManifest(manifestPath)
	if err != nil {
		return
	}

	err = os.MkdirAll(conf.ImageFolderPath(), os.ModePerm)
	if err != nil {
		return
	}

	newManifest, _, err := buildNewManifest(&conf, foundImages, oldManifest)
	if err != nil {
		return
	}

	saveManifest(manifestPath, newManifest)

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

package quiptool

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

var attachmentFilenameRE = regexp.MustCompile(`filename="(.+)"`)
var reImage = regexp.MustCompile(`\[Image: ([^\]]+)\]`)
var REVideo = regexp.MustCompile(`\{video: ([^\}]+)\}`)

var quipCookie string

func init() {
	quipCookie = os.Getenv("QUIP_COOKIE")
	if quipCookie == "" {
		log.Fatalln("Please set the environmental variable $QUIP_COOKIE")
	}
}

// export QUIP_COOKIE="..."

type QuipMarkdown struct {
	filename string
	content  string
}

func OpenMarkdown(filename string) (*QuipMarkdown, error) {
	input, err := os.Open(filename)
	defer input.Close()
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(input)
	if err != nil {
		return nil, err
	}

	return &QuipMarkdown{filename: filename, content: string(bytes)}, nil
}

func (q *QuipMarkdown) ImageUrls() []string {
	matches := reImage.FindAllStringSubmatch(q.content, -1)
	ms := make([]string, 0, len(matches))
	for _, match := range matches {
		ms = append(ms, match[1])
	}

	return ms
}

func (q *QuipMarkdown) DownloadAssets() (*QuipAssets, error) {
	baseDir := path.Dir(q.filename)
	assets := NewAssets(baseDir, quipCookie)
	err := assets.SaveAll(q.ImageUrls())
	return assets, err
}

func (q *QuipMarkdown) NormalizedContent() (string, error) {
	assets, err := q.DownloadAssets()
	if err != nil {
		return "", err
	}

	content := q.content
	content = reImage.ReplaceAllStringFunc(content, func(imageTag string) string {
		url := reImage.FindStringSubmatch(imageTag)[1]
		filename := assets.GetFilename(url)
		return fmt.Sprintf("\n![](%v)\n\n", filename)
	})

	content = REVideo.ReplaceAllStringFunc(content, func(videoTag string) string {
		url := REVideo.FindStringSubmatch(videoTag)[1]
		return fmt.Sprintf("<video src='%v' controls='true'></video>", url)
	})

	return content, nil
}

func parseCookies(rawCookies string) []*http.Cookie {
	h := make(http.Header, 1)
	h["Cookie"] = []string{rawCookies}
	req := &http.Request{Header: h}
	return req.Cookies()
}

type QuipAssets struct {
	baseDir   string
	cookie    string
	urlToFile map[string]string
	fileToURL map[string]string
}

func NewAssets(baseDir, cookie string) *QuipAssets {
	assets := &QuipAssets{
		baseDir:   baseDir,
		cookie:    cookie,
		urlToFile: make(map[string]string),
		fileToURL: make(map[string]string),
	}

	// try to load assets metadata
	err := assets.Load()
	if err != nil && !os.IsNotExist(err) {
		log.Println(err)
	}

	return assets
}

func (q *QuipAssets) SaveAll(urls []string) error {
	for _, url := range urls {
		err := q.Download(url)
		if err != nil {
			log.Println(err)
		}
	}

	return q.Save()
}

func (q *QuipAssets) Download(url string) error {
	if _, ok := q.urlToFile[url]; ok {
		log.Println("blob downloaded:", url)
		return nil
	}

	return q.Get(url)
}
func (q *QuipAssets) Get(url string) error {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header["Cookie"] = []string{q.cookie}

	client := http.Client{}
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		return err
	}

	disposition := res.Header["Content-Disposition"][0]
	originalFilename := attachmentFilenameRE.FindStringSubmatch(disposition)[1]
	filename := q.ensureUniqueFilename(originalFilename)

	filePath := path.Join(q.baseDir, filename)
	// if _, err := os.Stat(filePath)

	log.Println("saving:", url, filePath)

	output, err := os.Create(filePath) // os.OpenFile(filePath, os.O_CREATE, 0644)
	defer output.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(output, res.Body)
	if err != nil {
		return err
	}

	q.fileToURL[filename] = url
	q.urlToFile[url] = filename

	return nil
}

func (q *QuipAssets) Load() error {
	f, err := os.Open(q.assetsMetaFilename())
	defer f.Close()
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&q.urlToFile)
	if err != nil {
		return err
	}

	for url, filename := range q.urlToFile {
		q.fileToURL[filename] = url
	}

	return nil
}

func (q *QuipAssets) assetsMetaFilename() string {
	return path.Join(q.baseDir, "quip-assets.json")
}

func (q *QuipAssets) Save() error {
	f, err := os.Create(q.assetsMetaFilename())
	defer f.Close()
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	err = encoder.Encode(q.urlToFile)
	return err
}

func (q *QuipAssets) GetFilename(url string) string {
	return q.urlToFile[url]
}

func (q *QuipAssets) ensureUniqueFilename(originalFilename string) string {
	var i = 1
	var filename string
	ext := path.Ext(originalFilename)
	name := strings.TrimSuffix(originalFilename, ext)
	for {
		var newFilename string
		if i == 1 {
			newFilename = originalFilename
		} else {
			newFilename = fmt.Sprintf("%v_%v%v", name, i, ext)
		}

		if _, ok := q.fileToURL[newFilename]; ok {
			i += 1
		} else {
			filename = newFilename
			break
		}
	}

	return filename
}

func replaceQuipImage(image string) string {
	return "\n![](foo)\n\n"
}

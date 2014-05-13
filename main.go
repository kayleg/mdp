package main

import (
	"flag"
	"fmt"
	"github.com/BakedSoftware/blackfriday"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const template = `
<doctype html>
<html>
<head>
<title>%s</title>
<style>%s</style>
<script type='text/javascript' src='https://c328740.ssl.cf1.rackcdn.com/mathjax/latest/MathJax.js?config=TeX-AMS-MML_HTMLorMML'></script>
</head>
<body>%s
</body>
</html>
`

var cssPath = flag.String("css", "markdown.css", "path to css file to use")

func Exists(name string) (bool, error) {
	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func GetHtmlData(page string) ([]byte, error) {
	md, err := ioutil.ReadFile(page + ".mdown")
	if err != nil {
		return nil, err
	}
	var stylePath string
	if ok, _ := Exists(*cssPath); ok {
		stylePath = *cssPath
	} else {
		stylePath = os.ExpandEnv("${HOME}/bin/markdown.css")
	}
	styles, _ := ioutil.ReadFile(stylePath)
	html := blackfriday.MarkdownCommon(md)
	final := fmt.Sprintf(template, page, string(styles), string(html))
	return []byte(final), nil
}

func MakePreview(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	page := vars["page"]
	rw.Header().Set("Content-Type", "text/html")
	html, _ := GetHtmlData(page)
	rw.Write(html)
}

var imgSrcRegex = regexp.MustCompile("img(.*?)src=\"(.+?)\"")
var protocolRegex = regexp.MustCompile("(?:http[s]?|file)://")

func MakePdf(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	page := vars["page"]
	content, _ := GetHtmlData(page)
	cwd, _ := os.Getwd()
	contentStr := imgSrcRegex.ReplaceAllStringFunc(string(content), func(match string) string {
		if protocolRegex.MatchString(match) {
			return match
		}
		parts := imgSrcRegex.FindStringSubmatch(match)
		return fmt.Sprintf("img %s src=\"%s/%s\"", parts[1], cwd, parts[2])
	})
	var fname string
	tempfile, _ := ioutil.TempFile("", "mdp")
	fname = tempfile.Name()
	defer func() {
		tempfile.Close()
		os.Remove(fname + ".html")
		os.Remove(fname + ".pdf")
	}()
	tempfile.Write([]byte(contentStr))
	err := exec.Command("mv", fname, fname+".html").Run()
	if err != nil {
		log.Fatalln(err)
	}
	out, err := exec.Command("wkhtmltopdf", "--print-media-type", fname+
		".html", fname+".pdf").CombinedOutput()
	log.Println(string(out))
	if err != nil {
		log.Fatal(err)
	}

	pdf, _ := ioutil.ReadFile(tempfile.Name() + ".pdf")
	rw.Header().Set("Content-Type", "application/pdf")
	rw.Write(pdf)

}

var mdRegex = regexp.MustCompile("(.*?).mdown")

func Index(rw http.ResponseWriter, req *http.Request) {
	files, _ := ioutil.ReadDir("./")
	rw.Header().Set("Content-Type", "text/html")
	rw.Write([]byte("<table>"))
	count := 1
	for _, f := range files {
		name := f.Name()
		if name[0] == '.' || !mdRegex.MatchString(name) {
			continue
		}
		comps := strings.Split(name, ".")
		name = strings.Join(comps[:len(comps)-1], ".")
		rw.Write([]byte(fmt.Sprintf("<tr><td>%d</td><td>%s</td><td><a href='%s.html'>HTML</a></td><td><a href='%s.pdf'>PDF</a></td></tr>", count, name, name, name)))
		count++
	}
	rw.Write([]byte("</table>"))
}

func main() {
	flag.Parse()
	r := mux.NewRouter()
	r.HandleFunc("/", Index)
	r.HandleFunc("/{page}.html", MakePreview)
	r.HandleFunc("/{page}.pdf", MakePdf)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./")))
	log.Fatal(http.ListenAndServe(":5000", r))
}

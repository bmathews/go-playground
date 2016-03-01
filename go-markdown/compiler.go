package main

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"time"

	"github.com/russross/blackfriday"
	"gopkg.in/yaml.v2"
)

// FileData ... a struct that represents the data in the file
type FileData struct {
	Title    string
	Date     time.Time
	Template string
	Image    string
	Content  template.HTML
}

// Read a file
func readFile(path string) []byte {
	b, err := ioutil.ReadFile(path)
	perror(err)
	return b
}

// Pull the metadata byes out of a file
func getRawMetadata(fileContents []byte) []byte {
	endOfYaml := bytes.Index(fileContents, []byte("---"))
	return fileContents[:endOfYaml]
}

// Pull the markdown byteslice out of a file
func getRawMarkdown(fileContents []byte) []byte {
	endOfYaml := bytes.Index(fileContents, []byte("---")) + 4
	return fileContents[endOfYaml:]
}

// Convert a yaml string to the FileData struct
func convertYaml(rawYaml []byte) FileData {
	fileData := FileData{}
	err := yaml.Unmarshal(rawYaml, &fileData)
	perror(err)
	return fileData
}

// Convert a string of markdown to an html byteslice
func convertMarkdown(data []byte) template.HTML {
	return template.HTML(string(blackfriday.MarkdownBasic(data)))
}

// Compile a template based on the
func compileTemplate(file FileData) []byte {
	rawTemplate := readFile(file.Template)
	tmpl, err := template.New("tpl").Parse(string(rawTemplate))
	perror(err)
	var buff bytes.Buffer
	tmpl.Execute(&buff, file)
	return buff.Bytes()
}

// Compile a file
func Compile(path string) []byte {
	fileContents := readFile(path)
	rawYaml := getRawMetadata(fileContents)
	rawMarkdown := getRawMarkdown(fileContents)

	data := convertYaml(rawYaml)
	data.Content = convertMarkdown(rawMarkdown)

	return compileTemplate(data)
}

// Error helper util
func perror(err error) {
	if err != nil {
		panic(err)
	}
}

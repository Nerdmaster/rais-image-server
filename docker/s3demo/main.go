// Package main, along with the various *.go.html files, demonstrates a very
// simple (and ugly) asset server that reads all S3 assets in a given region
// and bucket, and serves up HTML pages which point to a IIIF server (RAIS, of
// course) for thumbnails and full-image views.
package main

import (
	"fmt"
	"html/template"
	"log"
	"net/url"
	"os"
	"strings"
)

type asset struct {
	Key    string
	IIIFID string
	Title  string
}

func (a asset) Thumb(width int) template.Srcset {
	return template.Srcset(fmt.Sprintf("/iiif/%s/full/%d,/0/default.jpg", a.IIIFID, width))
}

var emptyAsset asset

var s3assets []asset
var indexT, assetT *template.Template
var zone, bucket string
var keyID, secretKey string

func env(key string) string {
	for _, kvpair := range os.Environ() {
		var parts = strings.SplitN(kvpair, "=", 2)
		if parts[0] == key {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func main() {
	zone = env("RAIS_S3ZONE")
	bucket = env("RAIS_S3BUCKET")
	keyID = env("AWS_ACCESS_KEY_ID")
	secretKey = env("AWS_SECRET_ACCESS_KEY")

	if zone == "" || bucket == "" || keyID == "" || secretKey == "" {
		fmt.Println("You must set env vars RAIS_S3BUCKET, RAIS_S3ZONE, AWS_ACCESS_KEY_ID, and")
		fmt.Println("AWS_SECRET_ACCESS_KEY before running the demo.  You can export these directly")
		fmt.Println(`or use a the docker-compose ".env" file.`)
		os.Exit(1)
	}

	readAssets()
	preptemplates()
	serve()
}

func readAssets() {
	var contents = []string{
		"testjp2s/0-Almeida_Junior_.png.jp2",
		"testjp2s/1-Amedeo_Modigliani_-_.png.jp2",
		"testjp2s/2-Auguste_Renoir_-_Dan.png.jp2",
		"testjp2s/3-Bernat_Martorell_-_A.png.jp2",
		"testjp2s/4-James_McNeill_Whistl.png.jp2",
		"testjp2s/5-Giovanni_Bellini_-_S.png.jp2",
	}
	for _, key := range contents {
		var id = "s3:" + url.PathEscape(key)
		s3assets = append(s3assets, asset{Title: key, Key: key, IIIFID: id})
	}
	log.Printf("Indexed %d assets", len(s3assets))
}

func preptemplates() {
	var _, err = os.Stat("./layout.go.html")
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Unable to load HTML layout: file does not exist.  Make sure you run the demo from the docker/s3demo folder.")
		} else {
			log.Printf("Error trying to open layout: %s", err)
		}
		os.Exit(1)
	}

	var root = template.New("layout")
	var layout = template.Must(root.ParseFiles("layout.go.html"))
	indexT = template.Must(template.Must(layout.Clone()).ParseFiles("index.go.html"))
	assetT = template.Must(template.Must(layout.Clone()).ParseFiles("asset.go.html"))
}

type Data struct {
	Zone      string
	Bucket    string
	KeyID     string
	SecretKey string
}

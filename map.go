package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/gorm"
)

type mapData struct {
	gorm.Model
	MapID       int
	Name        string
	Author      string
	Size        string
	Category    string
	UploadDate  string
	Rating      string
	Downloads   int
	DownloadURL string
	Retrieved   bool
}

func (md *mapData) index() error {
	// index the html for the given map id
	mapURL := fmt.Sprintf("%v/maps/%v/", domain, md.MapID)
	resp, err := http.Get(mapURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot open the web page: status code %v", resp.StatusCode)
	}

	ctype := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ctype, "text/html") {
		return fmt.Errorf("invalid web page: received %v", ctype)
	}

	// parse the html element and read the data
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("invalid web page: cannot load into goquery: %v", err)
	}

	var unparsedData []string

	doc.Find(".listentry").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		link, _ := s.Find("a").Attr("href")

		link = strings.TrimSpace(link)
		text = strings.TrimSpace(text)

		for _, v := range strings.Split(text, "\n") {
			data := strings.TrimRight(v, "\r")
			data = strings.Trim(data, " ")
			if data != "" {
				unparsedData = append(unparsedData, data)
			}

		}

		for _, v := range strings.Split(link, "\n") {
			data := strings.TrimRight(v, "\r")
			data = strings.Trim(data, " ")
			if data != "" {
				unparsedData = append(unparsedData, data)
			}
		}
	})

	// test if the map is missing from the server
	if len(unparsedData) == 0 {
		return fmt.Errorf("map missing on the server")
	} else if strings.Contains(unparsedData[0], "/maps/") {
		return fmt.Errorf("map missing on the server")
	}

	md.Name = unparsedData[0]
	for _, v := range unparsedData {
		if strings.Contains(v, "by ") {
			md.Author = strings.Replace(v, "by ", "", 1)
			md.Author = strings.Replace(md.Author, "<br />", "", 1)
		} else if strings.Contains(v, "Size: ") {
			md.Size = strings.Replace(v, "Size: ", "", 1)
		} else if strings.Contains(v, "Category: ") {
			md.Category = strings.Replace(v, "Category: ", "", 1)
		} else if strings.Contains(v, "Submitted: ") {
			md.UploadDate = strings.Replace(v, "Submitted: ", "", 1)
		} else if strings.Contains(v, "Rating: ") {
			md.Rating = strings.Replace(v, "Rating: ", "", 1)
		} else if strings.Contains(v, "Downloads: ") {
			downloads := strings.Replace(v, "Downloads: ", "", 1)
			md.Downloads, err = strconv.Atoi(downloads)
			if err != nil {
				return fmt.Errorf("cannot parse downloads into int: %v", err)
			}
		} else if strings.Contains(v, "/maps/download/") {
			md.DownloadURL = domain + v
		}

	}
	md.Retrieved = false

	return nil
}
func (md *mapData) download(downloadDir string) error {
	// download the map
	resp, err := http.Get(md.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	fullPath := fmt.Sprintf("%v/%v", downloadDir, md.MapID)
	out, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the map data to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func createDownloadDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		return err
	}

	return nil
}

func readMapFromDB(mapID int, mi *mapIndex) (mapData, error) {
	md := mapData{MapID: mapID}

	path := fmt.Sprintf("./%v.db", mi.Name)
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return md, err
	}
	defer db.Close()

	// read data about the given map id
	db.First(&md, "map_id = ?", md.MapID)

	return md, nil
}

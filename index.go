package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

type mapIndex struct {
	Name string
}

func (mi *mapIndex) init() error {
	path := fmt.Sprintf("./%v.db", mi.Name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_, err := os.Create(path)
		if err != nil {
			return err
		}

		dbFile, err := sql.Open("sqlite3", path)
		if err != nil {
			return err
		}
		dbFile.Close()
	}

	// populate the database with schema
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()

	// Migrate the schema
	db.AutoMigrate(&mapData{})

	// write the initial data if there's none in the db
	initialMapData := mapData{
		MapID:       0,
		Name:        "Skrappy Test Map",
		Author:      "Skrappy Tool",
		Size:        "0 MB",
		Category:    "None",
		UploadDate:  "20 Nov 2019 21:13",
		Rating:      "0 Good 0 Bad",
		Downloads:   0,
		DownloadURL: "http://127.0.0.1/example",
		Retrieved:   true,
	}
	if db.First(&initialMapData, "map_id = 0").RecordNotFound() {
		db.Create(&initialMapData)
	}

	// check if the data was written correctly
	if db.First(&initialMapData, "map_id = 0").RecordNotFound() {
		return fmt.Errorf("initial map data not written")
	}

	return nil
}

func (mi *mapIndex) addMap(md *mapData) error {
	path := fmt.Sprintf("./%v.db", mi.Name)
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()

	db.Create(md)
	return nil

}

func (mi *mapIndex) markMapAsRetrieved(md *mapData) error {
	path := fmt.Sprintf("./%v.db", mi.Name)
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()

	db.Model(&md).Update("Retrieved", true)
	return nil
}

func (mi *mapIndex) mapIndexed(md mapData) bool {
	path := fmt.Sprintf("./%v.db", mi.Name)
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return false
	}
	defer db.Close()

	// check if the map is already indexed
	if db.First(&md, "map_id = ?", md.MapID).RecordNotFound() {
		return false
	}

	return true
}

func (mi *mapIndex) mapDownloaded(md mapData) bool {
	path := fmt.Sprintf("./%v.db", mi.Name)
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return false
	}
	defer db.Close()

	// if the given map id doesn't exist, probably missing on the server
	// skip it by telling the caller it's downloaded
	if db.First(&md, "map_id = ?", md.MapID).RecordNotFound() {
		return true
	}

	// given map wasn't downloaded
	if db.First(&md, "map_id = ? AND retrieved = 1", md.MapID).RecordNotFound() {
		return false
	}

	// given map was downloaded
	return true
}

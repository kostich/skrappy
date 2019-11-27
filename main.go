package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/cheggaaa/pb/v3"
)

var domain string
var indexName string = "maps"
var downloadDir string = "./maps"
var maxMapID int
var maxParallelConns int
var currentConns int = 0
var programStopWait = 10

func main() {
	// start the operation log
	f, err := os.OpenFile("skrappy.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	// get the env vars
	domain = os.Getenv("DOMAIN")
	maxMapID, err := strconv.Atoi(os.Getenv("MAXMAP"))
	if err != nil {
		usage()
	}
	maxParallelConns, err := strconv.Atoi(os.Getenv("MAXCONN"))
	if err != nil {
		usage()
	}
	stage := os.Getenv("STAGE")

	if domain == "" {
		usage()
	} else if maxMapID == 0 {
		usage()
	} else if maxParallelConns == 0 {
		usage()
	} else if stage != "index" && stage != "download" && stage != "both" && stage != "update" {
		usage()
	}

	// init the index
	fmt.Println("Initializing the index.")
	log.Println("Initializing the index.")
	mi := mapIndex{Name: indexName}
	err = mi.init()
	if err != nil {
		log.Printf(fmt.Sprintf("cannot init index: %v", err))
		os.Exit(1)
	}

	if stage == "index" || stage == "both" {
		// build the map index
		fmt.Println("Building the map index.")
		log.Println("Building the map index.")

		// progress bar
		bar := pb.Full.New(maxMapID)
		bar.SetRefreshRate(100 * time.Millisecond)
		bar.SetWriter(os.Stderr)
		bar.Start()

		for currentMapID := 1; currentMapID <= maxMapID; currentMapID++ {
			// we start a lot of async indexing requests
			// but we don't go over maxParallelConns so we
			// don't overload the server(s)
			if currentConns <= maxParallelConns {
				// check if we already indexed the map data to db
				md := mapData{MapID: currentMapID}
				if !mi.mapIndexed(md) {
					go asyncMapIndex(&md, &mi, bar)
				} else {
					bar.Increment()
				}
			} else {
				// wait a bit as we're over the max allowed connections so we
				currentMapID--
				rand.Seed(time.Now().UnixNano())
				waitTime := time.Duration(rand.Intn(500)) * time.Millisecond
				time.Sleep(waitTime)
			}
		}

		// wait a bit for all the writes and indexing requests and exit
		for currentConns > 0 {
			time.Sleep(1 * time.Second)
		}
		bar.Finish()
		log.Printf("Waiting %v seconds to complete the DB writes.\n", programStopWait)
		time.Sleep(time.Duration(programStopWait) * time.Second)
	}

	if stage == "download" || stage == "both" {
		fmt.Println("Downloading the indexed maps.")
		log.Println("Downloading the indexed maps.")
		err = createDownloadDir(downloadDir)
		if err != nil {
			log.Printf("cannot create download dir: %v\n", err)
		}

		// progress bar
		bar := pb.Full.New(maxMapID)
		bar.SetRefreshRate(100 * time.Millisecond)
		bar.SetWriter(os.Stderr)
		bar.Start()

		for currentMapID := 1; currentMapID <= maxMapID; currentMapID++ {
			// we start a lot of async download requests
			// but we don't go over maxParallelConns so we
			// don't overload the server(s)
			if currentConns <= maxParallelConns {
				// check if we already downloaded the map data to db
				md, err := readMapFromDB(currentMapID, &mi)
				if err != nil {
					fmt.Printf("cannot read map data: %v\n", err)
					log.Printf("cannot read map data: %v\n", err)
					os.Exit(1)
				}

				if !mi.mapDownloaded(md) {
					go asyncMapDownload(&md, &mi, bar, downloadDir)
				} else {
					bar.Increment()
				}
			} else {
				// wait a bit as we're over the max allowed connections so we
				currentMapID--
				rand.Seed(time.Now().UnixNano())
				waitTime := time.Duration(rand.Intn(500)) * time.Millisecond
				time.Sleep(waitTime)
			}
		}

		// wait until we download all maps and then finish
		for currentConns > 0 {
			time.Sleep(1 * time.Second)
		}
		log.Printf("Waiting %v seconds to complete the file writes.\n", programStopWait)
		time.Sleep(time.Duration(programStopWait) * time.Second)
		bar.Finish()
	}

	if stage == "update" {
		// mostly the same as the download stage except we go from the
		// newest mapID to the oldest

		// update the map index
		fmt.Println("Updating the existing map index.")
		log.Println("Updating the existing map index.")

		// progress bar
		bar := pb.Full.New(maxMapID)
		bar.SetRefreshRate(100 * time.Millisecond)
		bar.SetWriter(os.Stderr)
		bar.Start()

		for currentMapID := maxMapID; currentMapID > 0; currentMapID-- {
			// we start a lot of async indexing requests
			// but we don't go over maxParallelConns so we
			// don't overload the server(s)
			if currentConns <= maxParallelConns {
				// check if we already indexed the map data to db
				md := mapData{MapID: currentMapID}
				if !mi.mapIndexed(md) {
					go asyncMapIndex(&md, &mi, bar)
				} else {
					bar.Increment()
				}
			} else {
				// wait a bit as we're over the max allowed connections so we
				currentMapID++
				rand.Seed(time.Now().UnixNano())
				waitTime := time.Duration(rand.Intn(500)) * time.Millisecond
				time.Sleep(waitTime)
			}
		}

		// wait a bit for all the writes and indexing requests and exit
		for currentConns > 0 {
			time.Sleep(1 * time.Second)
		}
		bar.Finish()
		log.Printf("Waiting %v seconds to complete the DB writes.\n", programStopWait)
		time.Sleep(time.Duration(programStopWait) * time.Second)
	}

	fmt.Println("All done!")
	os.Exit(0)
}

func usage() {
	fmt.Println("Usage: DOMAIN=\"https://some.domain.tld\" MAXMAP=\"max map id\" MAXCONN=\"maximum no. of parallel download/indexing connections\" STAGE=\"index,download,both\" ./skrappy")
	os.Exit(1)
}

func asyncMapIndex(md *mapData, mi *mapIndex, bar *pb.ProgressBar) {
	// index the map info
	currentConns++
	err := md.index()
	bar.Increment()

	// wait a bit after indexing
	rand.Seed(time.Now().UnixNano())
	waitTime := time.Duration(rand.Intn(100)) * time.Millisecond
	time.Sleep(waitTime)

	// save the indexed map info if we indexed it correctly
	if err != nil {
		log.Printf("cannot index info about map %v: %v\n", md.MapID, err)
	} else {
		err = mi.addMap(md)
		if err != nil {
			log.Println(fmt.Sprintf("cannot add map to index: %v", err))
		}
	}

	currentConns--
}

func asyncMapDownload(md *mapData, mi *mapIndex, bar *pb.ProgressBar, downloadDir string) {
	// download the map
	currentConns++
	err := md.download(downloadDir)
	bar.Increment()

	// wait a bit after the download
	rand.Seed(time.Now().UnixNano())
	waitTime := time.Duration(rand.Intn(25)) * time.Millisecond
	time.Sleep(waitTime)

	// mark the map as retrieved if we downloaded it correctly
	if err != nil {
		log.Printf("cannot download map %v: %v\n", md.MapID, err)
	} else {
		err = mi.markMapAsRetrieved(md)
		if err != nil {
			log.Println(fmt.Sprintf("cannot add map to index: %v", err))
		}
	}

	currentConns--
}

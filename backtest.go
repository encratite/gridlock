package main

import (
	"encoding/csv"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/encratite/commons"
)

const (
	driverMinFileSize = 1024
	winnerPriceLimit = 0.95
)

type raceData struct {
	name string
	drivers []driverData
}

type driverData struct {
	name string
	practicePrice float64
	qualifyingPrice float64
	racePrice float64
	winner bool
}

func runBacktest() {
	loadConfiguration()
	races := []raceData{}
	stopWatch := commons.NewStopWatch()
	for _, raceConfig := range configuration.Races {
		race := loadRace(raceConfig)
		races = append(races, race)
	}
	stopWatch.Stop("Finished loading race data")
}

func loadRace(raceConfig RaceConfiguration) raceData {
	directory := filepath.Join(configuration.Source, raceConfig.Path)
	entries, err := os.ReadDir(directory)
	if err != nil {
		log.Fatalf("Unable to read directory: %s", directory)
	}
	paths := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && filepath.Ext(name) == ".csv" && !strings.Contains(name, "another") {
			path := filepath.Join(directory, name)
			info, err := os.Stat(path)
			if err != nil {
				log.Fatalf("Failed to determine file size: %s", path)
			}
			if info.Size() < driverMinFileSize {
				continue
			}
			paths = append(paths, path)
		}
	}
	drivers := commons.ParallelMap(paths, func (path string) driverData {
		driver := loadDriver(path, raceConfig)
		return driver
	})
	winnerCount := 0
	for _, driver := range drivers {
		if driver.winner {
			winnerCount++
		}
	}
	if winnerCount != 1 {
		log.Fatalf("Invalid number of winners for race %s (%d)", raceConfig.Path, winnerCount)
	}
	data := raceData{
		name: raceConfig.Path,
		drivers: drivers,
	}
	return data
}

func loadDriver(path string, raceConfig RaceConfiguration) driverData {
	fileName := filepath.Base(path)
	pattern := regexp.MustCompile("will-(.+?)-win-")
	matches := pattern.FindStringSubmatch(fileName)
	if matches == nil {
		log.Fatalf("Unable to extract name of driver: %s", fileName)
	}
	name := matches[1]
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Failed to read driver data: %v", err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	_, _ = reader.Read()
	var previousPrice *float64
	var practicePrice *float64
	var qualifyingPrice *float64
	var racePrice *float64
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		timestamp := commons.MustParseTime(record[0])
		price := commons.MustParseFloat(record[1])
		if previousPrice != nil {
			if practicePrice == nil && timestamp.After(raceConfig.Practice.Time) {
				practicePrice = previousPrice
			} else if qualifyingPrice == nil && timestamp.After(raceConfig.Qualifying.Time) {
				qualifyingPrice = previousPrice
			} else if racePrice == nil && timestamp.After(raceConfig.Race.Time) {
				racePrice = previousPrice
			}
		}
		previousPrice = &price
	}
	if practicePrice == nil || qualifyingPrice == nil || racePrice == nil || previousPrice == nil {
		log.Fatalf("Failed to extract prices from %s", path)
	}
	winner := *previousPrice > winnerPriceLimit
	data := driverData{
		name: name,
		practicePrice: *practicePrice,
		qualifyingPrice: *qualifyingPrice,
		racePrice: *racePrice,
		winner: winner,
	}
	return data
}
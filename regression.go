package main

import (
	"fmt"
	"log"
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/encratite/commons"
	"github.com/cdipaolo/goml/linear"
)

const (
	dataDirectory = "data"
	driverLimit = 10
	firstSeason = 2020
	lastSeason = 2025
	lastEventId = 17
	featureRaces = 5
	featurePositionScoreBase = 0.4
)

type raceResult int

const (
	resultPosition raceResult = iota
	resultRetired
	resultDisqualified
	resultOther
)

type wikiDataPath struct {
	season int
	path string
}

type driverSeasonalData struct {
	name string
	races []driverRaceResult
}

type driverRaceResult struct {
	season int
	id int
	result raceResult
	position int
}

func performRegression() {
	paths := downloadFiles()
	drivers := parseFiles(paths)
	features, labels := getFeatures(drivers)
	fitAndEvaluate(features, labels)
}

func downloadFiles() []wikiDataPath {
	commons.CreateDirectory(dataDirectory)
	paths := []wikiDataPath{}
	for year := firstSeason; year <= lastSeason; year++ {
		fileName := fmt.Sprintf("%d.html", year)
		outputPath := filepath.Join(dataDirectory, fileName)
		dataPath := wikiDataPath{
			season: year,
			path: outputPath,
		}
		paths = append(paths, dataPath)
		exists := commons.FileExists(outputPath)
		if !exists {
			url := fmt.Sprintf("https://en.wikipedia.org/wiki/%d_Formula_One_World_Championship", year)
			err := commons.DownloadFile(url, outputPath)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Downloaded %s\n", fileName)
		}
	}
	return paths
}

func parseFiles(paths []wikiDataPath) []driverSeasonalData {
	drivers := []driverSeasonalData{}
	for _, path := range paths {
		seasonDrivers := parseFile(path)
		for _, driver := range seasonDrivers {
			i := slices.IndexFunc(drivers, func (d driverSeasonalData) bool {
				return d.name == driver.name
			})
			if i >= 0 {
				races := &drivers[i].races
				*races = append(*races, driver.races...)
			} else {
				drivers = append(drivers, driver)
			}
		}
	}
	return drivers
}

func parseFile(dataPath wikiDataPath) []driverSeasonalData {
	path := dataPath.path
	fmt.Printf("Processing %s\n", path)
	html := commons.ReadFile(path)
	reader := strings.NewReader(string(html))
	doc, err := htmlquery.Parse(reader)
	if err != nil {
		log.Fatalf("Failed to parse HTML: %v", err)
	}
	table := htmlquery.FindOne(doc, "//table[.//text()[contains(., 'Driver')] and .//text()[contains(., 'BHR')] and not(.//table)]")
	if table == nil {
		log.Fatalf("Failed to locate race table in %s", path)
	}
	rows := htmlquery.Find(table, "/tbody/tr")
	if len(rows) < 20 {
		log.Fatalf("Failed to extract rows from table in %s", path)
	}
	firstRow := rows[0]
	links := htmlquery.Find(firstRow, "/th/a[contains(@title, 'Grand Prix') and not(*) and text()]")
	if len(links) < 10 {
		log.Fatalf("Failed to extract event codes from first row in %s", path)
	}
	eventCodes := []string{}
	for _, link := range links {
		eventCode := htmlquery.InnerText(link)
		// fmt.Printf("\tEvent code: %s\n", eventCode)
		eventCodes = append(eventCodes, eventCode)
	}
	drivers := []driverSeasonalData{}
	for i := range driverLimit {
		row := rows[i + 1]
		nameCell := htmlquery.FindOne(row, "/td[1]")
		if nameCell == nil {
			log.Fatalf("Failed to find name cell %s", path)
		}
		name := htmlquery.InnerText(nameCell)
		name = commons.Trim(name)
		// fmt.Printf("\tDriver: %s\n", name)
		cells := htmlquery.Find(row, "/td[position() > 1 and position() < last()]/text()[1]")
		if len(cells) < 10 {
			log.Fatalf("Failed to find driver cells in %s", path)
		}
		races := []driverRaceResult{}
		for j, cell := range cells {
			id := j + 1
			if dataPath.season == lastSeason && id > lastEventId {
				break
			}
			resultText := htmlquery.InnerText(cell)
			resultText = commons.Trim(resultText)
			resultText = strings.Replace(resultText, "†", "", 1)
			position, err := commons.ParseInt(resultText)
			var driverResult driverRaceResult
			if err == nil {
				driverResult = driverRaceResult{
					season: dataPath.season,
					id: id,
					result: resultPosition, 
					position: position,
				}
			} else {
				var result raceResult
				switch resultText {
				case "Ret":
					result = resultRetired
				case "DSQ":
					result = resultDisqualified
				default:
					result = resultOther
				}
				driverResult = driverRaceResult{
					season: dataPath.season,
					id: id,
					result: result, 
					position: 0,
				}
			}
			races = append(races, driverResult)
			// fmt.Printf("\t\tResult: %s\n", resultText)
		}
		driver := driverSeasonalData{
			name: name,
			races: races,
		}
		drivers = append(drivers, driver)
	}
	return drivers
}

func getFeatures(drivers []driverSeasonalData) ([][]float64, []float64) {
	features := [][]float64{}
	labels := []float64{}
	for _, driver := range drivers {
		for i, race := range driver.races {
			if i < featureRaces {
				continue
			}
			raceFeatures := []float64{}
			multiSeason := false
			j := 1
			for len(raceFeatures) < featureRaces && i >= j {
				previousRace := driver.races[i - j]
				if previousRace.season != race.season {
					multiSeason = true
					break
				}
				if previousRace.result == resultPosition {
					exponent := float64(previousRace.position - 1)
					value := math.Pow(featurePositionScoreBase, exponent)
					raceFeatures = append(raceFeatures, value)
				}
				j++
			}
			if multiSeason {
				continue
			}
			if len(raceFeatures) != featureRaces {
				continue
			}
			var label float64
			if race.result == resultPosition && race.position == 1 {
				label = 1.0
			} else {
				label = 0.0
			}
			labels = append(labels, label)
			features = append(features, raceFeatures)
		}
	}
	return features, labels
}

func fitAndEvaluate(features [][]float64, labels []float64) {
	model := linear.NewLogistic("Batch Gradient Ascent", 0.0001, 0.0, 1000, features, labels)
	err := model.Learn()
	if err != nil {
		log.Fatalf("Failed to train model: %v", err)
	}
	truePositives := 0
	falsePositives := 0
	trueNegatives := 0
	falseNegatives := 0
	positiveLabels := 0
	for i, currentFeatures := range features {
		label := labels[i] == 1.0
		predictionVector, err := model.Predict(currentFeatures)
		if err != nil {
			log.Fatalf("Failed to make predictions: %v", err)
		}
		prediction := predictionVector[0] > 0.5
		if label {
			if prediction {
				truePositives++
			} else {
				falseNegatives++
			}
			positiveLabels++
		} else {
			if prediction {
				falsePositives++
			} else {
				trueNegatives++
			}
		}
	}
	total := len(features)
	printRatio := func (description string, count int) {
		percentage := 100.0 * float64(count) / float64(total)
		fmt.Printf("%s: %.1f%% (%d samples)\n", description, percentage, count)
	}
	f1Score := 2.0 * float64(truePositives) / (2.0 * float64(truePositives) + float64(falsePositives) + float64(falseNegatives))
	printRatio("True positives", truePositives)
	printRatio("True negatives", trueNegatives)
	printRatio("False positives", falsePositives)
	printRatio("False negatives", falseNegatives)
	printRatio("True labels", positiveLabels)
	fmt.Printf("F1 score: %.3f", f1Score)
}
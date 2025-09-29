package main

import (
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/encratite/commons"
	"github.com/sjwhitworth/golearn/base"
	"github.com/sjwhitworth/golearn/linear_models"
	"github.com/sjwhitworth/golearn/evaluation"
)

const (
	dataDirectory = "data"
	driverLimit = 10
	firstSeason = 2020
	lastSeason = 2025
	lastEventId = 17
	featureRaces = 3
)

var featurePositionScores = []int{
	25,
	18,
	15,
	12,
	10,
	8,
	6,
	4,
	2,
	1,
}

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
	features := getFeatures(drivers)
	fitAndEvaluate(features)
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

func getFeatures(drivers []driverSeasonalData) *base.DenseInstances {
	features := [][]float64{}
	labels := []bool{}
	for _, driver := range drivers {
		for i, race := range driver.races {
			if i < featureRaces {
				continue
			}
			raceFeatures := []float64{}
			for j := 1; j <= featureRaces; j++ {
				previousRace := driver.races[i - j]
				value := 0
				if previousRace.result == resultPosition {
					positionIndex := previousRace.position - 1
					if positionIndex < len(featurePositionScores) {
						value = featurePositionScores[positionIndex]
					}
				}
				raceFeatures = append(raceFeatures, float64(value))
			}
			label := race.result == resultPosition && race.position == 1
			labels = append(labels, label)
			features = append(features, raceFeatures)
		}
	}
	if len(features) != len(labels) {
		log.Fatalf("Invalid number of features or labels: %d features vs. %d labels", len(features), len(labels))
	}
	data := base.NewDenseInstances()
	raceSpecs := []base.AttributeSpec{}
	for i := range featureRaces {
		name := fmt.Sprintf("race%d", i + 1)
		attribute := base.NewFloatAttribute(name)
		spec := data.AddAttribute(attribute)
		raceSpecs = append(raceSpecs, spec)
	}
	winnerAttribute := base.NewBinaryAttribute("winner")
	winnerSpec := data.AddAttribute(winnerAttribute)
	data.AddClassAttribute(winnerAttribute)
	data.Extend(len(features))
	for row, currentFeatures := range features {
		for j := range currentFeatures {
			raceSpec := raceSpecs[j]
			feature := currentFeatures[j]
			data.Set(raceSpec, row, base.PackFloatToBytes(feature))
		}
		var label byte
		if labels[row] {
			label = 1
		} else {
			label = 0
		}
		data.Set(winnerSpec, row, []byte{label})
	}
	return data
}

func fitAndEvaluate(data *base.DenseInstances) {
	regression, err := linear_models.NewLogisticRegression("l2", 1.0, 1e-4)
	if err != nil {
		log.Fatalf("Failed to create logistic regression object: %v", err)
	}
	regression.Fit(data)
	predictions, err := regression.Predict(data)
	if err != nil {
		log.Fatalf("Failed to create predictions: %v", err)
	}
	confusion, err := evaluation.GetConfusionMatrix(data, predictions)
	if err != nil {
		log.Fatalf("Failed to calculate confusion matrix: %v", err)
	}
	fmt.Println(evaluation.GetSummary(confusion))
}
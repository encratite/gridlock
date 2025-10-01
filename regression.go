package main

import (
	"cmp"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/cdipaolo/goml/linear"
	"github.com/encratite/commons"
)

const (
	dataDirectory = "data"
	firstSeason = 2015
	lastSeason = 2025
	lastEventId = 17
	driverLimit = 6
	raceWindowSize = 10
	classThreshold = 0.38
	logisticMethod = "Batch Gradient Ascent"
	alpha = 0.0001
	regularization = 0
	maxIterations = 1000
	predictionsSeason = 2025
	predictionsId = raceWindowSize + 1
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
	pole bool
}

type featureMetaData struct {
	driver1 string
	driver2 string
	season int
	id int
}

type driverPredictionData struct {
	features []float64
	label float64
	metaData featureMetaData
}

func performRegression(predictions bool) {
	paths := downloadFiles()
	drivers := parseFiles(paths)
	features, labels, metaData := getFeatures(drivers)
	if !predictions {
		fitAndEvaluate(features, labels)
	} else {
		makePredictions(features, labels, metaData)
	}
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
		cells := htmlquery.Find(row, "/td[position() > 1 and position() < last()]")
		if len(cells) < 10 {
			log.Fatalf("Failed to find driver cells in %s", path)
		}
		races := []driverRaceResult{}
		for j, cell := range cells {
			id := j + 1
			if dataPath.season == lastSeason && id > lastEventId {
				break
			}
			firstText := htmlquery.FindOne(cell, "./text()[1]")
			resultText := htmlquery.InnerText(firstText)
			resultText = commons.Trim(resultText)
			resultText = strings.Replace(resultText, "†", "", 1)
			poleNode := htmlquery.FindOne(cell, ".//sup[text() = 'P']")
			position, err := commons.ParseInt(resultText)
			pole := poleNode != nil
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
					pole: pole,
				}
			}
			races = append(races, driverResult)
		}
		driver := driverSeasonalData{
			name: name,
			races: races,
		}
		drivers = append(drivers, driver)
	}
	return drivers
}

func getFeatures(drivers []driverSeasonalData) ([][]float64, []float64, []featureMetaData) {
	features := [][]float64{}
	labels := []float64{}
	metaData := []featureMetaData{}
	for i, driver1 := range drivers {
		for j, driver2 := range drivers {
			if i >= j {
				continue
			}
			for k, race1 := range driver1.races {
				if k < raceWindowSize {
					continue
				}
				getMatchingRace := func (race driverRaceResult) (driverRaceResult, bool) {
					matchingRace, exists := commons.Find(driver2.races, func (r driverRaceResult) bool {
						return r.season == race.season && r.id == race.id
					})
					return matchingRace, exists
				}
				race2, exists := getMatchingRace(race1)
				if !exists {
					continue
				}
				position12 := 0
				position10 := 0
				position20 := 0
				retired := 0
				missingRace := false
				for l := 1; l <= raceWindowSize; l++ {
					windowRace1 := driver1.races[k - l]
					windowRace2, exists := getMatchingRace(windowRace1)
					if !exists {
						missingRace = true
						break
					}
					r1p1 := windowRace1.isPosition(1)
					r1p2 := windowRace1.isPosition(2)
					r1p0 := !r1p1 && !r1p2
					r2p1 := windowRace2.isPosition(1)
					r2p2 := windowRace2.isPosition(2)
					r2p0 := !r2p1 && !r2p2
					if (r1p1 && r2p2) || (r2p1 && r1p2) {
						position12++
					} else if (r1p1 && r2p0) || (r2p1 && r1p0) {
						position10++
					} else if (r1p2 && r2p0) || (r2p2 && r1p0) {
						position20++
					}
					if windowRace1.result == resultRetired || windowRace2.result == resultRetired {
						retired++
					}
				}
				if missingRace {
					continue
				}
				raceFeatures := []float64{
					float64(position12),
					float64(position10),
					float64(position20),
					float64(retired),
				}
				label := 0.0
				if race1.isWin() || race2.isWin() {
					label = 1.0
				}
				driverMetaData := featureMetaData{
					driver1: driver1.name,
					driver2: driver2.name,
					season: race1.season,
					id: race1.id,
				}
				labels = append(labels, label)
				features = append(features, raceFeatures)
				metaData = append(metaData, driverMetaData)
			}
		}
	}
	return features, labels, metaData
}

func fitAndEvaluate(features [][]float64, labels []float64) {
	model := linear.NewLogistic(logisticMethod, alpha, regularization, maxIterations, features, labels)
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
		prediction := predictionVector[0] > classThreshold
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
	positiveTerm := float64(truePositives) / (float64(truePositives) + float64(falseNegatives))
	negativeTerm := float64(trueNegatives) / (float64(trueNegatives) + float64(falsePositives))
	youdensJ := positiveTerm + negativeTerm - 1.0
	f1Score := 2.0 * float64(truePositives) / (2.0 * float64(truePositives) + float64(falsePositives) + float64(falseNegatives))
	printRatio("True positives", truePositives)
	printRatio("True negatives", trueNegatives)
	printRatio("False positives", falsePositives)
	printRatio("False negatives", falseNegatives)
	printRatio("True labels", positiveLabels)
	fmt.Printf("Youden's J: %.3f\n", youdensJ)
	fmt.Printf("F1 score: %.3f\n", f1Score)
}

func makePredictions(features [][]float64, labels []float64, metaData []featureMetaData) {
	predictionData := []driverPredictionData{}
	for i, currentFeatures := range features {
		label := labels[i]
		currentMetaData := metaData[i]
		currentPredictionData := driverPredictionData{
			features: currentFeatures,
			label: label,
			metaData: currentMetaData,
		}
		predictionData = append(predictionData, currentPredictionData)
	}
	slices.SortFunc(predictionData, func (a, b driverPredictionData) int {
		meta1 := a.metaData
		meta2 := b.metaData
		if meta1.season != meta2.season {
			return cmp.Compare(meta1.season, meta2.season)
		}
		return cmp.Compare(meta1.id, meta2.id)
	})
	for id := predictionsId; true; id++ {
		i := slices.IndexFunc(predictionData, func (f driverPredictionData) bool {
			return f.metaData.season == predictionsSeason && f.metaData.id == id
		})
		if i == -1 {
			break
		}
		trainingFeatures := [][]float64{}
		trainingLabels := []float64{}
		for _, currentPredictionData := range predictionData[:i] {
			trainingFeatures = append(trainingFeatures, currentPredictionData.features)
			trainingLabels = append(trainingLabels, currentPredictionData.label)
		}
		model := linear.NewLogistic(logisticMethod, alpha, regularization, maxIterations, trainingFeatures, trainingLabels)
		model.Output = io.Discard
		err := model.Learn()
		if err != nil {
			log.Fatalf("Failed to train model: %v", err)
		}
		for j := i; j < len(predictionData); j++ {
			currentPredictionData := predictionData[j]
			currentMetaData := currentPredictionData.metaData
			if currentMetaData.season != predictionsSeason || currentMetaData.id != id {
				break
			}
			predictionVector, err := model.Predict(currentPredictionData.features)
			if err != nil {
				log.Fatalf("Failed to make prediction: %v", err)
			}
			prediction := predictionVector[0]
			format := "Season = %d, event ID = %d, driver 1 = %s, driver 2 = %s: %.3f\n"
			fmt.Printf(format, currentMetaData.season, currentMetaData.id, currentMetaData.driver1, currentMetaData.driver2, prediction)
		}
	}
}

func (r *driverRaceResult) isWin() bool {
	return r.isPosition(1)
}

func (r *driverRaceResult) isPosition(position int) bool {
	return r.result == resultPosition && r.position == position
}
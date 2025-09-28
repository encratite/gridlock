package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/encratite/commons"
)

const (
	dataDirectory = "data"
	driverLimit = 10
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
	_ = parseFiles(paths)
}

func downloadFiles() []wikiDataPath {
	commons.CreateDirectory(dataDirectory)
	paths := []wikiDataPath{}
	for year := 2020; year <= 2025; year++ {
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
	for _, path := range paths {
		_ = parseFile(path)
		break
	}
	return nil
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
	for i := range driverLimit {
		row := rows[i + 1]
		nameCell := htmlquery.FindOne(row, "/td[1]")
		if nameCell == nil {
			log.Fatalf("Failed to find name cell %s", path)
		}
		name := htmlquery.InnerText(nameCell)
		name = commons.Trim(name)
		fmt.Printf("\tDriver: %s\n", name)
		cells := htmlquery.Find(row, "/td[position() > 1 and position() < last()]/text()[1]")
		if len(cells) < 10 {
			log.Fatalf("Failed to find driver cells in %s", path)
		}
		results := []driverRaceResult{}
		for id, cell := range cells {
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
			results = append(results, driverResult)
			fmt.Printf("\t\tResult: %s\n", resultText)
		}
	}
	return nil
}


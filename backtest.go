package main

import (
	"cmp"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/encratite/commons"
	"gonum.org/v1/gonum/stat"
)

const (
	driverMinFileSize = 1024
	winnerPriceLimit = 0.95
	spread = 0.02
	enableStopLoss = false
	stopLoss = 0.80
	verbose = false
	positionSize = 0.2
)

type strategyType int

const (
	strategyPractice strategyType = iota
	strategyQualifying
	strategyRace
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

type strategyParameters struct {
	stratType strategyType
	bets []strategyBet
}

type strategyBet struct {
	position int
	yes bool
}

func runBacktest() {
	loadConfiguration()
	races := []raceData{}
	for _, raceConfig := range configuration.Races {
		race := loadRace(raceConfig)
		races = append(races, race)
	}
	stratTypes := []strategyType{
		strategyPractice,
		// strategyQualifying,
		// strategyRace,
	}
	betConfigurations := [][]strategyBet{
		{
			{
				position: 1,
				yes: false,
			},
		},
		/*
		{
			{
				position: 2,
				yes: false,
			},
		},
		{
			{
				position: 3,
				yes: false,
			},
		},
		{
			{
				position: 4,
				yes: false,
			},
		},
		{
			{
				position: 5,
				yes: false,
			},
		},
		{
			{
				position: 6,
				yes: false,
			},
		},
		*/
		{
			{
				position: 2,
				yes: true,
			},
			{
				position: 3,
				yes: true,
			},
		},
		{
			{
				position: 1,
				yes: false,
			},
			{
				position: 2,
				yes: true,
			},
			{
				position: 3,
				yes: true,
			},
		},
	}
	for _, stratType := range stratTypes {
		for _, bets := range betConfigurations {
			strategy := strategyParameters{
				stratType: stratType,
				bets: bets,
			}
			executeBacktest(strategy, races)
		}
	}
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

func executeBacktest(parameters strategyParameters, races []raceData) {
	cash := 1.0
	returns := []float64{}
	for _, race := range races {
		raceReturns := getRaceReturns(parameters, race)
		cash += raceReturns
		returns = append(returns, raceReturns)
	}
	percentage := 100.0 * (cash - 1.0)
	typeString := getStrategyTypeString(parameters.stratType)
	fmt.Printf("Backtest result for type \"%s\":\n", typeString)
	for _, bet := range parameters.bets {
		fmt.Printf("\tPosition %d: %t\n", bet.position, bet.yes)
	}
	riskAdjusted := stat.Mean(returns, nil) / stat.StdDev(returns, nil)
	fmt.Printf("\tReturns: %+.1f%% (%.2f RAR)\n\n", percentage, riskAdjusted)
}

func getRaceReturns(parameters strategyParameters, race raceData) float64 {
	drivers := race.drivers
	slices.SortFunc(drivers, func (a, b driverData) int {
		price1 := a.getPrice(parameters.stratType)
		price2 := b.getPrice(parameters.stratType)
		return cmp.Compare(price2, price1)
	})
	returns := 0.0
	for _, bet := range parameters.bets {
		i := bet.position - 1
		if i < 0 || i >= len(drivers) {
			log.Fatalf("Invalid bet position: %d", i)
		}
		driver := drivers[i]
		price := driver.getPrice(parameters.stratType)
		if !bet.yes {
			price = 1.0 - price
		}
		betSize := positionSize / float64(len(parameters.bets))
		if verbose {
			if bet.yes {
				fmt.Printf("Betting on %s at %.2f\n", driver.name, price)
			} else {
				fmt.Printf("Betting against %s at %.2f\n", driver.name, price)
			}
		}
		won := bet.yes == driver.winner
		if won {
			returns += betSize * (1.0 / (price + spread) - 1.0)
		} else {
			if enableStopLoss {
				returns -= betSize * (stopLoss - spread)
			} else {
				returns -= betSize
			}
		}
	}
	winner, exists := commons.Find(drivers, func (d driverData) bool {
		return d.winner
	})
	if !exists {
		log.Fatalf("Unable to find winner for race: %s", race.name)
	}
	if verbose {
		fmt.Printf("Returns: %.2f (%s, won by %s)\n", returns, race.name, winner.name)
	}
	return returns
}

func (d *driverData) getPrice(stratType strategyType) float64 {
	switch stratType {
	case strategyPractice:
		return d.practicePrice
	case strategyQualifying:
		return d.qualifyingPrice
	case strategyRace:
		return d.racePrice
	default:
		log.Fatalf("Invalid strategy type: %d", stratType)
	}
	return 0.0
}

func getStrategyTypeString(stratType strategyType) string {
	switch stratType {
	case strategyPractice:
		return "practice"
	case strategyQualifying:
		return "qualifying"
	case strategyRace:
		return "race"
	default:
		log.Fatalf("Invalid strategy type: %d", stratType)
	}
	return "unknown"
}
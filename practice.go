package main

import (
	"cmp"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/encratite/commons"
)

func printPracticePrices(driverString string) {
	driverNames := strings.Split(driverString, " ")
	loadConfiguration()
	races := loadRaces()
	for _, race := range races {
		fmt.Printf("%s:\n", race.name)
		drivers := race.drivers
		slices.SortFunc(drivers, func (a, b driverData) int {
			return cmp.Compare(b.qualifyingPrice, a.qualifyingPrice)
		})
		for _, driver := range drivers {
			match := false
			for _, driverName := range driverNames {
				if strings.Contains(driver.name, driverName) {
					match = true
					break
				}
			}
			if match {
				fmt.Printf("\t%s: %.2f\n", driver.name, driver.practicePrice)
			}
		}
	}
}

func printWinners() {
	loadConfiguration()
	races := loadRaces()
	for _, race := range races {
		winner, exists := commons.Find(race.drivers, func (d driverData) bool {
			return d.winner
		})
		if !exists {
			log.Fatalf("Unable to determine winner of %s", race.name)
		}
		fmt.Printf("%s: %s\n", race.name, winner.name)
	}
}
package main

import (
	"fmt"

	"gonum.org/v1/gonum/stat"
)

type priceBinGroup struct {
	name string
	bins []priceBin
}

type priceBin struct {
	priceMin float64
	priceMax float64
	prices []float64
	hits int
}

func analyzeOutcomes() {
	loadConfiguration()
	races := loadRaces()
	practiceGroup := newBinGroup("Practice")
	qualifyingGroup := newBinGroup("Qualifying")
	raceGroup := newBinGroup("Race")
	for _, race := range races {
		for _, driver := range race.drivers {
			practiceGroup.add(driver.practicePrice, driver.winner)
			qualifyingGroup.add(driver.qualifyingPrice, driver.winner)
			raceGroup.add(driver.racePrice, driver.winner)
		}
	}
	practiceGroup.print()
	qualifyingGroup.print()
	raceGroup.print()
}

func newBinGroup(name string) priceBinGroup {
	bins := []priceBin{
		newPriceBin(0.00, 0.025),
		newPriceBin(0.025, 0.05),
		newPriceBin(0.05, 0.10),
		newPriceBin(0.10, 0.20),
		newPriceBin(0.20, 0.30),
		newPriceBin(0.40, 1.00),
	}
	return priceBinGroup{
		name: name,
		bins: bins,
	}
}

func (g *priceBinGroup) add(price float64, outcome bool) {
	for i := range g.bins {
		bin := &g.bins[i]
		bin.add(price, outcome)
	}
}

func (g *priceBinGroup) print() {
	fmt.Printf("%s:\n", g.name)
	for _, bin := range g.bins {
		count := len(bin.prices)
		if count > 0 {
			meanPrice := stat.Mean(bin.prices, nil)
			percentage := 100.0 * float64(bin.hits) / float64(count)
			fmt.Printf("\t%.3f - %.3f: %.1f%% (mean %.3f, %d samples)\n", bin.priceMin, bin.priceMax, percentage, meanPrice, count)
		} else {
			fmt.Printf("\t%.3f - %.3f: -\n", bin.priceMin, bin.priceMax)
		}
	}
	fmt.Println("")
}

func newPriceBin(priceMin, priceMax float64) priceBin {
	return priceBin{
		priceMin: priceMin,
		priceMax: priceMax,
		prices: []float64{},
		hits: 0,
	}
}

func (b *priceBin) add(price float64, outcome bool) {
	if price < b.priceMin || price >= b.priceMax {
		return
	}
	b.prices = append(b.prices, price)
	if outcome {
		b.hits++
	}
}
package main

import (
	"flag"
)

func main() {
	backtest := flag.Bool("backtest", false, "Backtest F1 betting strategies")
	outcomes := flag.Bool("outcomes", false, "Analyze the distribution of outcomes")
	flag.Parse()
	if *backtest {
		runBacktest()
	} else if *outcomes {
		analyzeOutcomes()
	} else {
		flag.Usage()
	}
}
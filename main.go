package main

import (
	"flag"
)

func main() {
	backtest := flag.Bool("backtest", false, "Backtest F1 betting strategies")
	outcomes := flag.Bool("outcomes", false, "Analyze the distribution of outcomes")
	regression := flag.Bool("regression", false, "Run regression model on drivers")
	flag.Parse()
	if *backtest {
		runBacktest()
	} else if *outcomes {
		analyzeOutcomes()
	} else if *regression {
		performRegression()
	} else {
		flag.Usage()
	}
}
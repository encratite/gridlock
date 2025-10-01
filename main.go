package main

import (
	"flag"
)

func main() {
	backtest := flag.Bool("backtest", false, "Backtest F1 betting strategies")
	outcomes := flag.Bool("outcomes", false, "Analyze the distribution of outcomes")
	regression := flag.Bool("regression", false, "Run regression model on drivers")
	predict := flag.Bool("predict", false, "Perform predictions")
	practice := flag.String("practice", "", "Print pre-practice prices of drivers extracted from historical date, filtering for the names specified in the string passed to this argument")
	win := flag.Bool("win", false, "Can only be used with -practice, enables output of the winner of the race")
	flag.Parse()
	if *backtest {
		runBacktest()
	} else if *outcomes {
		analyzeOutcomes()
	} else if *regression {
		performRegression(false)
	} else if *predict {
		performRegression(true)
	} else if *practice != "" {
		printPracticePrices(*practice)
	} else if *win {
		printWinners()
	} else {
		flag.Usage()
	}
}
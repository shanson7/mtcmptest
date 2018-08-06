package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
	diff "github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

func main() {

	endpoint := flag.String("url", "http://localhost:6060/render", "URL for requests.")
	timeRange := flag.Int("range", 300, "How many seconds to fetch results for.")
	verbose := flag.Bool("verbose", false, "Verbose output.")
	speed := flag.Bool("speed", false, "Perform a speed test.")
	series := flag.Bool("series", true, "Perform a response comparison.")
	flag.Parse()

	until := int(time.Now().UnixNano() / int64(time.Second))
	from := until - *timeRange

	var testCnt int
	var testsFailed int
	for _, filen := range flag.Args() {
		fmt.Printf("\n===== Testing %s =====\n", filen)

		jsonFile, err := os.Open(filen)
		if err != nil {
			fmt.Println(err)
			continue
		}

		byteValue, _ := ioutil.ReadAll(jsonFile)
		jsonFile.Close()

		var targets map[string]string
		err = json.Unmarshal(byteValue, &targets)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if *series {
			var fileTestsFailed int
			for name, target := range targets {
				url := fmt.Sprintf("%s?target=%s&from=%d&until=%d&format=json", *endpoint, target, from, until)
				if !compareResponses(name, url, *verbose) {
					fmt.Println("FAILED")
					fileTestsFailed++
				}
			}

			if *verbose {
				if fileTestsFailed == 0 {
					fmt.Printf("- All tests passed in %s\n", filen)
				} else {
					fmt.Printf("- %d tests passed in %s\n", len(targets)-fileTestsFailed, filen)
					fmt.Printf("- %d tests FAILED in %s\n", fileTestsFailed, filen)
				}
			}
			testCnt += len(targets)
			testsFailed += fileTestsFailed
		}
		if *speed {
			fmt.Printf("\n===== Running Speed Test on %s =====\n", filen)
			rate := uint64(5) // per second
			duration := 120 * time.Second
			var vegetaTargetsNative []vegeta.Target
			var vegetaTargetsProxy []vegeta.Target
			for _, target := range targets {
				url := fmt.Sprintf("%s?target=%s&from=%d&until=%d&format=json", *endpoint, target, from, until)
				vegetaTargetsNative = append(vegetaTargetsNative, vegeta.Target{
					Method: "GET",
					URL:    url + "&process=any",
				})
				vegetaTargetsProxy = append(vegetaTargetsProxy, vegeta.Target{
					Method: "GET",
					URL:    url + "&process=none",
				})
			}

			targeterNative := vegeta.NewStaticTargeter(vegetaTargetsNative...)
			targeterProxy := vegeta.NewStaticTargeter(vegetaTargetsProxy...)
			attackerNative := vegeta.NewAttacker()
			attackerProxy := vegeta.NewAttacker()

			var metricsNative vegeta.Metrics
			var metricsProxy vegeta.Metrics

			resultNative := attackerNative.Attack(targeterNative, rate, duration, filen)
			resultProxy := attackerProxy.Attack(targeterProxy, rate, duration, filen)

			for res := range resultNative {
				metricsNative.Add(res)
			}

			for res := range resultProxy {
				metricsProxy.Add(res)
			}

			metricsNative.Close()
			metricsProxy.Close()

			fmt.Printf("---------- Native %s Latencies ----------\n", filen)
			fmt.Printf("Mean: %s\n", metricsNative.Latencies.Mean)
			fmt.Printf("50th percentile: %s\n", metricsNative.Latencies.P50)
			fmt.Printf("95th percentile: %s\n", metricsNative.Latencies.P95)
			fmt.Printf("99th percentile: %s\n", metricsNative.Latencies.P99)
			fmt.Printf("Max: %s\n", metricsNative.Latencies.Max)
			fmt.Printf("Success: %g%%\n", metricsNative.Success*100)
			fmt.Printf("Errors: %v\n\n", metricsNative.Errors)

			fmt.Printf("---------- Graphite (Python) %s Latencies ----------\n", filen)
			fmt.Printf("Mean: %s\n", metricsProxy.Latencies.Mean)
			fmt.Printf("50th percentile: %s\n", metricsProxy.Latencies.P50)
			fmt.Printf("95th percentile: %s\n", metricsProxy.Latencies.P95)
			fmt.Printf("99th percentile: %s\n", metricsProxy.Latencies.P99)
			fmt.Printf("Max: %s\n\n", metricsProxy.Latencies.Max)
			fmt.Printf("Success: %g%%\n", metricsNative.Success*100)
			fmt.Printf("Errors: %v\n\n", metricsNative.Errors)

			fmt.Println("---------- Speed Improvement ----------")
			fmt.Printf("Mean: x%.1f\n", metricsProxy.Latencies.Mean.Seconds()/metricsNative.Latencies.Mean.Seconds())
			fmt.Printf("50th percentile: x%.1f\n", metricsProxy.Latencies.P50.Seconds()/metricsNative.Latencies.P50.Seconds())
			fmt.Printf("95th percentile: x%.1f\n", metricsProxy.Latencies.P95.Seconds()/metricsNative.Latencies.P95.Seconds())
			fmt.Printf("99th percentile: x%.1f\n", metricsProxy.Latencies.P99.Seconds()/metricsNative.Latencies.P99.Seconds())
			fmt.Printf("Max: x%.1f\n\n", metricsProxy.Latencies.Max.Seconds()/metricsNative.Latencies.Max.Seconds())
		}
	}
	fmt.Print("\n\n")
	if testsFailed == 0 {
		fmt.Println("== All tests passed ==")
	} else {
		fmt.Printf("== %d tests passed ==\n", testCnt-testsFailed)
		fmt.Printf("== %d tests FAILED ==\n", testsFailed)
	}
}

func compareResponses(name string, url string, verbose bool) bool {
	fmt.Printf("\n----- Comparing %s -----\n", name)
	if verbose {
		fmt.Println(url)
	}
	metrictankURL := url + "&process=any"
	graphiteURL := url + "&process=none"
	graphiteResp, err := http.Get(graphiteURL)
	if err != nil {
		fmt.Println(err)
		return false
	}
	metricTankResp, err := http.Get(metrictankURL)
	if err != nil {
		fmt.Println(err)
		return false
	}

	defer graphiteResp.Body.Close()
	gJSON, err := ioutil.ReadAll(graphiteResp.Body)
	defer metricTankResp.Body.Close()
	mJSON, err := ioutil.ReadAll(metricTankResp.Body)

	// BUG. diff tool assumes top layer is an object.
	gJSON = append([]byte("{\"response\":"), gJSON...)
	gJSON = append(gJSON, byte('}'))
	mJSON = append([]byte("{\"response\":"), mJSON...)
	mJSON = append(mJSON, byte('}'))

	differ := diff.New()
	d, err := differ.Compare(gJSON, mJSON)
	if err != nil {
		fmt.Println("Invalid response")
		fmt.Printf("Graphite response: %s\n", gJSON)
		fmt.Printf("Metrictank response: %s\n", mJSON)
		return false
	}
	if d.Modified() {
		fmt.Println("Differences found:")
		var aJSON map[string]interface{}
		json.Unmarshal(gJSON, &aJSON)

		config := formatter.AsciiFormatterConfig{
			ShowArrayIndex: true,
			Coloring:       true,
		}

		formatter := formatter.NewAsciiFormatter(aJSON, config)
		diffString, _ := formatter.Format(d)

		fmt.Print(diffString)
		if verbose {
			fmt.Printf("Graphite response: %s\n\n", gJSON)
			fmt.Printf("Metrictank response: %s\n", mJSON)
		}
		return false
	}
	if verbose {
		fmt.Println("Indentical")
	}
	return true

}

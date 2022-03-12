package aaa

import (
	"fmt"
	"log"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

func GetNationalAverages() string {
	// Request the HTML page.
	res, err := http.Get("https://gasprices.aaa.com")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	avgGasPrice := ""
	midGasPrice := ""
	premiumGasPrice := ""
	doc.Find("tr td").Each(func(i int, s *goquery.Selection) {
		if i == 1 {
			avgGasPrice = s.Text()
		} else if i == 2 {
			midGasPrice = s.Text()
		} else if i == 3 {
			premiumGasPrice = s.Text()
		}
	})

	return fmt.Sprintf("Today's National Average Gas Prices:\n\tRegular: %s\n\tMid: %s\n\tPremium: %s", avgGasPrice, midGasPrice, premiumGasPrice)
}

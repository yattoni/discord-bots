package buddy

import (
	"fmt"
	"log"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

func GetFromGasBuddy(url, city string) string {
	res, err := http.Get(url)
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
	doc.Find("div.GasPriceCollection-module__row___2JDQq").Each(func(i int, s *goquery.Selection) {
		// fmt.Println(s.Text())
		if i == 1 {
			s.Find(".FuelTypePriceDisplay-module__price___3iizb").Each(func(i int, s *goquery.Selection) {
				if i == 0 {
					avgGasPrice = s.Text()
				} else if i == 1 {
					midGasPrice = s.Text()
				} else if i == 2 {
					premiumGasPrice = s.Text()
				}
			})
		}
	})

	return fmt.Sprintf("%s:\n\tRegular: %s\n\tMid: %s\n\tPremium: %s", city, avgGasPrice, midGasPrice, premiumGasPrice)
}

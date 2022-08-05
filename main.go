package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	symbol = "xnousdt"
	url    = "wss://stream.binance.com:9443/ws/" + symbol + "@miniTicker"
)

var (
	mutex   sync.Mutex
	rate    float64
	fxRates = make(map[string]float64)
)

func main() {
	var (
		accessKey = flag.String("k", "", "Access key")
		port      = flag.Int("p", 8080, "Listen port")
		c         = &client{}
	)
	flag.Parse()
	if err := c.connect(); err != nil {
		log.Fatal(err)
	}
	go c.binanceLoop()
	go fxLoop(*accessKey)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		currencyVal, ok := query["currency"]
		if !ok {
			return
		}
		currency := strings.ToUpper(currencyVal[0])
		amountVal, ok := query["amount"]
		if !ok {
			return
		}
		amount, err := strconv.ParseFloat(amountVal[0], 64)
		if err != nil {
			return
		}
		mutex.Lock()
		defer mutex.Unlock()
		if currency != "USD" {
			amount /= fxRates[currency]
			amount *= fxRates["USD"]
		}
		fmt.Fprint(w, amount/rate)
	})
	log.Fatal(http.ListenAndServe(fmt.Sprint(":", *port), nil))
}

type client struct{ conn *websocket.Conn }

func (c *client) connect() (err error) {
	c.conn, _, err = websocket.DefaultDialer.Dial(url, nil)
	return
}

func (c *client) binanceLoop() {
	for {
		var v struct {
			C float64 `json:",string"`
		}
		if c.conn.ReadJSON(&v) != nil {
			c.conn.Close()
			for c.connect() != nil {
				time.Sleep(3 * time.Second)
			}
			continue
		}
		mutex.Lock()
		rate = v.C
		mutex.Unlock()
	}
}

func fxLoop(accessKey string) {
	for ; ; time.Sleep(time.Hour) {
		resp, err := http.Get("http://api.exchangeratesapi.io/v1/latest?access_key=" + accessKey)
		if err != nil {
			continue
		}
		var v struct{ Rates map[string]float64 }
		if json.NewDecoder(resp.Body).Decode(&v) == nil {
			mutex.Lock()
			fxRates = v.Rates
			mutex.Unlock()
		}
		resp.Body.Close()
	}
}

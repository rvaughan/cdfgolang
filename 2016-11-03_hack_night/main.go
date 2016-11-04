package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/signal"
    "net/http"
    "strconv"
    "sync"
    "syscall"

    "github.com/labstack/echo"
    "github.com/labstack/echo/engine/standard"
    "github.com/labstack/echo/middleware"
)


var results = make(map[int][]string)


type Query struct {
    id int
    query string

    response *QueryResponse
}


type QueryResponse struct {
    StartIndex int `json:"start_index"`
    EndIndex int `json:"end_index"`
    Items []ItemType `json:"items"`
}


type ItemType struct {
    Address AddressType `json:"address"`
}


type AddressType struct {
    AddressList string `json:"address_line_1"`
}


func registerSIGINTHandler(cleanup chan bool) {
    // Register for SIGINT.
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

    // Start a goroutine that will trigger the cleanup channel when it completes.
    go func() {
        for {
            s := <-signalChan
            log.Println("Received", s, "shutting down...")

            cleanup <- true
        }
    }()
}


func process_data(c chan *Query) {
    channel_closed := false

    for (!channel_closed) {
        select {
            case query, ok := <- c:
                if (!ok) {
                    channel_closed = true
                } else {
                    r := make([]string, 0)

                    for _, addr := range query.response.Items {

                        r = append(r, addr.Address.AddressList)

                        results[query.id] = r
                    }
                }
        }
    }
}


func make_request(query *Query, results_chan chan *Query) {
    url := fmt.Sprintf("https://api.companieshouse.gov.uk/search/companies?q=%s", query.query)
    apikey := "your_api_key_goes_here"

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        log.Println("Error :", err)
    }

    req.SetBasicAuth(apikey, "")

    client := &http.Client{}

    resp, e_resp := client.Do(req)
    if (e_resp != nil) {
        log.Println(e_resp)
    } else {
        defer resp.Body.Close()
        contents, err := ioutil.ReadAll(resp.Body)
        if (err != nil) {
            log.Printf("%s\n", err)
        } else {
            result := &QueryResponse{}

            e := json.Unmarshal(contents, result)
            if (e == nil) {
                query.response = result
                results_chan <- query
            }
        }
    }
}


func process_requests(queries chan *Query, results_chan chan *Query) {
    channel_closed := false

    for (!channel_closed) {
        select {
            case query, ok := <- queries:
                if (!ok) {
                    channel_closed = true
                } else {
                    go make_request(query, results_chan)
                }
        }
    }
}


func main() {
    cleanup_chan := make(chan bool, 1)

    registerSIGINTHandler(cleanup_chan)

    results_chan := make(chan *Query, 20)

    go process_data(results_chan)

    query_chan := make(chan *Query, 50)

    go process_requests(query_chan, results_chan)

    e := echo.New()

    // Middleware
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())

    mu := &sync.Mutex{}

    query_id := 1

    e.Post("/retrieve/:query", func(c echo.Context) error {
        mu.Lock()

        qry := &Query {
            id: query_id,
            query: c.Param("query"),
        }
        query_chan <- qry

        response := fmt.Sprintf("{\"id\": %d}", query_id)

        query_id += 1

        mu.Unlock()

        return c.String(http.StatusOK, response)
    })

    e.Get("/:id", func(c echo.Context) error {
        id, _ := strconv.Atoi(c.Param("id"))
        if data, ok := results[id]; ok {
            out := ""
            for _, d := range data {
                out = fmt.Sprintf("%s\n%s", out, d)
            }

            return c.String(http.StatusOK, out)
        } else {
            return c.String(http.StatusNotFound, "{nope}")
        }
    })

    // Start server
    e.Run(standard.New(":8000"))

    <- cleanup_chan
}

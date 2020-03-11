// This code is heavily --inspired by-- **stolen from** the grafana API examples.
// https://github.com/grafana-tools/sdk/blob/master/cmd/
// Rather than separate commands for dashboard and datasources, I have combined the logic into a single
// file with appropriate changes.
// dataset files are stored in ./datasources
// dashboards are stored in ./dashboards
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/gosimple/slug"
	"github.com/grafana-tools/sdk"
)

func main() {
	imode := flag.Bool("import", false, "import dashboards into grafana. (default: false)")
	grafana := flag.String("grafana", "http://localhost:3000", "Grafana HTTP endpoint")
	apikey := flag.String("apikey", os.Getenv("GRAFANA_API_KEY"), "Grafana API key")
	flag.Parse()

	if *grafana == "" {
		log.Fatal("Unknown grafana API key. Please set GRAFANA_API_KEY or use the -apikey flag")
	}

	client := sdk.NewClient(*grafana, *apikey, sdk.DefaultHTTPClient)

	if *imode {
		// import mode
		log.Println("importing datasources")
		ImportDatasources(client)
		log.Println("importing dashboards")
		ImportDashboards(client)
	} else {
		// backup mode
		log.Println("backing up datasources")
		BackupDatasources(client)
		log.Println("backing up dashboards")
		BackupDashboards(client)
	}

}

// ImportDatasources imports datasource json files from ./datasources and uploads them to the
// grafana api
// Credit:
// https://raw.githubusercontent.com/grafana-tools/sdk/master/cmd/import-datasources/main.go
func ImportDatasources(c *sdk.Client) {
	var (
		datasources []sdk.Datasource
		filesInDir  []os.FileInfo
		rawDS       []byte
		status      sdk.StatusMessage
		err         error
	)
	if datasources, err = c.GetAllDatasources(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	filesInDir, err = ioutil.ReadDir("./datasources")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
	for _, file := range filesInDir {
		if strings.HasSuffix(file.Name(), ".json") {
			if rawDS, err = ioutil.ReadFile(file.Name()); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				continue
			}
			var newDS sdk.Datasource
			if err = json.Unmarshal(rawDS, &newDS); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				continue
			}
			for _, existingDS := range datasources {
				if existingDS.Name == newDS.Name {
					sm, err := c.DeleteDatasource(existingDS.ID)
					if err != nil {
						log.Print(sm.Message)
					}
					break
				}
			}
			if status, err = c.CreateDatasource(newDS); err != nil {
				fmt.Fprint(os.Stderr, fmt.Sprintf("error on importing datasource %s with %s (%s)", newDS.Name, err, *status.Message))
			}
		}
	}
}

// ImportDashboards loads dashboard json files from ./dashboards and uplaods them to the grafana api
// Credit:
// https://raw.githubusercontent.com/grafana-tools/sdk/master/cmd/import-dashboards/main.go
func ImportDashboards(c *sdk.Client) {
	var (
		filesInDir []os.FileInfo
		rawBoard   []byte
		err        error
	)
	filesInDir, err = ioutil.ReadDir("./dashboards")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range filesInDir {
		if strings.HasSuffix(file.Name(), ".json") {
			if rawBoard, err = ioutil.ReadFile("./dashboards/" + file.Name()); err != nil {
				log.Println(err)
				continue
			}
			var board sdk.Board
			if err = json.Unmarshal(rawBoard, &board); err != nil {
				log.Println(err)
				continue
			}
			sm, err := c.DeleteDashboard(board.UpdateSlug())
			if err != nil {
				log.Print(sm.Message)
			}
			_, err = c.SetDashboard(board, false)
			if err != nil {
				log.Printf("error on importing dashboard %s", board.Title)
				continue
			}
		}
	}
}

// BackupDatasources searches a grafana endpoint for datasets and saves them to ./datasets
// Credit:
// https://raw.githubusercontent.com/grafana-tools/sdk/master/cmd/backup-datasources/main.go
func BackupDatasources(c *sdk.Client) {
	var (
		datasources []sdk.Datasource
		dsPacked    []byte
		meta        sdk.BoardProperties
		err         error
	)
	if datasources, err = c.GetAllDatasources(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	for _, ds := range datasources {
		if dsPacked, err = json.Marshal(ds); err != nil {
			fmt.Fprintf(os.Stderr, "%s for %s\n", err, ds.Name)
			continue
		}
		if err = ioutil.WriteFile(fmt.Sprintf("./datasources/%s.json", slug.Make(ds.Name)), dsPacked, os.FileMode(int(0666))); err != nil {
			fmt.Fprintf(os.Stderr, "%s for %s\n", err, meta.Slug)
		}
	}
}

// BackupDashboards searches a grafana endpoint for dashboards with the "testground" tag
// and saves them into ./dashboards
// Credit:
// https://raw.githubusercontent.com/grafana-tools/sdk/master/cmd/backup-dashboards/main.go
func BackupDashboards(c *sdk.Client) {
	var (
		boardLinks []sdk.FoundBoard
		rawBoard   []byte
		meta       sdk.BoardProperties
		err        error
	)
	if boardLinks, err = c.SearchDashboards("", false, "testground"); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	var board sdk.Board
	for _, link := range boardLinks {
		if rawBoard, meta, err = c.GetRawDashboardBySlug(link.URI); err != nil {
			fmt.Fprintf(os.Stderr, "%s for %s\n", err, link.URI)
			continue
		}
		err := json.Unmarshal(rawBoard, &board)
		if err != nil {
			log.Println(err)
			continue
		}
		pretty, err := json.MarshalIndent(board, "", "    ")
		if err != nil {
			log.Printf("couldn't pretty print %s. continuing anyway.", meta.Slug)
			pretty = rawBoard
		}
		if err = ioutil.WriteFile(fmt.Sprintf("./dashboards/%s.json", meta.Slug), pretty, os.FileMode(int(0666))); err != nil {
			fmt.Fprintf(os.Stderr, "%s for %s\n", err, meta.Slug)
		}
	}
}

/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// syncMindergasCmd represents the syncMindergas command
var syncMindergasCmd = &cobra.Command{
	Use:   "sync-mindergas",
	Short: "Synchronizes gas meter readings to mindergas.nl",
	Run:   startSync,
}

// BasicInformation the basic information type, as returned by the P1 meter API
type BasicInformation struct {
	ProductName     string `json:"product_name"`
	ProductType     string `json:"product_type"`
	Serial          string `json:"serial"`
	FirmwareVersion string `json:"firmware_version"`
	ApiVersion      string `json:"api_version"`
}

// External the external information type, as returned by the P1 meter API
type External struct {
	UniqueId  string  `json:"unique_id"`
	Type      string  `json:"type"`
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
}

// Data the data type, as returned by the P1 meter API
type Data struct {
	External []*External `json:"external"`
}

func init() {
	rootCmd.AddCommand(syncMindergasCmd)
}

// startSync starts the mindergas.nl sync loop
func startSync(cmd *cobra.Command, args []string) {
	// Check if the ip address has been set
	ip := viper.GetString("p1.ip")
	if ip == "" {
		slog.Error("No P1 ip configured, please set the p1.ip key to the ip address of your P1 meter")
		os.Exit(1)
	}

	// Check if the mindergas.nl token has been set
	token := viper.GetString("mindergas.token")
	if token == "" {
		slog.Error("No mindergas token configured, please set the mindergas.token key to the authentication token of your mindergas.nl account")
		os.Exit(1)
	}

	// Retrieve the basic information
	basicInformation := getBasicInformation(ip)
	slog.Info(
		"Successfully connected", "ip", ip, "productName", basicInformation.ProductName, "serial", basicInformation.Serial, "firmwareVersion", basicInformation.FirmwareVersion,
	)

	// Run an endless loop until the user quits the program
	loop(ip, token)
}

// getBasicInformation retrieves the basic information from the P1 meter
func getBasicInformation(ip string) *BasicInformation {
	basicInformation := &BasicInformation{}
	client := resty.New()
	_, err := client.R().
		SetHeader("Accept", "application/json").
		SetResult(basicInformation).
		Get("http://" + ip + "/api")

	if err != nil {
		slog.Error("Could not retrieve data from the P1 meter", "err", err)
		os.Exit(1)
	}

	return basicInformation
}

// loop checks for a valid export window (every day starting from 23:59), retrieves the gas meter reading from the P1 meter, and uploads it to the mindergas.nl API
func loop(ip string, token string) {
	latestExportDay := -1
	for range time.Tick(5 * time.Second) {
		now := time.Now()
		slog.Debug(fmt.Sprintf("Checking for export window, current time: %s", now.Format("2006-01-02 15:04:05")))

		// Check if we are almost at the end of the day and we haven't already exported the data
		if now.Hour() == 19 && now.Minute() == 9 && (latestExportDay == -1 || latestExportDay != now.Day()) {
			slog.Info(fmt.Sprintf("In export window, current time: %s", now.Format("2006-01-02 15:04:05")))

			// Retrieve the data
			slog.Info("Retrieving data from P1 meter")
			data := &Data{}
			client := resty.New()
			_, err := client.R().
				SetHeader("Accept", "application/json").
				SetResult(data).
				Get("http://" + ip + "/api/v1/data")

			if err != nil {
				slog.Error("Could not retrieve data from the P1 meter", "err", err)
			} else {
				success := false
				for _, external := range data.External {
					if external.Type == "gas_meter" {
						timestampAsString := strconv.FormatInt(external.Timestamp, 10)
						readingTime, err := time.Parse("060102150405", timestampAsString) // YYMMDDhhmmss
						if err != nil {
							slog.Error(fmt.Sprintf("Could not parse timestamp [%d]", external.Timestamp), "timestamp", external.Timestamp)
							continue
						}

						slog.Info("Found gas meter reading", "value", external.Value, "unit", external.Unit, "readingTime", readingTime.Format("2006-01-02 15:04:05"))

						// Sleep a random amount of minutes between 0 and 15
						sleepMinutes := rand.Intn(14) + 1
						slog.Info(fmt.Sprintf("Sleeping %d minutes to reduce load on API server", sleepMinutes))
						time.Sleep(time.Duration(sleepMinutes) * time.Minute)

						// Post data to mindergas.nl
						postBody := fmt.Sprintf(`{"date":"%s", "reading":"%f"}`, readingTime.Format("2006-01-02"), external.Value)
						slog.Info("Posting to mindergas.nl", "data", postBody)
						resp, err := client.R().
							SetHeader("Content-Type", "application/json").
							SetHeader("AUTH-TOKEN", token).
							SetBody(postBody).
							Post("https://www.mindergas.nl/api/meter_readings")

						if err != nil {
							slog.Error("Could not post reading to mindergas.nl", "err", err)
							continue
						}

						slog.Info("The mindergas.nl API responded with status", "status", resp.Status())
						success = true

						slog.Debug("Marked run ass successful")
					}
				}

				// Make sure we don't export again today
				if success {
					latestExportDay = now.Day()
				}
			}
		}
	}
}

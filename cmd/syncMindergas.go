/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
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
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if the ip address has been set
		ip := viper.GetString("p1.ip")
		if ip == "" {
			fmt.Println("No P1 ip configured, please set the p1.ip key to the ip address of your P1 meter")
			os.Exit(1)
		}

		// Check if the mindergas.nl token has been set
		token := viper.GetString("mindergas.token")
		if token == "" {
			fmt.Println("No mindergas token configured, please set the mindergas.token key to the authentication token of your mindergas.nl account")
			os.Exit(1)
		}

		// Basic information
		type BasicInformation struct {
			ProductName     string `json:"product_name"`
			ProductType     string `json:"product_type"`
			Serial          string `json:"serial"`
			FirmwareVersion string `json:"firmware_version"`
			ApiVersion      string `json:"api_version"`
		}

		// Print some information
		basicInformation := &BasicInformation{}
		client := resty.New()
		_, err := client.R().
			SetHeader("Accept", "application/json").
			SetResult(basicInformation).
			Get("http://" + ip + "/api")

		if err != nil {
			fmt.Printf("Could not retrieve data from the P1 meter: %s", err)
			os.Exit(1)
		}

		fmt.Printf("Connected to %s, a %s with serial %s and firmware version %s\n", ip, basicInformation.ProductName, basicInformation.Serial, basicInformation.FirmwareVersion)

		// Data
		type External struct {
			UniqueId  string  `json:"unique_id"`
			Type      string  `json:"type"`
			Timestamp int64   `json:"timestamp"`
			Value     float64 `json:"value"`
			Unit      string  `json:"unit"`
		}

		type Data struct {
			External []*External `json:"external"`
		}

		// Call the data endpoint every minute
		latestExportDay := -1
		for range time.Tick(5 * time.Second) {
			// Check if we are almost at the end of the day and we haven't already exported the data
			now := time.Now()
			if now.Hour() == 20 && now.Minute() == 49 && (latestExportDay == -1 || latestExportDay != now.Day()) {
				fmt.Println("In export window, current time: " + now.Format("2006-01-02 15:04:05"))

				// Retrieve the data
				fmt.Println("Retrieving data from P1 meter")
				data := &Data{}
				client := resty.New()
				_, err := client.R().
					SetHeader("Accept", "application/json").
					SetResult(data).
					Get("http://" + ip + "/api/v1/data")

				if err != nil {
					fmt.Printf("Could not retrieve data from the P1 meter: %s\n", err)
				} else {

					success := false
					for _, external := range data.External {
						if external.Type == "gas_meter" {
							timestampAsString := strconv.FormatInt(external.Timestamp, 10)
							readingTime, err := time.Parse("060102150405", timestampAsString) // YYMMDDhhmmss
							if err != nil {
								fmt.Printf("Could not parse timestamp [%d]\n", external.Timestamp)
								continue
							}

							fmt.Printf("Found gas meter with reading [%f %s] from [%s]\n", external.Value, external.Unit, readingTime.Format("2006-01-02 15:04:05"))

							// Sleep a random amount of minutes between 0 and 15
							sleepMinutes := rand.Intn(14) + 1
							fmt.Printf("Sleeping %d minutes to reduce load on API server", sleepMinutes)
							time.Sleep(time.Duration(sleepMinutes) * time.Minute)

							// Post data to mindergas.nl
							postBody := fmt.Sprintf(`{"date":"%s", "reading":"%f"}`, readingTime.Format("2006-01-02"), external.Value)
							fmt.Printf("Posting [%s] to mindergas.nl\n", postBody)
							resp, err := client.R().
								SetHeader("Content-Type", "application/json").
								SetHeader("AUTH-TOKEN", token).
								SetBody(postBody).
								Post("https://www.mindergas.nl/api/meter_readings")

							if err != nil {
								fmt.Printf("Could not post reading to mindergas.nl: %s\n", err)
								continue
							}

							fmt.Printf("The mindergas.nl API responded with status %s\n", resp.Status())
							success = true
						}
					}

					// Make sure we don't export again today
					if success {
						latestExportDay = now.Day()
					}
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(syncMindergasCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// syncMindergasCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// syncMindergasCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"git.mills.io/yarnsocial/yarn/client"
)

// timelineCmd represents the pub command
var timelineCmd = &cobra.Command{
	Use:     "timeline [flags]",
	Aliases: []string{"view", "show", "events"},
	Short:   "Display your timeline",
	Long: `The timeline command retrieve the timeline from the logged in
Yarn.social account.`,
	//Args:    cobra.NArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		uri := viper.GetString("uri")
		token := viper.GetString("token")
		cli, err := client.NewClient(
			client.WithURI(uri),
			client.WithToken(token),
		)
		if err != nil {
			log.WithError(err).Error("error creating client")
			os.Exit(1)
		}

		outputJSON, err := cmd.Flags().GetBool("json")
		if err != nil {
			log.WithError(err).Error("error getting json flag")
			os.Exit(1)
		}

		outputRAW, err := cmd.Flags().GetBool("raw")
		if err != nil {
			log.WithError(err).Error("error getting raw flag")
			os.Exit(1)
		}

		timeline(cli, outputJSON, outputRAW, args)
	},
}

func init() {
	RootCmd.AddCommand(timelineCmd)

	timelineCmd.Flags().BoolP(
		"json", "j", false,
		"Output in JSON for processing with eg jq",
	)

	timelineCmd.Flags().BoolP(
		"raw", "r", false,
		"Output in raw text for processing with eg grep",
	)

}

func timeline(cli *client.Client, outputJSON, outputRAW bool, args []string) {
	// TODO: How do we get more pages?
	res, err := cli.Timeline(0)
	if err != nil {
		log.WithError(err).Error("error retrieving timeline")
		os.Exit(1)
	}

	sort.Sort(res.Twts)

	if outputJSON {
		data, err := json.Marshal(res)
		if err != nil {
			log.WithError(err).Error("error marshalling json")
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		sort.Sort(sort.Reverse(res.Twts))
		for _, twt := range res.Twts {
			if outputRAW {
				PrintTwtRaw(twt)
			} else {
				PrintTwt(twt, time.Now(), cli.Twter)
				fmt.Println()
			}
		}
	}
}

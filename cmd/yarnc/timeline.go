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

		timeline(cli, outputJSON, args)
	},
}

func init() {
	RootCmd.AddCommand(timelineCmd)

	timelineCmd.Flags().Bool(
		"json", false,
		"Output raw JSON for processing with eg jq",
	)

}

func timeline(cli *client.Client, outputJSON bool, args []string) {
	// TODO: How do we get more pages?
	res, err := cli.Timeline(0)
	if err != nil {
		log.WithError(err).Error("error retrieving timeline")
		os.Exit(1)
	}

	sort.Sort(sort.Reverse(res.Twts))

	if outputJSON {
		data, err := json.Marshal(res)
		if err != nil {
			log.WithError(err).Error("error marshalling json")
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		for _, twt := range res.Twts {
			PrintTwt(twt, time.Now())
			fmt.Println()
		}
	}
}

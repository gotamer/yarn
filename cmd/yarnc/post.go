package main

import (
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"git.mills.io/yarnsocial/yarn/client"
)

// postCmd represents the pub command
var postCmd = &cobra.Command{
	Use:     "post [flags]",
	Aliases: []string{"tweet", "twt", "new", "yarn"},
	Short:   "Post a new twt to a Yarn.social pod",
	Long: `The post command makes a new post to a Yarn.social pod.
if the optional flag -a/--post-as is used the post is made from that specified
feed (persona) if the logged in user owned that feed.`,
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

		postAs, err := cmd.Flags().GetString("post-as")
		if err != nil {
			log.WithError(err).Error("error getting post-as flag")
			os.Exit(1)
		}

		post(cli, postAs, args)
	},
}

func init() {
	RootCmd.AddCommand(postCmd)

	postCmd.Flags().StringP(
		"post-as", "a", "",
		"Post as a different feed (default: primary account feed)",
	)
}

func post(cli *client.Client, postAs string, args []string) {
	text := strings.Join(args, " ")

	if text == "" {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.WithError(err).Error("error reading text from stdin")
			os.Exit(1)
		}
		text = string(data)
	}

	if text == "" {
		log.Error("no text provided")
		os.Exit(1)
	}

	if postAs == "" {
		log.Info("posting twt...")
	} else {
		log.Infof("posting twt as %s...", postAs)
	}

	_, err := cli.Post(text, postAs)
	if err != nil {
		log.WithError(err).Error("error making post")
		os.Exit(1)
	}

	log.Info("post successful")
}

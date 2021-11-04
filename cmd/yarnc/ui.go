package main

import (
	"fmt"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/dustin/go-humanize"
)

func red(s string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", s)
}
func green(s string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", s)
}
func yellow(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}
func boldgreen(s string) string {
	return fmt.Sprintf("\033[32;1m%s\033[0m", s)
}
func blue(s string) string {
	return fmt.Sprintf("\033[34m%s\033[0m", s)
}

func PrintFollowee(nick, url string) {
	fmt.Printf("> %s @ %s",
		yellow(nick),
		url,
	)
}

func PrintFolloweeRaw(nick, url string) {
	fmt.Printf("%s: %s\n", nick, url)
}

func PrintTwt(twt types.Twt, now time.Time, me types.Twter) {
	text := FormatTwt(fmt.Sprintf("%t", twt))
	time := humanize.Time(twt.Created())
	nick := green(twt.Twter().DomainNick())
	hash := blue(twt.Hash())

	if twt.Mentions().IsMentioned(me) {
		nick = boldgreen(twt.Twter().DomainNick())
	}

	fmt.Printf(
		"> %s (%s) [%s]\n%s\n",
		nick, time, hash, text,
	)
}

func PrintTwtRaw(twt types.Twt) {
	fmt.Printf(
		"%s\t%s\t%t\n",
		twt.Twter().URL,
		twt.Created().Format(time.RFC3339),
		twt,
	)
}

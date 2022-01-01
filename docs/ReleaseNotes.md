Hello Yarners! 🤗

Happy New Year! #2021 #NewYearsEve 🥳

For those of you that live on _this_ side of the planet, Welcome to #2022 🥳

----

This year in [Yarn](https://yarn.social) we have a brand new version of `yarnd`
for you all with exciting new features! 🥳 Some of you have already been using them! 😅

Highlights:

- **NEW** Peers _try_ to validate the authenticity of injected Twts between peers.This has _finally_ #resolved the issue with my Avatar that kept reverting back and forth 😂
- **NEW** New `refresh` Metadata field added to the [Metadata ext](https://dev.twtxt.net/doc/metadataextension.html) allowing feed authors to _manually_ hint to clients as to how often their feeds should be fetched.
- **NEW** Moving Average Feed Refresh (_currently in experimental_) feature (Enable with `moving_average_feed_refresh`) that automatically backs off fetching/refreshing feeds based on an exponential moving average of feed's update frequency. This helps to reduce traffic to pods and external feeds but keep the network convergent within 60s to 10m (_on-pod are always instantaneous_) 🚀
- **NEW** Improvements to the `yarnc timeline` command to support `-r/--reverse` and `-n/--twts` 🧑‍💻
- **NEW** Support for pruning old dead accounts a multi-user pod _might_ accumulate. This sends an email to the Pod Owner on a weekly basis and uses a heuristic score ranges from 1000 (_every user_) to 2993 (_users who have never posted, never updated any part of their profile_). A Poderator (_Pod Owner/Operator_) is sent a weekly candidates of up to 10 users with a score `> 1200` for consideration. 👻
- **NEW** Active Users. Pods now measure two key metrics a Poderator can track, Daily Active Users (DAU) and Monthly Active Users (MAU). These are only accurate to a day. 📈
- **NEW** New followers tracking that is now _actually_ accurate and is pruned once-per-week. So now your "Followers" count, even across pods **will** be accurate and up-to-date 🥳
- **FIXED** Finally squished that annoying bug 🐞 causing profile data to flip-flop
- **FIXED** The `LookupHandler()` was fixed to support auto/tab-completion for external feeds! 🥳
- **UPDATE** Update Chinese and Taiwanese translations! 🇨🇳 🇹🇼

Big thanks to @gbmor for driving the need for some much needed network optimizations!
Thanks again to @ullarah for the many UI/UX updates and improvements! 🙇‍♂️
And thank you again to @venjiang for updating the CN and TW translations! 👌

As per usual, please provide feedback to @prologic or reply to this Yarn 🤗

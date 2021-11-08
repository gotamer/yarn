---
layout: page
title: "Archive Feeds Extension"
category: doc
date: 2021-10-30 11:00:00
order: 3
---

At [twtxt.net](https://twtxt.net/) **Archive Feeds** were invented as an
extension to the original [Twtxt File Format
Specification](https://twtxt.readthedocs.io/en/latest/user/twtxtfile.html#format-specification).

## Purpose

Feeds grow over time. To avoid feeds of virtually unlimited size,
pagination can be used to move old twts to a different (partial) feed.
Clients can then choose to retrieve only some of those feeds.

## Main Feed and Archived Feeds

There is exactly one main feed, which is the same as the traditional
twtxt.txt file. This feed keeps growing by adding new twts at the end
(this differs from the original twtxt spec, which allowed adding new
twts anywhere in the feed). Deletion or editing of twts anywhere in the
feed is allowed.

Once the main feed is "full", some or all of its twts can be moved to a
different feed: an archived feed. There can be any number of archived
feeds. Once they are made public, they are supposed to be left alone and
won't receive further updates. Deletion or editing is still allowed, but
feed authors should not expect clients to retrieve archived feeds on a
regular basis (or at all). When moving twts to an archived feed, their
relative order should be retained. A twt should only appear in one feed,
either the main feed or an archived feed, but not in both.

A feed's author decides when a feed is "full" and should be archived.
For example, this can be based on the number of twts in the feed or it
can be based on date ranges.

The main feed and all archived feeds form a linked list using the
[metadata](metadataextension.html) field described below.

## Format

The main feed can contain a [metadata](metadataextension.html) field
called `prev` which points to the URL of an archived feed (i.e., it
contains *older* twts):

```
# url = https://example.com/twtxt.txt
# url = gopher://example.com/0/twtxt.txt
# nick = cathy
# prev = kpw257a twtxt-2021-10-18.txt
```

The file names of archived feeds are implementation specific and don't
carry special meaning.

Archived feeds *can* contain another `prev` field to point to yet
another archived feed.

The first value of `prev` is the [twt hash](twthashextension.html) of
the last twt in that feed. It is provided as a hint for clients.

The second value of `prev` is a name relative to the base directory of
the feed's URL in `url` (more specifically, in the URL that the client
used to retrieve the feed). In the example above, `prev` would evaluate
to the full URL <https://example.com/twtxt-2021-10-18.txt> for HTTPS and
<gopher://example.com/0/twtxt-2021-10-18.txt> for Gopher.

For all feeds (main and archived), the `url` fields of the main feed
shall be used for [twt hashing](twthashextension.html). (There can be
multiple `url` fields in the main feed, see [the page metadata
extension](metadataextension.html) on how to select the correct one.)

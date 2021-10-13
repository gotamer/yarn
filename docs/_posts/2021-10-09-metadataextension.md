---
layout: page
title: "Metadata Extension"
category: doc
date: 2021-10-09 11:00:00
order: 3
---

In the wild several twtxt users came up with metadata comments in their twtxt
files for different purposes. So the **Metadata** extension is actually not
invented at [twtxt.net](https://twtxt.net/). However, this specification tries
to settle on a common standard as extension to the original [Twtxt File Format
Specification](https://twtxt.readthedocs.io/en/latest/user/twtxtfile.html).

## Purpose

Twtxt feed authors might want to provide additional information for their feeds
or about themselves which clients can pick up and display in a suitable way.

## Format

Whenever a physical line in a twtxt file starts with a hash sign (`#`) it is
considered a comment. The comment ends at the end of the line. Comments are
usually ignored by twtxt clients and can appear anywhere in a feed. Comments
must not be preceded with whitespace.

```
# this is a comment
### # this one, too
```

All metadata are part of comments, so they can be easily ignored by clients
which do not support metdata.

Metadata are simple key value pairs, they consist of a field name and a field
value. Field names and values are separated by an equal sign (`=`). Each field
is on its own physical line. Field names are case insensitive and can contain
any number of ASCII letters, digits, minuses and underscores. Whitespace is not
allowed as part of the field name. Field names must consist of at least one
character.

Field values start after the first equal sign (`=`) if that follows a valid
field name. They are case sensitive and can contain anything, there is no
character restriction other than line breaks. Values end at the end of the
line. Multiline values must use the Unicode line separator `U+2028` just like
multiline twts do.

All whitespace around field names and values must be stripped. There must be no
more than one hash sign (`#`) preceding fields. If fields cannot be parsed,
they must be ignored and treated as regular comments.

```
# field-name = field value
```

It is legal for the same field name to appear more than once. The format allows
this, but it doesn't make sense for all fields. For example, it is reasonable
to have many `follow` fields (one for each feed that a user is following), but
you probably won't find multiple `description` fields. The order of fields with
the same name must be kept so clients can work with them properly.

Valid meta data examples are:

```
# url = https://example.com/twtxt.txt
#nick=joe
#description =This feed tells about my everyday adventures.
```

## Standardized Fields

This section describes common fields and their purposes.

### `url`

This specifies the URL(s) of the feed. There might be several `url` fields in a
single feed. The first `url` field value will be used for [twt
hashing](twthashextension.html).

### `nick`

This is the feed author's nick name. When following feeds clients can suggest
to go with this nick.

For security reasons clients should not automatically update local nicks
without user consent. Otherwise users can be tricked into believing twts are
coming from somebody else they're following. Clients should ask the users when
a nick change is detected.

### `avatar`

This specifies the URL for the author's or feed's avatar, so it can be
displayed along twts, e.g. next to the author. The avatar image is typically in
JPEG, PNG or WebP format. Different clients prefer different ratios, so there
is no strict rule to follow for feed authors. Often avatars are square.

If the `avatar` field is missing, some clients like
[yarnd](https://git.mills.io/yarnsocial/yarn) automatically attempt to fetch
avatars with the basenames `avatar` or `logo` and file extensions `.webp`,
`.png`, `.jpg` and `.jpeg` next or one level up to the feed.

### `description`

The `description` field contains an explanation what the feed is about. Clients
might display this information in a feed details view.

### `follow`

Publicly discloses that the feed author is following another twtxt feed. This
can be helpful to aid feed discoverability. The value contains the nick and the
URL of the feed separated by whitespace:

```
# follow = joe https://example.com/twtxt.txt
```

### `following`

The number of feeds the author is following.

```
# following = 42
```

### `followers`

The number of followers this feed has.

```
# followers = 23
```

### `link`

A link to some other resource which is often connected to the feed or author.
Similar to `follow` fields the syntax for `link` values consists of a link text
followed by whitespace and the actual URL. However, the link text can contain
whitespace.

```
# link = Blog https://example.com/blog/
# link = All my source code https://git.example.com/
```

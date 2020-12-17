# pubsub-push-middleware

Helper for handling [GCP pubsub push messages](https://cloud.google.com/pubsub/docs/push) in Go HTTP servers.

## Why?

This middleware allows you to use your normal HTTP handlers with pubsub push messages.  It does this by "promoting" the data in
the pubsub push message to the HTTP request body.  I use this with both RESTful and [Twirp](https://github.com/twitchtv/twirp) APIs.
This allows me to use the same handlers for pubsub push without changing them.

## Usage

```

import push "github.com/bakins/pubsub-push-middleware"

// assume handler is you normal HTTP handler.
mw := push.New(handler)

http.ListenAndServe(":8080", mw)
```


## LICENSE

See [LICENSE](./LICENSE)
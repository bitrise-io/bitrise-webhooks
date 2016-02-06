# bitrise-webhooks

Bitrise Webhooks processor.

Transforms various webhooks (GitHub, Bitbucket, Slack, ...) to [bitrise.io](https://www.bitrise.io)'s
Build Trigger API format, and calls it to start a build.


## Supported webhooks / providers

* GitHub - WIP
* Bitbucket V2 (aka Webhooks) - WIP


## Development: How to start the server

* Install [Go](https://golang.org), and [set up your Workspace](https://golang.org/doc/code.html#Workspaces) and your [$GOPATH](https://golang.org/doc/code.html#GOPATH)
* If you want to change things, Fork this project
* `git clone` the project into your GOPATH: `git clone your-fork.url $GOPATH/src/github.com/bitrise-io/bitrise-webhooks`
* `cd $GOPATH/src/github.com/bitrise-io/bitrise-webhooks`

Start the server:

* Compile the `Go` code with `godep go install`
  * If you don't have [godep](https://github.com/tools/godep) yet, you can get it with: `go get github.com/tools/godep`
* Run it with: `bitrise-webhooks -port=4000`

Alternatively, with [bitrise CLI](https://github.com/bitrise-io/bitrise):

* `bitrise run start`
  * This will start the server with [gin](https://github.com/codegangsta/gin), which does automatic re-compilation when the code changes, so you don't have to compile & restart the server manually after every code change.

### Development mode:

By default the server will be started in Development Mode. This means that
it **won't** send requests, it'll only print the request in the logs.

You can pass the `-send-request-to` flag, or set the `SEND_REQUEST_TO` environment
variable to enable sending the requests to the specified URL (every request
will be posted to the exact URL you specify).

You can switch the server into Production mode by defining the
environment variable: `RACK_ENV=production`. In production mode
the server **will send requests to [bitrise.io](https://www.bitrise.io)**,
unless you specify a send-request-to parameter.


### How to use it / test it

* Register a webhook at any supported provided, pointing to your `bitrise-webhooks` server
  * The format should be: `http(s)://YOUR-BITRISE-WEBHOOKS.DOMAIN/hook/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`
    * *Keep in mind that most of the providers only support SSL (HTTPS) URLs by default. If you want to use an HTTP URL you might have to set additional parameters when you register your webhook.*


## Development

### Testing a (new) webhook format

You can use [http://requestb.in](http://requestb.in) to debug/check
a service's webhook format.

*If you have a Heroku account you can quickly create & start your
own RequestBin server for free, just follow the guide on
[RequestBin's GitHub page](https://github.com/Runscope/requestbin).*

Just create a RequestBin, and register the provided URL as
the Webhook URL on the service you want to test. Once the service
triggers a webhook you'll see the webhook data on RequestBin.

You can pass the `-send-request-to` option (or `SEND_REQUEST_TO` env var) to this server to
send all the request to the specified URL (e.g. to RequestBin), instead of sending
it to [bitrise.io](https://www.bitrise.io).


## Deploy to Heroku

* `git clone` the code - either the official code or your own fork
* `cd` into the source code directory
* `heroku create`
* Optionally, once it's created:
  * To debug: set a specific URL, where every request will be made by the server: `heroku config:set SEND_REQUEST_TO=http://request-bin-or-similar-service.com/abc123`
  * To send requests to [bitrise.io](https://www.bitrise.io): set the server into Production Mode: `heroku config:set RACK_ENV=production`
    * Make sure the `SEND_REQUEST_TO` config is no longer set. While `SEND_REQUEST_TO` is set every request will be sent to the specified URL, it doesn't matter whether you're in Development or Production mode. You can simply set an empty value for `SEND_REQUEST_TO` to disable it: `heroku config:set SEND_REQUEST_TO=""`
* `git push heroku master`

Done. Your Bitrise Webhooks server is now running on Heroku.
You can open it with `heroku open` - opening the root URL of the server
should present a JSON data, including the server's `version`,
the current `time`, the server's `environment_mode` and a welcome `message`.


## TODO

* only send request to bitrise.io if serverEnvironmentMode==production
  * in every other case just log the request, but don't send it
* Provider Support: Bitbucket V1 (aka Services)
* Provider Support: Visual Studio Online
* Provider Support: GitLab
* Provider Support: Slack

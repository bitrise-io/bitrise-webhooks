# bitrise-webhooks

Bitrise Webhooks processor.

Transforms various webhooks (GitHub, Bitbucket, Slack, ...) to [bitrise.io](https://www.bitrise.io)'s
Build Trigger API format, and calls it to start a build.

**Feel free to add your own webhook transform provider to this project!**
For more information check the *How to add support for a new Provider* section.


## Supported webhooks / providers

* GitHub
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


## How to add support for a new Provider

Implement your support code into `./providers/theprovider`.
Unit tests are **required**.

Once the implementation is ready add it to the `selectProvider` function,
to the `supportedProviders` list, in the `endpoint_hook.go` file.

For an example you should check the `providers/github` provider implementation.

### Notes, tips & tricks

* You can use [http://requestb.in](http://requestb.in) to debug the webhook format
  of the (new) service.
* Once you have an example webhook of the service you can create the provider
  in the `./providers` folder.
* You should now declare the data model of the webhook. You don't have to
  specify the whole webhook format, just the pieces which you'll use.
  You can check the `./providers/github/github.go` file for an example.
* Once you have at least the base data model you should start writing your
  unit tests.
  * Probably the best is to start with the `HookCheck` method, which doesn't
    do any transformation, just determines whether the provider should be used
    for processing the webhook or not.
  * Now you should write a minimal `Transform`, but because this method
    requires a full http request object it's probably better to keep it minimal
    first, and instead write your internal transformer functions which work
    with the parsed data, instead of the raw http request object.
    The GitHub provider has two separate internal transformer functions,
    `transformCodePushEvent` and `transformPullRequestEvent`.
  * You should define your tests and make your code pass the tests.
    The final touch is to define a sample, raw request data,
    and write a proper test for the `Transform` function.
* Once the implementation is ready add it to the `supportedProviders` list,
  (`endpoint_hook.go` file, `selectProvider` function), and implement the related
  Unit Test to make sure that your provider will actually be selected.
  You don't have to change any other code, adding your provider to
  the `supportedProviders` list is all what's required.
* Done! You can now test your provider on a server (check the *Deployment* section),
  and you can create a Pull Request, to have it merged with the official
  [bitrise.io](https://www.bitrise.io) webhook processor.


## TODO

* Provider Support: Bitbucket V1 (aka Services)
* Provider Support: Visual Studio Online
* Provider Support: GitLab
* Provider Support: Slack

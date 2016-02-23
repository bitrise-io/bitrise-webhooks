# bitrise-webhooks

Bitrise Webhooks processor.

Transforms various webhooks (GitHub, Bitbucket, Slack, ...) to [bitrise.io](https://www.bitrise.io)'s
[Build Trigger API format](http://devcenter.bitrise.io/docs/build-trigger-api),
and calls it to start a build.

**Feel free to add your own webhook transform provider to this project!**
For more information check the *How to add support for a new Provider* section.


## Supported webhooks / providers

* [GitHub](https://github.com)
  * handled on the path: `/h/github/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`
* [Bitbucket](https://bitbucket.org) webhooks V2 ("Webhooks" on the Bitbucket web UI)
  * handled on the path: `/h/bitbucket-v2/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`
* [Slack](https://slack.com) (both outgoing webhooks & slash commands)
  * handled on the path: `/h/slack/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`
* [Visual Studio Team Services](https://www.visualstudio.com/products/visual-studio-team-services-vs)
  * handled on the path: `/h/visualstudio/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`

Work in progress:

* [GitLab](https://gitlab.com)


### GitHub - setup & usage:

All you have to do is register your `bitrise-webhooks` URL for
a [GitHub](https://github.com) *repository*.

1. Open your *repository* on [GitHub.com](https://github.com)
2. Go to `Settings` of the *repository*
3. Select `Webhooks & services`
4. Click on `Add webhook`
5. Specify the `bitrise-webhooks` URL (`.../h/github/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`) in the `Payload URL` field
6. Select the *events* you want to trigger a webhook for
  * Right now `bitrise-webhooks` supports the `Push` and `Pull Request` events,
    every other webhook (triggered by another event) will be ignored.
7. Click `Add webhook`

That's all, the next time you push code or create a pull request (if you enabled the related event(s))
a build will be triggered.


### Bitbucket (V2) Webhooks - setup & usage:

All you have to do is register your `bitrise-webhooks` URL for
a [Bitbucket](https://bitbucket.org) *repository*.

1. Open your *repository* on [Bitbucket.org](https://bitbucket.org)
2. Go to `Settings` of the *repository*
3. Select `Webhooks`
4. Click on `Add webhook`
5. Specify the `bitrise-webhooks` URL (`.../h/bitbucket-v2/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`) in the `URL` field
6. In the *Triggers* section select `Repository push`
  * Right now `bitrise-webhooks` only supports the *Repository push* trigger for
    Bitbucket Webhooks.
7. Click `Save`

That's all, the next time you push code (into your repository) a build will be triggered.


### Visual Studio Online / Visual Studio Team Services - setup & usage:

All you have to do is register your `bitrise-webhooks` URL for
a [visualstudio.com](https://visualstudio.com) *project* as a `Service Hooks` integration.

You can find an official guide
on [visualstudio.com 's documentations site](https://www.visualstudio.com/en-us/get-started/integrate/service-hooks/webhooks-and-vso-vs).

A short step-by-step guide:

1. Open your *project* on [visualstudio.com](https://visualstudio.com)
2. Go to the *Admin/Control panel* of the *project*
3. Select `Service Hooks`
4. Create a service integration
  * In the Service list select the `Web Hooks` option
  * Select the `Code pushed` event as the *Trigger*
  * In the `Filters` section select the `Repository` you want to integrate
  * You can leave the other filters on default
  * Click `Next`
  * On the `Action` setup form specify the `bitrise-webhooks` URL (`.../h/visualstudio/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`) in the `URL` field
  * You can leave every other option on default
7. Click `Finish`

That's all, the next time you push code (into your repository) a build will be triggered.


### Slack - setup & usage:

You can register the `bitrise-webhooks` URL (`.../h/slack/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`) as either
an [Outgoing Webhook](https://my.slack.com/services/new/outgoing-webhook) or
as a [slash command](https://my.slack.com/services/new/slash-commands) for your Slack team.

Once the URL is registered check the *usage* section below for all the
accepted and required parameters you can define in the message, and
for a couple of examples.

#### Usage - the message format

Your message have to be in the format: `key:value|key:value|...`,
where the supported `keys` are:

At least one of these two parameters are required:

* `b` or `branch` - example: `branch: master`
* `w` or `workflow` - example: `workflow: primary`

Other, optional parameters:

* `t` or `tag` - example: `branch: master|tag: v1.0`
* `c` or `commit` - example: `workflow: primary|commit: eee55509f16e7715bdb43308bb55e8736da4e21e`
* `m` or `message` - example: `branch: master|message: ship it!!`

**NOTE**: at least either `branch` or `workflow` have to be specified, and of course
you can specify both if you want to. You're free to specify any number of optional parameters.

An example with all parameters included: `workflow: primary|b: master|tag: v1.0|commit:eee55509f16e7715bdb43308bb55e8736da4e21e|m: start my build!`


## How to compile & run the server

* Install [Go](https://golang.org), and [set up your Workspace](https://golang.org/doc/code.html#Workspaces) and your [$GOPATH](https://golang.org/doc/code.html#GOPATH)
  * Go `1.6` or newer required!
* If you want to change things, Fork this repository
* `git clone` the project into your GOPATH: `git clone your-fork.url $GOPATH/src/github.com/bitrise-io/bitrise-webhooks`
* `cd $GOPATH/src/github.com/bitrise-io/bitrise-webhooks`

Start the server:

* Compile the `Go` code with `godep go install`
  * If you don't have [godep](https://github.com/tools/godep) yet, you can get it with: `go get github.com/tools/godep`
* Run it with: `bitrise-webhooks -port=4000`

Alternatively, with [bitrise CLI](https://www.bitrise.io/cli):

* `bitrise run start`
  * This will start the server with [gin](https://github.com/codegangsta/gin),
    which does automatic re-compilation when the code changes,
    so you don't have to compile & restart the server manually after every code change.

### Development mode:

By default the server will be started in Development Mode. This means that
it **won't** send requests, it'll only print the request in the logs.

You can pass the `-send-request-to` flag, or set the `SEND_REQUEST_TO` environment
variable to enable sending the requests to the specified URL (every request
will be posted to the exact URL you specify).

You can switch the server into Production mode by defining the
environment variable: `RACK_ENV=production`. In production mode
the server **will send requests to [bitrise.io](https://www.bitrise.io)**,
unless you specify a *send-request-to* parameter.


### How to use it / test it

* Register a webhook at any supported provided, pointing to your `bitrise-webhooks` server
  * The format should be: `http(s)://YOUR-BITRISE-WEBHOOKS.DOMAIN/h/SERVICE/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`
    * *Keep in mind that most of the providers only support SSL (HTTPS) URLs by default. If you want to use an HTTP URL you might have to set additional parameters when you register your webhook.*


## Development

### Testing a (new) webhook format

You can use [http://requestb.in](http://requestb.in) to debug/check
a service's webhook format.

*If you have a Heroku account you can quickly create & start your
own RequestBin server for free, just follow the guide on
[RequestBin's GitHub page](https://github.com/Runscope/requestbin).*

Create a RequestBin, and register the provided URL as
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
  * To debug: set a specific URL, where every request will be sent by the server:
    * `heroku config:set SEND_REQUEST_TO=http://request-bin-or-similar-service.com/abc123`
  * To send requests to [bitrise.io](https://www.bitrise.io):
    * Switch the server into Production Mode: `heroku config:set RACK_ENV=production`
    * Make sure the `SEND_REQUEST_TO` config is no longer set. While `SEND_REQUEST_TO` is set
      every request will be sent to the specified URL, it doesn't matter whether you're in Development
      or Production mode.
      You can simply set an empty value for `SEND_REQUEST_TO` to disable it: `heroku config:set SEND_REQUEST_TO=""`
* `git push heroku master`
* `heroku ps:scale web=1`

Done. Your Bitrise Webhooks server is now running on Heroku.

You can open it with `heroku open` - opening the root URL of the server
should present a JSON data, including the server's `version`,
the current `time`, the server's `environment_mode` and a welcome `message`.


## How to add support for a new Provider

Implement your webhook provider support code into `./service/hook/theprovider`.
**Unit tests are required** if you want your code to be merged into the
main `bitrise-wekhooks` repository!

Once the implementation is ready add it to the `selectProvider` function,
to the `supportedProviders` list, in the `service/hook/endpoint.go` file.

For an example you should check the `service/hook/github` (single webhook triggers
only one build)
or the `service/hook/bitbucketv2` (a single webhook might trigger multiple builds)
provider implementation.

### Guide

* You should check `service/hook/github` for an example if a single webhook
  can be transformed into a single build trigger, or `service/hook/bitbucketv2`
  if a single webhook might trigger multiple builds
* You can use [http://requestb.in](http://requestb.in) to debug the webhook format
  of the service.
  * To format & browse JSON responses you can use [http://www.jsoneditoronline.org/](http://www.jsoneditoronline.org/)
    or a similar tool - it helps a lot in debugging & cleaning up JSON webhooks.
* Create a folder in `service/hook`, following the naming pattern of existing providers (and Go package naming conventions)
  * Use only lowercase ASCII letters & numbers, without any whitespace, dash, underscore, ... characters
* Create a test file (`..._test.go`)
* **Note:** you should create a testing function for every function you add,
  before you'd write any code for the function!
* Split the logic into functions; usually this split should be fine (at least for starting):
  * Validate the required headers (content type, event ID if supported, etc.)
  * Declare your **data model(s)** for the Webhook data
  * Create a test for the `TransformRequest` method, with checks for the required inputs (headers, event type, etc.).
    * You can test it with a sample webhook request string right away, but it's probably easier to write the
      transform utility function(s) first
  * Create your transform utility function(s):
    * These function(s) should get your declared Webhook data models as it's input (not the full raw request body)
      to make it easier to test/validate
    * You can check the `transformCodePushEvent` function in the `service/hook/github` service as for an example
  * Write tests for these transform utility function(s)
    * It's usually a good idea to just write a list of error tests first, directly in the `_test.go` file,
      without an actual test implementation.
    * Then start to write the test implementations, one by one; write the test first, then
      make the code pass, then go to the next test implementation
  * Once your transform functions are well tested you should get back to the `TransformRequest` function,
    test & implement that too
    * You should include a sample webhook data & test, as you can see it in the `github` and `bitbucketv2` services.
* Once the implementation is ready you can register a path/route for the service/provider:
  * Open `service/hook/endpoint.go`
  * Add your provider to the `supportedProviders` map
    * the **key** will be the URL (PROVIDER-ID component in the URL) this provider is registered for; URL format will be: `/h/PROVIDER-ID/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`
    * the **value** is an object of your provider
* At this point you can start the server and your provider should handle the calls as expected
  * You can run the `bitrise-webhooks` executable on your server
* You should also send a Pull Request, so your provider will be available for others

Done! You can now test your provider on a server (check the *Deployment* section),
and you can create a Pull Request, to have it merged with the official
[bitrise.io](https://www.bitrise.io) webhook processor.


#### Optional: define response Transform functions

Once you have a working Provider you can optionally define response
transformers too. With this you can define the exact response JSON data,
and the HTTP status code, both for success and for error responses.

If you don't define a response Transformer the default response provider
will be used (`service/hook/default_reponse_provider.go`).

To define your own Response Provider/Transform functions you just have to
implement the functions of `ResponseTransformer` (`service/hook/common/common.go`).

If your Provider implements these functions it'll be used for generating
the response. You have to implement every function defined in the
interface, or your Provider won't be considered as an implementation of the interface
and the default response provider will be used instead.


## Response

Response is always in JSON format.

**If provider declares the response transformers** it'll be used, and the
provider is responsible for generating the response JSON.

If it doesn't provide the response transformer functions then the default
response provider will be used.

**The default response provider** generates the following responses:

* If an error prevents any build calls then a single `{"error": "..."}` response
  will be generated (with HTTP code `400`).
* If a single success message is generated (e.g. if the hook is skipped and it's
  declared as a success, instead of an error) then a `{"message": "..."}` response
  will be generated (with HTTP status code `200`).
* If at least one Bitrise Trigger call was initiated:
  * All the received responses will be included as a `"success_responses": []`
    and `"failed_responses": []` JSON arrays
  * And all the errors (where the response was not available / call timed out, etc.)
    as a `"errors": []` JSON array (if any)
  * If at least one call fails or the response is an error response
    the HTTP status code will be `400`
  * If all trigger calls succeed the status code will be `201`


## TODO

* Re-try handling
* Bitbucket V1 (aka "Services" on the Bitbucket web UI) - not sure whether we should support this,
  it'll be deprecated in the future, and we already support the newer, V2 webhooks.

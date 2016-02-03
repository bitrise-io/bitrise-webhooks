# bitrise-webhooks

Bitrise Webhooks processor.

Transforms various webhooks (GitHub, Bitbucket, ...) to [bitrise.io](https://www.bitrise.io)'s
Build Trigger API format, and calls it to start a build.


## Supported webhooks / providers

* GitHub - WIP
* Bitbucket V2 (aka Webhooks) - WIP


## How to start the server

* Compile the `Go` code with `godep go install`
* Run it with: `bitrise-webhooks -port=3000`

Alternatively, with [bitrise CLI](https://github.com/bitrise-io/bitrise):

* `bitrise run start`

### How to use it / test it

* Register a webhook at any supported provided, pointing to your `bitrise-webhooks` server
  * The format should be: `http(s)://YOUR-BITRISE-WEBHOOKS.DOMAIN/hook/BITRISE-APP-SLUG/BITRISE-APP-API-TOKEN`
  * *Keep in mind that most of the providers only support SSL (HTTPS) URLs by default. If you want to use an HTTP URL you might have to set additional parameters when you register your webhook.*


## Development

### Testing a (new) webhook format

You can use [http://requestb.in](http://requestb.in) to debug/check
a service's webhook format.

Just create a RequestBin, and register the provided URL as
the Webhook URL on the service you want to test. Once the service
triggers a webhook you'll see the webhook data on RequestBin.

You can pass the `-send-request-to` option to this server to
send all the request to the specified URL (e.g. to RequestBin), instead of sending
it to [bitrise.io](https://www.bitrise.io).


## TODO

* only send request to bitrise.io if serverEnvironmentMode==production
  * in every other case just log the request, but don't send it
* Provider Support: Bitbucket V1 (aka Services)
* Provider Support: Visual Studio Online
* Provider Support: GitLab
* Provider Support: Slack

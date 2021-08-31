# potential-disco

This is a (very janky) prototype app for my OpenTelemetry exploration cobbled together from a few different places, most notably:

* [AWS Distro for OpenTelemetry Go SDK guide](https://aws-otel.github.io/docs/getting-started/go-sdk)
* [Ricardo Ferreira's OpenTelemetry for Dummies example code](https://github.com/riferrei/otel-with-golang)

It is ready for integration into AWS using the [`aws-otel-collector`](https://github.com/aws-observability/aws-otel-collector) sidecar, which will send the metrics and traces built in here to Cloudwatch and X-Ray respectively. A sample task def is included for reference, but you'll still need to make sure the appropriate permissions are attached to the task role in order to make it work (see the AWS guide above for more information).

FROM golang:1.17.0

ADD . /usr/src/app
WORKDIR /usr/src/app

RUN go build -o otel-test-app
ENTRYPOINT [ "/usr/src/app/otel-test-app" ]

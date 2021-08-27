FROM golang:1.17.0

ADD . /usr/src/app
WORKDIR /usr/src/app

RUN go build -o mtb-app
ENTRYPOINT [ "/usr/src/app/mtb-app" ]

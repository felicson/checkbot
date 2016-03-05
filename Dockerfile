FROM google/golang:latest

WORKDIR /gopath/src/app
ADD . /gopath/src/app/
#RUN go get app
VOLUME /gopath/src/app

CMD []

ENTRYPOINT ["/gopath/bin/app"]


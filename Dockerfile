FROM golang:1.8

WORKDIR /go/src/github.com/rgamba/postman

RUN go get github.com/jteeuwen/go-bindata/...

ADD . .
ADD config.sample.toml /config.toml

RUN make build
RUN make install

VOLUME [ "/log" ]

EXPOSE 8130 18130

ENTRYPOINT [ "/go/bin/postman", "-config", "/config.toml", "-v=3" ]
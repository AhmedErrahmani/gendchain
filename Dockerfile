# Build GendChain in a stock Go builder container
FROM golang:1.17-alpine3.13 as builder

RUN apk --no-cache add build-base git mercurial gcc linux-headers
ENV D=/gendchain
WORKDIR $D
# cache dependencies
ADD go.mod $D
ADD go.sum $D
RUN go mod download
# build
ADD . $D
RUN cd $D && make all && mkdir -p /tmp/gendchain && cp $D/bin/* /tmp/gendchain/

# Pull all binaries into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /tmp/gendchain/* /usr/local/bin/
EXPOSE 6060 8545 8546 30303 30303/udp 30304/udp
CMD [ "gendchain" ]

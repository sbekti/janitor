FROM golang:alpine AS build-env
RUN apk add --update tzdata bash wget curl git
RUN mkdir -p $$GOPATH/bin && \
    curl https://glide.sh/get | sh
ADD . /go/src/janitor
WORKDIR /go/src/janitor
RUN glide update && go build -o main

FROM alpine
WORKDIR /app
COPY --from=build-env /go/src/janitor/main /app/
ENTRYPOINT ["./main"]
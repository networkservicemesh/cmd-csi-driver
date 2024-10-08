FROM golang:1.23.1 as go
ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOBIN=/bin
RUN go install github.com/go-delve/delve/cmd/dlv@v1.20.2

FROM go as build
WORKDIR /build
COPY go.mod go.sum ./
COPY ./local ./local
COPY ./internal/imports imports
RUN go build ./imports
COPY . .
RUN go build -o /bin/app .

FROM build as test
CMD go test -test.v ./...

FROM test as debug
CMD dlv -l :40000 --headless=true --api-version=2 test -test.v ./...

FROM alpine as runtime
COPY --from=build /bin/app /bin/app
ENTRYPOINT ["/bin/app"]

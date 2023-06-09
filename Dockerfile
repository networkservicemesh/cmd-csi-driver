# Build the NSM CSI Driver binary
FROM golang:1.20.1-alpine AS builder
ARG GIT_TAG
ARG GIT_COMMIT
ARG GIT_DIRTY
RUN apk add make
WORKDIR /code
COPY go.mod /code/go.mod
COPY go.sum /code/go.sum
RUN go mod download
ADD . /code
#RUN CGO_ENABLED=0 make test
RUN CGO_ENABLED=0 make GIT_TAG="${GIT_TAG}" GIT_COMMIT="${GIT_COMMIT}" GIT_DIRTY="${GIT_DIRTY}" build

# Build a scratch image with just the NSM CSI driver binary
FROM scratch AS cmd-csi-driver
COPY --from=builder /code/bin/nsm-csi-driver /nsm-csi-driver
WORKDIR /
ENTRYPOINT ["/nsm-csi-driver"]
CMD []

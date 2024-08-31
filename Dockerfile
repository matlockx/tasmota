FROM --platform=$BUILDPLATFORM golang:1.23.0-bookworm as builder
WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -v -o /out/binary ./...

FROM arm64v8/alpine:latest
COPY --from=builder /out/binary /tasmota
CMD  ["/tasmota"]

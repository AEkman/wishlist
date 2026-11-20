FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine3.24 AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -o /out/wishlist ./cmd/wishlist

FROM alpine:3.24.4

RUN addgroup -g 10001 wishlist \
    && adduser -D -H -u 10001 -G wishlist wishlist \
    && mkdir -m 0750 /data \
    && chown wishlist:wishlist /data
COPY --from=build /out/wishlist /usr/local/bin/wishlist

WORKDIR /data
USER wishlist
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/wishlist"]

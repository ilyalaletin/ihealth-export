FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/ihealth-server ./cmd/ihealth-server

FROM alpine:3.23
RUN addgroup -S ihealth && adduser -S -G ihealth ihealth && mkdir /data && chown ihealth:ihealth /data
COPY --from=build /out/ihealth-server /usr/local/bin/ihealth-server
USER ihealth
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["ihealth-server"]

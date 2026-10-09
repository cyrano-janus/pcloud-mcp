FROM golang:1.26.9-bookworm@sha256:d9c68c2c51161e12fd77e4c6320687c9cd86e1af1e3ad6e6cd63ff970641453c AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/pcloud-mcp ./cmd/pcloud-mcp

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/pcloud-mcp /pcloud-mcp
USER 65532:65532
EXPOSE 8443
ENTRYPOINT ["/pcloud-mcp"]

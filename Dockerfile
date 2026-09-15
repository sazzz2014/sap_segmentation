FROM golang:1.25.4-bookworm AS source
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM source AS quality
RUN go vet ./... && go test -race ./...

FROM quality AS build
RUN CGO_ENABLED=0 go build -trimpath -o /out/importer ./cmd/sap_segmentationd && \
    CGO_ENABLED=0 go build -trimpath -o /out/mock-erp ./cmd/mock_erp && \
    CGO_ENABLED=0 go build -trimpath -o /out/e2e-verify ./cmd/e2e_verify

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && mkdir /log && chown 10001:10001 /log
USER 10001:10001
WORKDIR /app

FROM runtime AS importer
COPY --from=build /out/importer /app/importer
ENTRYPOINT ["/app/importer"]

FROM runtime AS mock
COPY --from=build /out/mock-erp /app/mock-erp
ENTRYPOINT ["/app/mock-erp"]

FROM runtime AS verifier
COPY --from=build /out/e2e-verify /app/e2e-verify
ENTRYPOINT ["/app/e2e-verify"]

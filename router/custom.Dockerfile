FROM segment/chamber:2.12.0 AS chamber
FROM golang:1.23 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG COMMIT
ARG DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

ARG VERSION=dev
ENV VERSION=$VERSION

ENV COMMIT=$COMMIT

ENV DATE=$DATE

WORKDIR /app/

# Copy only the files required for go mod download
COPY ./go.* .

# Download dependencies
RUN go mod download

# Copy the rest of the files
COPY . .

# Run tests
RUN go test -v ./...

# Build router
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-extldflags=-static \
    -X 'github.com/wundergraph/cosmo/router/core.Version=${VERSION}' \
    # -X 'github.com/wundergraph/cosmo/router/core.Commit=${COMMIT}' \
    -X 'github.com/wundergraph/cosmo/router/core.Date=${DATE}'" \
    -a -o router cmd/cbs-sports/main.go

FROM gcr.io/distroless/static:latest

COPY --from=chamber /chamber /bin/chamber
COPY --from=builder /app/router /router

ENTRYPOINT ["/router"]

EXPOSE 3002

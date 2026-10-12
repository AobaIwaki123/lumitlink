# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
    -trimpath -ldflags="-w -s" \
    -o /out/lumipage ./cmd/lumipage

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/lumipage /lumipage

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/lumipage"]

FROM golang:1.24 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o folio .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /build/folio /folio
EXPOSE 8080
ENTRYPOINT ["/folio"]

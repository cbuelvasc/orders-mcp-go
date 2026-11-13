FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/orders-mcp ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/orders-mcp /orders-mcp
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/orders-mcp"]

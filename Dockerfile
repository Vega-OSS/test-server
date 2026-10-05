FROM golang:alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/testserver ./cmd/testserver

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/testserver /testserver
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/testserver"]
CMD ["--port=:8081", "--vega-grpc=vegadb:50051", "--vega-http=http://vegadb:8080"]

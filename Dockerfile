# syntax=docker/dockerfile:1.6

# ---------- build stage ----------
FROM golang:1.24-alpine AS build

WORKDIR /src

# Cache module dependencies first.
COPY go.mod go.sum* ./
RUN go mod download

# Copy the rest of the source and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/app ./

# ---------- runtime stage ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/app /app/app

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/app"]

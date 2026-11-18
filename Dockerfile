# --- build ---
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /payroll ./cmd

# --- runtime ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /payroll /payroll
EXPOSE 8080
ENTRYPOINT ["/payroll"]

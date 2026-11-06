# One image, two binaries (api + worker); pick with `command`.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY services/game/go.mod services/game/go.sum ./
RUN go mod download
COPY services/game/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ /app/
COPY content/ /app/content/
USER nonroot
EXPOSE 8080

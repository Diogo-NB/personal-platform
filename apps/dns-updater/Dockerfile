# syntax=docker/dockerfile:1

FROM golang:1.27.1-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -tags lambda.norpc -trimpath -ldflags="-s -w" \
    -o /out/bootstrap ./cmd/dns-updater

FROM public.ecr.aws/lambda/provided:al2023

COPY --from=build /out/bootstrap /var/runtime/bootstrap

ENTRYPOINT ["/var/runtime/bootstrap"]

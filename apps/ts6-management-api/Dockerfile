FROM public.ecr.aws/awsguru/aws-lambda-adapter:1.0.1 AS lambda-adapter
FROM golang:1.27.1-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath -ldflags="-s -w" -o /out/bootstrap ./cmd/ts6-management-api

FROM public.ecr.aws/lambda/provided:al2023
COPY --from=lambda-adapter /lambda-adapter /opt/extensions/lambda-adapter
COPY --from=build /out/bootstrap /var/task/bootstrap
ENV AWS_LWA_PORT=8080 \
    AWS_LWA_PASS_THROUGH_PATH=/internal/events \
    AWS_LWA_READINESS_CHECK_PATH=/internal/readiness \
    AWS_LWA_READINESS_CHECK_HEALTHY_STATUS=200-299
EXPOSE 8080
ENTRYPOINT ["/var/task/bootstrap"]

FROM node:22.23.1-bookworm-slim AS web
WORKDIR /src
COPY package.json yarn.lock ./
COPY sdk/ts/ sdk/ts/
COPY third_party/ third_party/
COPY web/ web/
RUN yarn install --frozen-lockfile --non-interactive && yarn build

FROM golang:1.26.7-bookworm AS go
WORKDIR /src
ENV GOWORK=off CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/
RUN go build -trimpath -o /courier ./cmd/courier

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go /courier /app/courier
COPY --from=web /src/web/dist /app/web/dist
EXPOSE 8080
ENTRYPOINT ["/app/courier"]

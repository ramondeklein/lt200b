ARG GO_VERSION=1.27.1
FROM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/lt200b .

FROM scratch

COPY --from=build /out/lt200b /lt200b

ENV LT200B_ADDRESS=
ENV LT200B_PASSWORD=
ENV LT200B_LISTEN=:8080

EXPOSE 8080

ENTRYPOINT ["/lt200b"]

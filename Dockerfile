FROM golang:1.27.2-alpine AS build
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w' -o /out/nhl-stats-vim ./cmd/nhl-stats-vim

FROM scratch
LABEL org.opencontainers.image.source=https://github.com/Malvore/nhl-stats-vim
COPY LICENSE /LICENSE
COPY THIRD_PARTY_NOTICES.md /THIRD_PARTY_NOTICES.md
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /usr/share/zoneinfo/America/New_York /usr/share/zoneinfo/America/New_York
COPY --from=build /out/nhl-stats-vim /nhl-stats-vim
USER 65532:65532
EXPOSE 2222
ENTRYPOINT ["/nhl-stats-vim"]

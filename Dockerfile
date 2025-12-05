FROM --platform=$BUILDPLATFORM golang:alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY . .
RUN go tool task build-ci

FROM scratch
COPY --from=build /out/shimdns /shimdns
ENTRYPOINT ["/shimdns"]
CMD  [ "-c", "/config/config.yaml" ]
# builder image
FROM golang:alpine as builder
ARG TARGETPLATFORM
COPY . /src
WORKDIR /src
RUN apk add --no-cache make bash git
RUN make app-build PLATFORMS=$TARGETPLATFORM

# running image
FROM alpine
WORKDIR /home
COPY --from=builder /src/bin/vidai-* /bin/vidai

# executable
ENTRYPOINT [ "/bin/vidai" ]

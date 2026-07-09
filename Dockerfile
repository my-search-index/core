# choose the build time image
FROM golang:1.24-alpine AS builder
# create and set the current working directory inside the image
WORKDIR /app
# copy everything from the current directory to the working directory inside the image
COPY . .
# download the dependencies
RUN go mod download
# build the Go app 
RUN CGO_ENABLED=0 GOOS=linux go build -o core ./cmd/core

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/core .

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["./core"]

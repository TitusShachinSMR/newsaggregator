FROM golang:1.23-alpine
WORKDIR /app
COPY . .
RUN go env -w GOPROXY=https://proxy.golang.org,direct
RUN go env -w GOSUMDB=sum.golang.org
RUN go build -o nlp ./
CMD ["/app/nlp"]
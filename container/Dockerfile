FROM docker.io/golang:1.20-alpine
RUN apk add git
WORKDIR /app
RUN git clone https://github.com/itsByte/MarkovBotGo src
WORKDIR /app/src
RUN go build -o ../
CMD [ "/app/MarkovBotGo" ]
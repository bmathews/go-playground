# pull in golang base
FROM golang:latest

# Copy all contents from host machine to go/src/app
ADD . /go/src/app

# cd to that dir
WORKDIR /go/src/app

# install go deps
RUN go get -v -d .
RUN go install -v .

# build go project
RUN go build -o main .

# Set run command
CMD ["/go/src/app/main", "--redisServer", "echo ${REDIS_SERVER}"]

# expose port
EXPOSE 8080

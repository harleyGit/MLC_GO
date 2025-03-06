FROM golang:latest 

ENV GOPROXY https://goproxy.cn,direct
WORKDIR $GOPATH/src/MLC_GO
COPY . $GOPATH/src/MLC_GO
RUN go build .

EXPOSE 8000
ENTRYPOINT [ "./MLC_GO" ]
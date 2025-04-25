clean:
  rm -f dvr

build: clean
  go build -v -ldflags="-s -w" -o dvr ./ 

build_linux: clean
  GOOS=linux GOARCH=386 go build -v -ldflags="-s -w" -o dvr_linux ./ && mv dvr_linux /Volumes/apps_storage/dvr/.

dbuild:
  docker build -t dvr . && \
  docker save dvr | pv | ssh nas docker load

run:
  go run *.go

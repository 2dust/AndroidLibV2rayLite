#!/bin/bash

red="\e[0;31m"
green="\e[0;32m"
reset="\033[0m"

get_modules()
{
	printf "${green}Start installing go modules ${reset}\n\n"
	GO111MODULE=on go mod download
	printf "${green}Done ${reset}\n\n"
}

install_gomobile()
{
	printf "${green}Installing gomobile${reset}\n\n"
	
	go get golang.org/x/mobile/cmd/gomobile@latest
	go get golang.org/x/mobile/cmd/gobind@latest
	
	go build golang.org/x/mobile/cmd/gomobile
	go build golang.org/x/mobile/cmd/gobind
	PATH=$(pwd):$PATH
	printf "${green}Done ${reset}\n\n"
}




install_gomobile
get_modules

go get -u github.com/golang/protobuf/protoc-gen-go
mkdir -p assets data
bash gen_assets.sh download
cp -v data/*.dat assets/
go get -v golang.org/x/mobile/cmd/...
mkdir -p $(go env GOPATH)/src/v2ray.com/core
git clone https://github.com/v2fly/v2ray-core.git $(go env GOPATH)/src/v2ray.com/core
go get -u github.com/v2fly/v2ray-core/v5
go get -d github.com/2dust/AndroidLibV2rayLite
gomobile init
gomobile bind -androidapi 19 -v -ldflags='-s -w' github.com/2dust/AndroidLibV2rayLite

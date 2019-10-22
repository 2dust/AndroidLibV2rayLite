pb:
	  go get -u github.com/golang/protobuf/protoc-gen-go
		@echo "pb Start"
asset:
	bash gen_assets.sh download
	mkdir assets
	cp -v data/*.dat assets/
	cd assets;curl https://raw.githubusercontent.com/2dust/AndroidLibV2rayLite/master/data/geosite.dat > geosite.dat		
	cd assets;curl https://raw.githubusercontent.com/2dust/AndroidLibV2rayLite/master/data/geoip.dat > geoip.dat

fetchDep:
	go get -v golang.org/x/mobile/cmd/...
	go get -v go.starlark.net/starlark
	go get -v github.com/refraction-networking/utls
	go get -v github.com/gorilla/websocket
	go get -v -insecure v2ray.com/core
	-go get  github.com/2dust/AndroidLibV2rayLite
	go get github.com/2dust/AndroidLibV2rayLite

ANDROID_HOME=$(HOME)/android-sdk-linux
export ANDROID_HOME
PATH:=$(PATH):$(GOPATH)/bin
export PATH
downloadGoMobile:
	cd ~ ;curl -L https://raw.githubusercontent.com/2dust/AndroidLibV2rayLite/master/ubuntu-cli-install-android-sdk.sh | sudo bash -
	ls ~
	ls ~/android-sdk-linux/

BuildMobile:
	gomobile init
	env GO111MODULE=off gomobile bind -v -ldflags='-s -w' github.com/2dust/AndroidLibV2rayLite

all: asset pb fetchDep
	@echo DONE

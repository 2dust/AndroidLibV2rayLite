# AndroidLibV2rayLite

## Build requirements
* JDK
* Android SDK
* Go
* gomobile

## Build instructions
0. `git clone https://github.com/XTLS/Xray-core.git`
1. `git clone [repo] && cd AndroidLibV2rayLite`
2. `gomobile init`
3. `go mod tidy -v`
4. `gomobile bind -v -androidapi 21 -ldflags='-s -w -checklinkname=0' ./`

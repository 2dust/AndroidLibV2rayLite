module github.com/2dust/AndroidLibV2rayLite

go 1.26.3

require (
	github.com/xtls/xray-core v26.3.27+incompatible
	golang.org/x/mobile v0.0.0-20260217195705-b56b3793a9c4
)

replace github.com/xtls/xray-core => ../Xray-core

replace github.com/wlynxg/anet => github.com/wlynxg/anet v0.0.5

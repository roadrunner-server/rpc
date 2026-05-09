module github.com/roadrunner-server/rpc/v6

go 1.26

toolchain go1.26.0

require (
	connectrpc.com/connect v1.19.2
	github.com/roadrunner-server/endure/v2 v2.6.2
	github.com/roadrunner-server/errors v1.5.0
	github.com/roadrunner-server/tcplisten v1.5.2
	google.golang.org/protobuf v1.36.11
)

exclude (
	github.com/spf13/viper v1.18.0
	github.com/spf13/viper v1.18.1
)

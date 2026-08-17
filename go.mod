module github.com/roadrunner-server/rpc/v6

go 1.26

toolchain go1.26.6

require (
	github.com/roadrunner-server/endure/v2 v2.6.2
	github.com/roadrunner-server/errors v1.5.0
	github.com/roadrunner-server/goridge/v4 v4.0.0-beta.3
	github.com/roadrunner-server/tcplisten v1.5.2
	github.com/stretchr/testify v1.12.0
)

exclude (
	github.com/spf13/viper v1.18.0
	github.com/spf13/viper v1.18.1
)

require (
	github.com/kr/pretty v0.3.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

module github.com/roadrunner-server/rpc/v6

go 1.26

toolchain go1.26.0

require (
	github.com/goccy/go-json v0.10.6
	github.com/roadrunner-server/endure/v2 v2.6.2
	github.com/roadrunner-server/errors v1.5.0
	github.com/roadrunner-server/goridge/v4 v4.0.0-beta.1
	github.com/roadrunner-server/tcplisten v1.5.2
	go.uber.org/zap v1.28.0
)

exclude (
	github.com/spf13/viper v1.18.0
	github.com/spf13/viper v1.18.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

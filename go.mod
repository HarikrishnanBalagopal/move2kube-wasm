module github.com/konveyor/move2kube-wasm

go 1.21.0

// require github.com/sirupsen/logrus v1.9.3
require github.com/sirupsen/logrus v1.9.4-0.20230606125235-dd1b4c2e81af

require (
	github.com/Masterminds/semver/v3 v3.2.1
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.16.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)

// See this PR https://github.com/spf13/afero/pull/400
replace github.com/spf13/afero v1.9.5 => github.com/jilleJr/afero v1.9.6-0.20230808154115-904d2897c961

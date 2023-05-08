module github.com/Azure/azure-storage-fuse/v2

go 1.16

require (
	cloud.google.com/go/storage v1.30.1 // indirect
	github.com/Azure/azure-pipeline-go v0.2.4-0.20220425205405-09e6f201e1e4
	github.com/Azure/azure-storage-azcopy/v10 v10.18.1
	github.com/Azure/azure-storage-blob-go v0.15.0
	github.com/Azure/go-autorest/autorest v0.11.29
	github.com/Azure/go-autorest/autorest/adal v0.9.23
	github.com/JeffreyRichter/enum v0.0.0-20180725232043-2567042f9cda
	github.com/fsnotify/fsnotify v1.6.0
	github.com/golang/mock v1.6.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/mitchellh/mapstructure v1.5.0
	github.com/montanaflynn/stats v0.6.6
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58
	github.com/radovskyb/watcher v1.0.7
	github.com/sevlyar/go-daemon v0.1.6
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.15.0
	github.com/stretchr/testify v1.8.2
	go.uber.org/atomic v1.11.0
	golang.org/x/crypto v0.8.0 // indirect
	gopkg.in/ini.v1 v1.67.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/spf13/cobra => github.com/gapra-msft/cobra v1.4.1-0.20220411185530-5b83e8ba06dd

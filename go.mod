module github.com/Azure/azure-storage-fuse/v2

go 1.16

require (
	github.com/Azure/azure-pipeline-go v0.2.3
	github.com/Azure/azure-storage-azcopy/v10 v10.13.1-0.20211218014522-24209b81028e
	github.com/Azure/azure-storage-blob-go v0.13.1-0.20210823171415-e7932f52ad61
	github.com/Azure/azure-storage-file-go v0.6.1-0.20220815164042-f37a99d62e3f
	github.com/Azure/go-autorest/autorest v0.11.27
	github.com/Azure/go-autorest/autorest/adal v0.9.20
	github.com/JeffreyRichter/enum v0.0.0-20180725232043-2567042f9cda
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/mock v1.6.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/mitchellh/mapstructure v1.4.1
	github.com/montanaflynn/stats v0.6.6
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58
	github.com/sevlyar/go-daemon v0.1.5
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	go.uber.org/atomic v1.7.0
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

replace github.com/spf13/cobra => github.com/gapra-msft/cobra v1.4.1-0.20220411185530-5b83e8ba06dd

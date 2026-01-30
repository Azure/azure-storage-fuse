module github.com/Azure/azure-storage-fuse/v2

go 1.25.1

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.19.1
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.0
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.2
	github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake v1.4.2
	github.com/JeffreyRichter/enum v0.0.0-20180725232043-2567042f9cda
	github.com/fsnotify/fsnotify v1.9.0
	github.com/go-viper/mapstructure/v2 v2.4.0
	github.com/golang/mock v1.6.0
	github.com/montanaflynn/stats v0.7.0
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58
	github.com/radovskyb/watcher v1.0.7
	github.com/sevlyar/go-daemon v0.1.6
	github.com/shirou/gopsutil/v4 v4.25.12
	github.com/spf13/cobra v1.10.1
	github.com/spf13/pflag v1.0.10
	github.com/spf13/viper v1.21.0
	github.com/stretchr/testify v1.11.1
	github.com/vibhansa-msft/blobfilter v0.0.0-20250115104552-d9d40722be3e
	github.com/vibhansa-msft/tlru v0.0.0-20240410102558-9e708419e21f
	github.com/winfsp/cgofuse v1.6.0
	go.uber.org/atomic v1.11.0
	golang.org/x/sys v0.38.0
	gopkg.in/ini.v1 v1.67.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/text v0.30.0 // indirect
)

replace github.com/spf13/cobra => github.com/gapra-msft/cobra v1.4.1-0.20220411185530-5b83e8ba06dd

//replace github.com/Azure/azure-storage-azcopy/v10 v10.19.1-0.20230717101935-ab8ff0a85e48 => <local path>/azure-storage-azcopy

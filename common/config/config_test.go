/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package config

import (
	"os"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type Labels struct {
	App string `config:"app"`
}

type Metadata struct {
	Name  string `config:"name"`
	Label Labels `config:"labels"`
}

type MatchLabels struct {
	App string `config:"app"`
}

type Selector struct {
	Match MatchLabels `config:"matchLabels"`
}

type Template struct {
	Meta Metadata `config:"metadata"`
}

type Spec struct {
	Replicas int32    `config:"replicas"`
	Select   Selector `config:"selector"`
	Templ    Template `config:"template"`
}

type Config1 struct {
	ApiVer string   `config:"apiVersion"`
	Kind   string   `config:"kind"`
	Meta   Metadata `config:"metadata"`
}

type Config2 struct {
	ApiVer string   `config:"apiVersion"`
	Kind   string   `config:"kind"`
	Meta   Metadata `config:"metadata"`
	Specs  Spec     `config:"spec"`
}

type ConfigTestSuite struct {
	suite.Suite
}

var config1 = `
apiVersion: v1
kind: Pod
metadata:
  name: rss-site
  labels:
    app: web
`

var config2 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rss-site
  labels:
    app: web
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
`

var metaconf = `
name: hooli
labels:
  app: pied-piper
`

var specconf = `
replicas: 2
selector:
  matchLabels:
    app: web
template:
  metadata:
    labels:
      app: web
`

// Function to test config reader when there is both env vars and cli flags that overlap config file.
func (suite *ConfigTestSuite) TestOverlapShadowConfigReader() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	specOptsTruth := &Spec{
		Replicas: 2,
		Select: Selector{
			Match: MatchLabels{
				App: "bachmanity",
			},
		},
		Templ: Template{
			Meta: Metadata{
				Label: Labels{
					App: "prof. bighead",
				},
			},
		},
	}

	err := os.Setenv("CF_TEST_MATCHLABELS_APP", specOptsTruth.Select.Match.App)
	assert.Nil(err)
	BindEnv("selector.matchLabels.app", "CF_TEST_MATCHLABELS_APP")

	templAppFlag := AddStringFlag("template-flag", "defoval", "OJ")
	err = templAppFlag.Value.Set(specOptsTruth.Templ.Meta.Label.App)
	assert.Nil(err)
	templAppFlag.Changed = true
	BindPFlag("template.metadata.labels.app", templAppFlag)
	err = os.Setenv("CF_TEST_TEMPLABELS_APP", "somethingthatshouldnotshowup")
	BindEnv("template.metadata.labels.app", "CF_TEST_TEMPLABELS_APP")

	err = ReadConfigFromReader(strings.NewReader(specconf))
	assert.Nil(err)
	specOpts := &Spec{}
	err = Unmarshal(specOpts)
	assert.Nil(err)
	assert.Equal(specOptsTruth, specOpts)

}

// Function to test only config file reader: testcase 2
func (suite *ConfigTestSuite) TestPlainConfig2Reader() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())
	err := ReadConfigFromReader(strings.NewReader(config2))
	assert.Nil(err)

	//Case 1
	metaDeepOpts2 := &Metadata{}
	metaDeepOpts2Truth := &Metadata{
		Label: Labels{
			App: "web",
		},
	}
	err = UnmarshalKey("spec.template.metadata", metaDeepOpts2)
	assert.Nil(err)
	assert.Equal(metaDeepOpts2Truth, metaDeepOpts2)

	//Case 2
	templatOpts2 := &Template{}
	templatOpts2Truth := &Template{
		Meta: Metadata{
			Label: Labels{
				App: "web",
			},
		},
	}
	err = UnmarshalKey("spec.template", templatOpts2)
	assert.Nil(err)
	assert.Equal(templatOpts2Truth, templatOpts2)

	//Case 3
	specOpts2 := &Spec{}
	specOpts2Truth := &Spec{
		Replicas: 2,
		Select: Selector{
			Match: MatchLabels{
				App: "web",
			},
		},
		Templ: Template{
			Meta: Metadata{
				Label: Labels{
					App: "web",
				},
			},
		},
	}
	err = UnmarshalKey("spec", specOpts2)
	assert.Nil(err)
	assert.Equal(specOpts2Truth, specOpts2)

	// Case 4
	opts2 := &Config2{}
	opts2Truth := &Config2{
		ApiVer: "apps/v1",
		Kind:   "Deployment",
		Meta: Metadata{
			Name: "rss-site",
			Label: Labels{
				App: "web",
			},
		},
		Specs: Spec{
			Replicas: 2,
			Select: Selector{
				Match: MatchLabels{
					App: "web",
				},
			},
			Templ: Template{
				Meta: Metadata{
					Label: Labels{
						App: "web",
					},
				},
			},
		},
	}
	err = Unmarshal(opts2)
	assert.Nil(err)
	assert.Equal(opts2Truth, opts2)

	//Case 5
	apiVersion := 0
	err = UnmarshalKey("apiVersion", &apiVersion)
	assert.NotNil(err)
}

// Function to test only config file reader: testcase 1
func (suite *ConfigTestSuite) TestPlainConfig1Reader() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())
	err := ReadConfigFromReader(strings.NewReader(config1))
	assert.Nil(err)

	//Case1
	opts1 := &Config1{}
	opts1Truth := &Config1{
		ApiVer: "v1",
		Kind:   "Pod",
		Meta: Metadata{
			Name: "rss-site",
			Label: Labels{
				App: "web",
			},
		},
	}
	err = Unmarshal(opts1)
	assert.Nil(err)
	assert.Equal(opts1Truth, opts1)

	//Case2
	metaOpts1 := &Metadata{}
	metaOpts1Truth := &Metadata{
		Name: "rss-site",
		Label: Labels{
			App: "web",
		},
	}
	err = UnmarshalKey("metadata", metaOpts1)
	assert.Nil(err)
	assert.Equal(metaOpts1Truth, metaOpts1)

	//Case 3
	labelOpts1 := &Labels{}
	labelOpts1Truth := &Labels{
		App: "web",
	}
	err = UnmarshalKey("metadata.labels", labelOpts1)
	assert.Nil(err)
	assert.Equal(labelOpts1Truth, labelOpts1)

	//Case 4:
	randOpts := struct {
		NewName       string `config:"newname"`
		NotExistField int    `config:"notexists"`
	}{}

	err = Unmarshal(&randOpts)
	assert.Empty(randOpts)
}

// Function to test config reader when there is environment variables that shadow config file
func (suite *ConfigTestSuite) TestEnvShadowedConfigReader() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())
	metaOptsTruth := &Metadata{
		Name: "mcdhee",
		Label: Labels{
			App: "zigby",
		},
	}
	err := os.Setenv("CF_TEST_NAME", metaOptsTruth.Name)
	assert.Nil(err)
	err = os.Setenv("CF_TEST_APP", metaOptsTruth.Label.App)
	assert.Nil(err)

	//Case 1
	BindEnv("name", "CF_TEST_NAME")
	BindEnv("labels.app", "CF_TEST_APP")

	metaOpts := &Metadata{}
	err = Unmarshal(metaOpts)
	assert.Nil(err)
	assert.Equal(metaOptsTruth, metaOpts)

	ResetConfig()

	//Case 2
	err = ReadConfigFromReader(strings.NewReader(metaconf))
	assert.Nil(err)
	BindEnv("name", "CF_TEST_NAME")
	BindEnv("labels.app", "CF_TEST_APP")
	metaOpts = &Metadata{}
	err = Unmarshal(metaOpts)
	assert.Nil(err)
	assert.Equal(metaOptsTruth, metaOpts)

}

// Function to test config reader when there is cli flags that shadow config file
func (suite *ConfigTestSuite) TestFlagShadowedConfigReader() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())
	metaOptsTruth := &Metadata{
		Name: "mcdhee",
		Label: Labels{
			App: "zigby",
		},
	}

	//Case 1
	nameFlag := AddStringFlag("name", "defo", "nahnahnah")
	err := nameFlag.Value.Set(metaOptsTruth.Name)
	assert.Nil(err)
	nameFlag.Changed = true
	BindPFlag("name", nameFlag)

	appFlag := AddStringFlag("app", "seefood", "jianyang")
	err = appFlag.Value.Set(metaOptsTruth.Label.App)
	assert.Nil(err)
	appFlag.Changed = true
	BindPFlag("labels.app", appFlag)

	metaOpts := &Metadata{}
	err = Unmarshal(metaOpts)
	assert.Nil(err)
	assert.Equal(metaOptsTruth, metaOpts)

	ResetConfig()

	//Case 2
	err = ReadConfigFromReader(strings.NewReader(metaconf))
	assert.Nil(err)
	nameFlag = AddStringFlag("name", "defo", "nahnahnah")
	err = nameFlag.Value.Set(metaOptsTruth.Name)
	assert.Nil(err)
	nameFlag.Changed = true
	BindPFlag("name", nameFlag)

	appFlag = AddStringFlag("app", "seefood", "jianyang")
	err = appFlag.Value.Set(metaOptsTruth.Label.App)
	assert.Nil(err)
	appFlag.Changed = true
	BindPFlag("labels.app", appFlag)

	metaOpts = &Metadata{}
	err = Unmarshal(metaOpts)
	assert.Nil(err)
	assert.Equal(metaOptsTruth, metaOpts)

}

func (suite *ConfigTestSuite) TestAddFlags() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	flag := AddBoolFlag("boolFlag", false, "")
	assert.NotNil(flag)

	flag = AddBoolPFlag("b", false, "")
	assert.NotNil(flag)

	flag = AddDurationFlag("durationFlag", 5, "")
	assert.NotNil(flag)

	flag = AddFloat64Flag("Float64Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddIntFlag("intFlag", 5.0, "")
	assert.NotNil(flag)

	flag = AddInt8Flag("int8Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddInt16Flag("int16Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddInt32Flag("int32Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddInt64Flag("int64Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddUintFlag("uintFlag", 5.0, "")
	assert.NotNil(flag)

	flag = AddUint8Flag("uint8Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddUint16Flag("uint16Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddUint32Flag("uint32Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddUint64Flag("uint64Flag", 5.0, "")
	assert.NotNil(flag)

	flag = AddStringFlag("stringFlag", "abc", "")
	assert.NotNil(flag)

	Set("abcd", "1234")
	SetBool("flag", true)
	BindPFlag("abcd", flag)
	BindEnv("abcd", "CF_TEST_ABCD")
}

func (suite *ConfigTestSuite) TestConfigFileDescryption() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	os.WriteFile("test.yaml", []byte(config2), 0644)
	plaintext, err := os.ReadFile("test.yaml")
	assert.Nil(err)
	assert.NotEqual(plaintext, nil)

	cipherText, err := common.EncryptData(plaintext, []byte("123123123123123123123123"))
	assert.Nil(err)
	err = os.WriteFile("test_enc.yaml", cipherText, 0644)
	assert.Nil(err)

	err = DecryptConfigFile("test_enc.yaml", "123123123123123123123123")
	assert.Nil(err)

	_ = os.Remove("test.yaml")
	_ = os.Remove("test_enc.yaml")
}

func (suite *ConfigTestSuite) cleanupTest() {
	ResetConfig()
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

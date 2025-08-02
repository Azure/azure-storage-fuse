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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type keysTreeTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *keysTreeTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func TestKeysTree(t *testing.T) {
	suite.Run(t, new(keysTreeTestSuite))
}

type parseVal struct {
	val    string
	toType reflect.Kind
	result any
}

func (suite *keysTreeTestSuite) TestParseValue() {
	var inputs = []parseVal{
		{val: "true", toType: reflect.TypeOf(true), result: true},
		{val: "87", toType: reflect.TypeOf(int(0)), result: 87},
		{val: "127", toType: reflect.TypeOf(int8(0)), result: 127},
		{val: "32767", toType: reflect.TypeOf(int16(0)), result: 32767},
		{val: "2147483647", toType: reflect.TypeOf(int32(0)), result: 2147483647},
		{val: "9223372036854775807", toType: reflect.TypeOf(int64(0)), result: 9223372036854775807},
		{val: "1374", toType: reflect.TypeOf(uint(0)), result: 1374},
		{val: "255", toType: reflect.TypeOf(uint8(0)), result: 255},
		{val: "65535", toType: reflect.TypeOf(uint16(0)), result: 65535},
		{val: "4294967295", toType: reflect.TypeOf(uint32(0)), result: 4294967295},
		{val: "18446744073709551615", toType: reflect.TypeOf(uint64(0)), result: uint64(18446744073709551615)},
		{val: "6.24321908234", toType: reflect.TypeOf(float32(0)), result: (float32)(6.24321908234)},
		{val: "31247921747687123.123871293791263", toType: reflect.TypeOf(float64(0)), result: 31247921747687123.123871293791263},
		{val: "6-8i", toType: reflect.TypeOf(complex64(0)), result: 6 - 8i},
		{val: "2341241-910284i", toType: reflect.TypeOf(complex128(0)), result: 2341241 - 910284i},
		{val: "Hello World", toType: reflect.TypeOf(""), result: "Hello World"},
		{val: "Hello World,random path", toType: reflect.TypeOf([]string{}), result: []string{"Hello World", "random path"}},
		{val: "23,45", toType: reflect.TypeOf([]int{}), result: nil},
	}
	for _, i := range inputs {
		suite.Run(i.val, func() {
			output := parseValue(i.val, i.toType)
			suite.assert.EqualValues(i.result, output)
		})
	}
}

func (suite *keysTreeTestSuite) TestParseValueErr() {
	var inputs = []parseVal{
		{val: "Hello World", toType: reflect.TypeOf(true)},
		{val: "Hello World", toType: reflect.TypeOf(int(0))},
		{val: "Hello World", toType: reflect.TypeOf(int8(0))},
		{val: "Hello World", toType: reflect.TypeOf(int16(0))},
		{val: "Hello World", toType: reflect.TypeOf(int32(0))},
		{val: "Hello World", toType: reflect.TypeOf(int64(0))},
		{val: "Hello World", toType: reflect.TypeOf(uint(0))},
		{val: "Hello World", toType: reflect.TypeOf(uint8(0))},
		{val: "Hello World", toType: reflect.TypeOf(uint16(0))},
		{val: "Hello World", toType: reflect.TypeOf(uint32(0))},
		{val: "Hello World", toType: reflect.TypeOf(uint64(0))},
		{val: "Hello World", toType: reflect.TypeOf(float32(0))},
		{val: "Hello World", toType: reflect.TypeOf(float64(0))},
		{val: "Hello World", toType: reflect.TypeOf(complex64(0))},
		{val: "Hello World", toType: reflect.TypeOf(complex128(0))},
	}
	for _, i := range inputs {
		suite.Run(i.val, func() {
			output := parseValue(i.val, i.toType)
			suite.assert.Nil(i.result, output)
		})
	}
}

func (suite *keysTreeTestSuite) TestIsPrimitiveType() {
	var inputs = []reflect.Kind{
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.String,
	}
	for _, i := range inputs {
		suite.Run(i.String(), func() {
			output := isPrimitiveType(i)
			suite.assert.True(output)
		})
	}
}

func (suite *keysTreeTestSuite) TestIsNotPrimitiveType() {
	var inputs = []reflect.Kind{
		reflect.Array,
		reflect.Func,
		reflect.Map,
		reflect.Ptr,
		reflect.Slice,
		reflect.Struct,
	}
	for _, i := range inputs {
		suite.Run(i.String(), func() {
			output := isPrimitiveType(i)
			suite.assert.False(output)
		})
	}
}

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
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const STRUCT_TAG = "config"

type TreeNode struct {
	children map[string]*TreeNode
	value    interface{}
	name     string
}

type Tree struct {
	head *TreeNode
}

// NewTree returns a new Tree object with the head initialized to a default root TreeNode
func NewTree() *Tree {
	return &Tree{
		head: NewTreeNode("root"),
	}
}

// NewTreeNode returns a TreeNode initialized with the passed in string as name
func NewTreeNode(name string) *TreeNode {
	return &TreeNode{
		children: make(map[string]*TreeNode),
		name:     name,
	}
}

// Insert function is used to insert a new object into the tree
// The key is specified as a dot separated hierarchical value
// For eg. root.child1.child2
func (tree *Tree) Insert(key string, value interface{}) {
	subKeys := strings.Split(key, ".")
	curNode := tree.head
	for _, idx := range subKeys {
		if subStruct, ok := curNode.children[idx]; ok {
			if subStruct != nil {
				curNode = subStruct
			} else {
				break
			}
		} else {
			curNode.children[idx] = NewTreeNode(idx)
			curNode = curNode.children[idx]
		}
	}

	curNode.value = value
}

// Print is a utility function that prints the Tree in a level order fashion
func (tree *Tree) Print() {
	nodes := make([]*TreeNode, 0)
	nodes = append(nodes, tree.head)
	for len(nodes) > 0 {
		curNode := nodes[0]
		nodes = nodes[1:]
		for key, node := range curNode.children {
			fmt.Print(key, ",")
			nodes = append(nodes, node)
		}
	}
}

// GetSubTree returns the sub Tree that is present from the last child of the key passed in.
// For eg. to retrieve the subtree starting from child2 the passed key can be root.child1.child2
func (tree *Tree) GetSubTree(key string) *TreeNode {
	subKeys := strings.Split(key, ".")
	curNode := tree.head
	for _, idx := range subKeys {
		if curNode == nil {
			return nil
		}
		curNode = curNode.children[idx]
	}
	return curNode
}

// parseValue is a utility function that accepts a val and returns the parsed value of that type.
// Apart from primitive types it also handles the slice type where value is a comma separated string
func parseValue(val string, toType reflect.Type) interface{} {
	switch toType.Kind() {
	case reflect.Slice:
		if toType.Elem().Kind() != reflect.String {
			return nil // only support []string for now
		}
		stringSlice := strings.Split(val, ",")
		for i := range stringSlice {
			stringSlice[i] = strings.TrimSpace(strings.Trim(stringSlice[i], "[]"))
		}
		return stringSlice
	case reflect.Bool:
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			return nil
		}
		return parsed
	case reflect.Int:
		parsed, err := strconv.ParseInt(val, 0, 0)
		if err != nil {
			return nil
		}
		return int(parsed)
	case reflect.Int8:
		parsed, err := strconv.ParseInt(val, 0, 8)
		if err != nil {
			return nil
		}
		return int8(parsed)
	case reflect.Int16:
		parsed, err := strconv.ParseInt(val, 0, 16)
		if err != nil {
			return nil
		}
		return int16(parsed)
	case reflect.Int32:
		parsed, err := strconv.ParseInt(val, 0, 32)
		if err != nil {
			return nil
		}
		return int32(parsed)
	case reflect.Int64:
		parsed, err := strconv.ParseInt(val, 0, 64)
		if err != nil {
			return nil
		}
		return int64(parsed)
	case reflect.Uint:
		parsed, err := strconv.ParseUint(val, 0, 0)
		if err != nil {
			return nil
		}
		return uint(parsed)
	case reflect.Uint8:
		parsed, err := strconv.ParseUint(val, 0, 8)
		if err != nil {
			return nil
		}
		return uint8(parsed)
	case reflect.Uint16:
		parsed, err := strconv.ParseUint(val, 0, 16)
		if err != nil {
			return nil
		}
		return uint16(parsed)
	case reflect.Uint32:
		parsed, err := strconv.ParseUint(val, 0, 32)
		if err != nil {
			return nil
		}
		return uint32(parsed)
	case reflect.Uint64:
		parsed, err := strconv.ParseUint(val, 0, 64)
		if err != nil {
			return nil
		}
		return uint64(parsed)
	case reflect.Float32:
		parsed, err := strconv.ParseFloat(val, 32)
		if err != nil {
			return nil
		}
		return float32(parsed)
	case reflect.Float64:
		parsed, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil
		}
		return float64(parsed)
	case reflect.Complex64:
		parsed, err := strconv.ParseComplex(val, 64)
		if err != nil {
			return nil
		}
		return complex64(parsed)
	case reflect.Complex128:
		parsed, err := strconv.ParseComplex(val, 128)
		if err != nil {
			return nil
		}
		return complex128(parsed)
	case reflect.String:
		return val
	default:
		return nil
	}
}

// MergeWithKey is used to merge the contained tree with the object (obj) that is passed in as parameter.
// getValue parameter is a function that accepts the value stored in a TreeNode and performs any business logic and returns the value that has to be placed in the obj parameter
// it must also return true|false based on which the value will be set in the obj parameter.
func (tree *Tree) MergeWithKey(key string, obj interface{}, getValue func(val interface{}) (res interface{}, ok bool)) {
	subTree := tree.GetSubTree(key)
	if subTree == nil {
		return
	}
	var elem = reflect.Indirect(reflect.ValueOf(obj))
	if obj == nil {
		return
	}

	if elem.Type().Kind() == reflect.Struct {
		for i := 0; i < elem.NumField(); i++ {
			idx := getIdxFromField(elem.Type().Field(i))
			if _, ok := subTree.children[idx]; ok {
				if elem.Field(i).Type().Kind() == reflect.Struct {
					subKey := key + "." + idx
					tree.MergeWithKey(subKey, elem.Field(i).Addr().Interface(), getValue)
				} else if elem.Field(i).Type().Kind() == reflect.Ptr {
					subKey := key + "." + idx
					tree.MergeWithKey(subKey, elem.Field(i).Elem().Addr().Interface(), getValue)
				} else {
					val, ok := getValue(subTree.children[idx].value)
					if ok {
						assignToField(elem.Field(i), val)
					}
				}
			}
		}
	} else if isPrimitiveType(elem.Type().Kind()) {
		val, ok := getValue(subTree.value)
		if ok {
			assignToField(elem, val)
		}
	}
}

// Merge performs the same function as MergeWithKey but at the root level
func (tree *Tree) Merge(obj interface{}, getValue func(val interface{}) (res interface{}, ok bool)) {
	subTree := tree.head
	if subTree == nil {
		return
	}
	var elem = reflect.Indirect(reflect.ValueOf(obj))
	if obj == nil {
		return
	}

	if elem.Type().Kind() == reflect.Struct {
		for i := 0; i < elem.NumField(); i++ {
			idx := getIdxFromField(elem.Type().Field(i))
			if _, ok := subTree.children[idx]; ok {
				if elem.Field(i).Type().Kind() == reflect.Struct {
					subKey := idx
					tree.MergeWithKey(subKey, elem.Field(i).Addr().Interface(), getValue)
				} else if elem.Field(i).Type().Kind() == reflect.Ptr {
					subKey := idx
					tree.MergeWithKey(subKey, elem.Field(i).Elem().Addr().Interface(), getValue)
				} else {
					val, ok := getValue(subTree.children[idx].value)
					if ok {
						assignToField(elem.Field(i), val)
					}
				}
			}
		}
	} else if isPrimitiveType(elem.Type().Kind()) {
		val, ok := getValue(subTree.value)
		if ok {
			assignToField(elem, val)
		}
	}
}

// isPrimitiveType is a utility function that returns true if the kind parameter is a primitive data type or not
func isPrimitiveType(kind reflect.Kind) bool {
	switch kind {
	case reflect.Bool:
		return true
	case reflect.Int:
		return true
	case reflect.Int8:
		return true
	case reflect.Int16:
		return true
	case reflect.Int32:
		return true
	case reflect.Int64:
		return true
	case reflect.Uint:
		return true
	case reflect.Uint8:
		return true
	case reflect.Uint16:
		return true
	case reflect.Uint32:
		return true
	case reflect.Uint64:
		return true
	case reflect.Float32:
		return true
	case reflect.Float64:
		return true
	case reflect.Complex64:
		return true
	case reflect.Complex128:
		return true
	case reflect.String:
		return true
	default:
		return false
	}
}

// assignToField is utility function to set the val to the passed field based on it's state
func assignToField(field reflect.Value, val interface{}) {
	if field.CanSet() {
		if reflect.TypeOf(val).Kind() == reflect.String {
			parseVal := parseValue(val.(string), field.Type())
			if parseVal != nil {
				field.Set(reflect.ValueOf(parseVal))
			}
		} else {
			field.Set(reflect.ValueOf(val))
		}
	}
}

// getIdxFromField is a utility function that returns the key to index into the map based on struct tags.
func getIdxFromField(structField reflect.StructField) string {
	idx := structField.Tag.Get(STRUCT_TAG)
	if idx == "" {
		idx = strings.ToLower(structField.Name)
	}
	return idx
}

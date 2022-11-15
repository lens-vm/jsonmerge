/*
 * Copyright (c) 2022, John-Alan Simmons
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 *
 * 1. Redistributions of source code must retain the above copyright notice,
 *    this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 * 3. Neither the name of mosquitto nor the names of its
 *    contributors may be used to endorse or promote products derived from
 *    this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
 * AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE
 * LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
 * CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
 * SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
 * INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
 * CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */

package jsonmerge

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/valyala/fastjson"
)

var (
	_ container = (*fastdoc)(nil)
	_ container = (*fastarray)(nil)
)

type container interface {
	get(key string) (*fastjson.Value, error)
	set(key string, val *fastjson.Value) error
	add(key string, val *fastjson.Value) error
	remove(key string) error
}

type fastdoc struct {
	v *fastjson.Object
}

func (d *fastdoc) get(key string) (*fastjson.Value, error) {
	if val := d.v.Get(key); val != nil {
		return val, nil
	}
	return nil, fmt.Errorf("missing key %v", key)
}

func (d *fastdoc) set(key string, val *fastjson.Value) error {
	d.v.Set(key, val)
	return nil
}

func (d *fastdoc) add(key string, val *fastjson.Value) error {
	return d.set(key, val)
}

func (d *fastdoc) remove(key string) error {
	d.v.Del(key)
	return nil
}

type fastarray struct {
	v *fastjson.Value // required to be a fastjson.TypeArray
}

func (arr *fastarray) set(key string, val *fastjson.Value) error {
	idx, err := arr.getIndex(key)
	if err != nil {
		return err
	}

	arr.v.SetArrayItem(idx, val)
	return nil
}

func (arr *fastarray) add(key string, val *fastjson.Value) error {
	fmt.Println("fastarray add")
	// fmt.Printf("before pointer %p\n", arr)
	// append key
	if key == "-" {
		// NOTE: math.MaxInt just gurantees
		// that we append to the end.
		// It *doesn't* insert at this index
		// the SetArrayItem func just checks if
		// the given index is *larger* then
		// the current length, and applies
		// a simple append if so.
		arr.v.SetArrayItem(math.MaxInt, val)
		return nil
	}

	idx, err := arr.getIndex(key)
	if err != nil {
		return err
	}

	// add into the array at index
	arr.v.InsertArrayItem(idx, val)
	// fmt.Printf("after pointer %p\n", arr)
	return nil
}

func (arr *fastarray) get(key string) (*fastjson.Value, error) {
	return arr.v.Get(key), nil
}

func (arr *fastarray) remove(key string) error {
	arr.v.Del(key)
	return nil
}

func (arr *fastarray) getIndex(key string) (int, error) {
	idx, err := strconv.Atoi(key)
	if err != nil {
		return 0, err
	}

	if idx < 0 {
		return 0, fmt.Errorf("invalid negative index %v", idx)
	}

	return idx, nil
}

// todo: Look into simplifying this type
// into just `type Operation fastjson.Object`
type Operation struct {
	v *fastjson.Object
}

func (o Operation) Kind() string {
	if val, ok := o.getStringField("op"); ok {
		return val
	}
	return "unknown"
}

func (o Operation) Path() (string, error) {
	if val, ok := o.getStringField("path"); ok {
		return val, nil
	}
	return "", fmt.Errorf("couldn't get path field")
}

func (o Operation) From() (string, error) {
	if val, ok := o.getStringField("from"); ok {
		return val, nil
	}
	return "", fmt.Errorf("couldn't get from field")
}

func (o Operation) value() *fastjson.Value {
	return o.v.Get("value")
}

func (o Operation) getStringField(key string) (string, bool) {
	obj := o.v.Get(key)
	if obj == nil {
		return "", false
	}
	val, err := obj.StringBytes()
	if err != nil {
		return "", false
	}
	return string(val), true
}

func (o Operation) Marshal() []byte {
	return o.v.MarshalTo(nil)
}

type Patch []Operation

func DecodePatch(buf []byte) (Patch, error) {
	parsed, err := fastjson.ParseBytes(buf)
	if err != nil {
		return nil, err
	}

	if parsed.Type() != fastjson.TypeArray {
		return nil, fmt.Errorf("unexpected patch type: %s", parsed.Type().String())
	}

	var patch Patch
	patchOps := parsed.GetArray()
	for _, patchOp := range patchOps {
		obj, err := patchOp.Object()
		if err != nil {
			return nil, err
		}

		patch = append(patch, Operation{
			v: obj,
		})
	}

	return patch, nil
}

func (p Patch) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("[")
	for _, op := range p {
		_, err := buf.Write(op.Marshal())
		if err != nil {
			return nil, err
		}
	}
	buf.WriteString("]")
	return buf.Bytes(), nil
}

func (p Patch) Apply(doc []byte) ([]byte, error) {
	if len(doc) == 0 {
		return doc, nil
	}

	parsedDoc, err := fastjson.ParseBytes(doc)
	if err != nil {
		return nil, err
	}

	patchedDoc, err := p.ApplyFast(parsedDoc)
	if err != nil {
		return nil, err
	}

	buf := patchedDoc.MarshalTo(nil)
	return buf, nil
}

func (p Patch) add(doc *container, op Operation) error {
	path, err := op.Path()
	if err != nil {
		return fmt.Errorf("%w: add operation failed decoding path", err)
	}
	fmt.Println("adding path:", path)

	con, key := findObject(doc, path)
	if con == nil {
		return fmt.Errorf("doc is missing path: %s", path)
	}
	fmt.Println("adding at key:", key)

	fmt.Println("before add:")
	spew.Dump(doc)
	spew.Dump(con)
	err = con.add(key, op.value())
	if err != nil {
		return fmt.Errorf("%w: executing add op for path: %s", err, path)
	}
	fmt.Println("after add:")
	spew.Dump(doc)
	spew.Dump(con)

	return nil
}

func (p Patch) remove(doc *container, op Operation) error {
	path, err := op.Path()
	if err != nil {
		return fmt.Errorf("%w: remove operation failed decoding path", err)
	}

	con, key := findObject(doc, path)
	if con == nil {
		return fmt.Errorf("doc is missing path: %s", path)
	}

	err = con.remove(key)
	if err != nil {
		return fmt.Errorf("%w: executing add op for path: %s", err, path)
	}

	return nil
}

func (p Patch) replace(doc *container, op Operation) error {
	path, err := op.Path()
	if err != nil {
		return fmt.Errorf("%w: replace operation: decoding path", err)
	}

	// apply replace on root
	if path == "" {
		val := op.value()
		con, err := intoFastType(val)
		if err != nil {
			return fmt.Errorf("%w: replace operation: value must be object or array", err)
		}

		*doc = con
		return nil
	}

	con, key := findObject(doc, path)
	if con == nil {
		return fmt.Errorf("%w: replace operation: doc is missing path: %s", err, path)
	}

	// exists?
	if _, err = con.get(key); err != nil {
		return fmt.Errorf("%w: replace operation: doc is missing key: %s", err, path)
	}

	if err = con.set(key, op.value()); err != nil {
		return fmt.Errorf("%w: replace operation: setting value for path: %s", err, path)
	}

	return nil
}

func (p Patch) move(doc *container, op Operation) error {
	from, err := op.From()
	if err != nil {
		return fmt.Errorf("%w: move operation: failed to decode from", err)
	}

	con, key := findObject(doc, from)
	if con == nil {
		return fmt.Errorf("move operation: doc is missing path %s", from)
	}

	val, err := con.get(key)
	if err != nil {
		return fmt.Errorf("%w: move operation: getting value at path: %s", err, from)
	}

	path, err := op.Path()
	if err != nil {
		return fmt.Errorf("%w: move operation: decoding path", err)
	}

	con, key = findObject(doc, path)

	if con == nil {
		return fmt.Errorf("%w: move operation: doc is missing destination path: %s", err, path)
	}

	err = con.add(key, val)
	if err != nil {
		return fmt.Errorf("%w: move operation: adding value at path: %s", err, key)
	}

	return nil
}

func (p Patch) test(doc *container, op Operation) error {
	panic("impl")
}

func (p Patch) copy(doc *container, op Operation) error {
	panic("impl")
}

func (p Patch) ApplyFast(doc *fastjson.Value) (*fastjson.Value, error) {
	var pd container
	switch doc.Type() {
	case fastjson.TypeArray:
		pd = &fastarray{
			v: doc,
		}
	case fastjson.TypeObject:
		obj := doc.GetObject()
		pd = &fastdoc{
			v: obj,
		}
	}

	var err error
	for _, op := range p {
		switch op.Kind() {
		case "add":
			fmt.Println("adding")
			err = p.add(&pd, op)
		case "remove":
			err = p.remove(&pd, op)
		case "replace":
			err = p.replace(&pd, op)
		case "move":
			err = p.move(&pd, op)
		case "test":
			err = p.test(&pd, op)
		case "copy":
			err = p.copy(&pd, op)
		default:
			err = fmt.Errorf("unexpected operation kind: %v", op.Kind())
		}

		if err != nil {
			return nil, err
		}
	}

	return doc, nil
}

// convert the generic fastjson.Value into a concrete implementation
// of container, either as a fastdoc or fastarray type
func intoFastType(val *fastjson.Value) (container, error) {
	var pd container
	switch val.Type() {
	case fastjson.TypeArray:
		pd = &fastarray{
			v: val,
		}
	case fastjson.TypeObject:
		obj := val.GetObject()
		pd = &fastdoc{
			v: obj,
		}
	default:
		return nil, fmt.Errorf("invalid json type for container: %v", val.Type().String())
	}
	return pd, nil
}

// iterates through the given patch path (json pointer)
// and retrieve the document and last element of the path
func findObject(pd *container, path string) (container, string) {
	doc := *pd

	split := strings.Split(path, "/")
	if len(split) < 2 {
		return nil, ""
	}

	parts := split[1 : len(split)-1]
	lastkey := split[len(split)-1]

	for _, part := range parts {
		next, err := doc.get(decodePatchKey(part))
		if next == nil || err != nil {
			return nil, ""
		}

		switch next.Type() {
		case fastjson.TypeArray:
			doc = &fastarray{
				v: next,
			}
		case fastjson.TypeObject:
			obj := next.GetObject()
			doc = &fastdoc{
				v: obj,
			}
		default:
			return nil, ""
		}
	}

	return doc, decodePatchKey(lastkey)
}

// From http://tools.ietf.org/html/rfc6901#section-4 :
//
// Evaluation of each reference token begins by decoding any escaped
// character sequence.  This is performed by first transforming any
// occurrence of the sequence '~1' to '/', and then transforming any
// occurrence of the sequence '~0' to '~'.

var (
	rfc6901Decoder = strings.NewReplacer("~1", "/", "~0", "~")
)

func decodePatchKey(k string) string {
	return rfc6901Decoder.Replace(k)
}

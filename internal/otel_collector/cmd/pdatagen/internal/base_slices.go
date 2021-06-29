// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"os"
	"strings"
)

const commonSliceTemplate = `
// MoveAndAppendTo moves all elements from the current slice and appends them to the dest.
// The current slice will be cleared.
func (es ${structName}) MoveAndAppendTo(dest ${structName}) {
	if *dest.orig == nil {
		// We can simply move the entire vector and avoid any allocations.
		*dest.orig = *es.orig
	} else {
		*dest.orig = append(*dest.orig, *es.orig...)
	}
	*es.orig = nil
}

// RemoveIf calls f sequentially for each element present in the slice.
// If f returns true, the element is removed from the slice.
func (es ${structName}) RemoveIf(f func(${elementName}) bool) {
	newLen := 0
	for i := 0; i < len(*es.orig); i++ {
		if f(es.At(i)) {
			continue
		}
		if newLen == i {
			// Nothing to move, element is at the right place.
			newLen++
			continue
		}
		(*es.orig)[newLen] = (*es.orig)[i]
		newLen++
	}
	// TODO: Prevent memory leak by erasing truncated values.
	*es.orig = (*es.orig)[:newLen]
}`

const commonSliceTestTemplate = `

func Test${structName}_MoveAndAppendTo(t *testing.T) {
	// Test MoveAndAppendTo to empty
	expectedSlice := generateTest${structName}()
	dest := New${structName}()
	src := generateTest${structName}()
	src.MoveAndAppendTo(dest)
	assert.EqualValues(t, generateTest${structName}(), dest)
	assert.EqualValues(t, 0, src.Len())
	assert.EqualValues(t, expectedSlice.Len(), dest.Len())

	// Test MoveAndAppendTo empty slice
	src.MoveAndAppendTo(dest)
	assert.EqualValues(t, generateTest${structName}(), dest)
	assert.EqualValues(t, 0, src.Len())
	assert.EqualValues(t, expectedSlice.Len(), dest.Len())

	// Test MoveAndAppendTo not empty slice
	generateTest${structName}().MoveAndAppendTo(dest)
	assert.EqualValues(t, 2*expectedSlice.Len(), dest.Len())
	for i := 0; i < expectedSlice.Len(); i++ {
		assert.EqualValues(t, expectedSlice.At(i), dest.At(i))
		assert.EqualValues(t, expectedSlice.At(i), dest.At(i+expectedSlice.Len()))
	}
}

func Test${structName}_RemoveIf(t *testing.T) {
	// Test RemoveIf on empty slice
	emptySlice := New${structName}()
	emptySlice.RemoveIf(func (el ${elementName}) bool {
		t.Fail()
		return false
	})

	// Test RemoveIf
	filtered := generateTest${structName}()
	pos := 0
	filtered.RemoveIf(func (el ${elementName}) bool {
		pos++
		return pos%3 == 0
	})
	assert.Equal(t, 5, filtered.Len())
}`

const commonSliceGenerateTest = `func generateTest${structName}() ${structName} {
	tv := New${structName}()
	fillTest${structName}(tv)
	return tv
}

func fillTest${structName}(tv ${structName}) {
	tv.Resize(7)
	for i := 0; i < tv.Len(); i++ {
		fillTest${elementName}(tv.At(i))
	}
}`

const slicePtrTemplate = `// ${structName} logically represents a slice of ${elementName}.
//
// This is a reference type. If passed by value and callee modifies it, the
// caller will see the modification.
//
// Must use New${structName} function to create new instances.
// Important: zero-initialized instance is not valid for use.
type ${structName} struct {
	// orig points to the slice ${originName} field contained somewhere else.
	// We use pointer-to-slice to be able to modify it in functions like Resize.
	orig *[]*${originName}
}

func new${structName}(orig *[]*${originName}) ${structName} {
	return ${structName}{orig}
}

// New${structName} creates a ${structName} with 0 elements.
// Can use "Resize" to initialize with a given length.
func New${structName}() ${structName} {
	orig := []*${originName}(nil)
	return ${structName}{&orig}
}

// Len returns the number of elements in the slice.
//
// Returns "0" for a newly instance created with "New${structName}()".
func (es ${structName}) Len() int {
	return len(*es.orig)
}

// At returns the element at the given index.
//
// This function is used mostly for iterating over all the values in the slice:
//   for i := 0; i < es.Len(); i++ {
//       e := es.At(i)
//       ... // Do something with the element
//   }
func (es ${structName}) At(ix int) ${elementName} {
	return new${elementName}((*es.orig)[ix])
}

// CopyTo copies all elements from the current slice to the dest.
func (es ${structName}) CopyTo(dest ${structName}) {
	srcLen := es.Len()
	destCap := cap(*dest.orig)
	if srcLen <= destCap {
		(*dest.orig) = (*dest.orig)[:srcLen:destCap]
		for i := range *es.orig {
			new${elementName}((*es.orig)[i]).CopyTo(new${elementName}((*dest.orig)[i]))
		}
		return
	}
	origs := make([]${originName}, srcLen)
	wrappers := make([]*${originName}, srcLen)
	for i := range *es.orig {
		wrappers[i] = &origs[i]
		new${elementName}((*es.orig)[i]).CopyTo(new${elementName}(wrappers[i]))
	}
	*dest.orig = wrappers
}

// Resize is an operation that resizes the slice:
// 1. If the newLen <= len then equivalent with slice[0:newLen:cap].
// 2. If the newLen > len then (newLen - cap) empty elements will be appended to the slice.
//
// Here is how a new ${structName} can be initialized:
//   es := New${structName}()
//   es.Resize(4)
//   for i := 0; i < es.Len(); i++ {
//       e := es.At(i)
//       // Here should set all the values for e.
//   }
func (es ${structName}) Resize(newLen int) {
	oldLen := len(*es.orig)
	oldCap := cap(*es.orig)
	if newLen <= oldLen {
		*es.orig = (*es.orig)[:newLen:oldCap]
		return
	}

	if newLen > oldCap {
		newOrig := make([]*${originName}, oldLen, newLen)
		copy(newOrig, *es.orig)
		*es.orig = newOrig
	}

	// Add extra empty elements to the array.
	extraOrigs := make([]${originName}, newLen-oldLen)
	for i := range extraOrigs {
		*es.orig = append(*es.orig, &extraOrigs[i])
	}
}

// Append will increase the length of the ${structName} by one and set the
// given ${elementName} at that new position.  The original ${elementName}
// could still be referenced so do not reuse it after passing it to this
// method.
// Deprecated: Use AppendEmpty.
func (es ${structName}) Append(e ${elementName}) {
	*es.orig = append(*es.orig, e.orig)
}

// AppendEmpty will append to the end of the slice an empty ${elementName}.
// It returns the newly added ${elementName}.
func (es ${structName}) AppendEmpty() ${elementName} {
	*es.orig = append(*es.orig, &${originName}{})
	return es.At(es.Len() - 1)
}`

const slicePtrTestTemplate = `func Test${structName}(t *testing.T) {
	es := New${structName}()
	assert.EqualValues(t, 0, es.Len())
	es = new${structName}(&[]*${originName}{})
	assert.EqualValues(t, 0, es.Len())

	es.Resize(7)
	emptyVal := new${elementName}(&${originName}{})
	testVal := generateTest${elementName}()
	assert.EqualValues(t, 7, es.Len())
	for i := 0; i < es.Len(); i++ {
		assert.EqualValues(t, emptyVal, es.At(i))
		fillTest${elementName}(es.At(i))
		assert.EqualValues(t, testVal, es.At(i))
	}
}

func Test${structName}_CopyTo(t *testing.T) {
	dest := New${structName}()
	// Test CopyTo to empty
	New${structName}().CopyTo(dest)
	assert.EqualValues(t, New${structName}(), dest)

	// Test CopyTo larger slice
	generateTest${structName}().CopyTo(dest)
	assert.EqualValues(t, generateTest${structName}(), dest)

	// Test CopyTo same size slice
	generateTest${structName}().CopyTo(dest)
	assert.EqualValues(t, generateTest${structName}(), dest)
}

func Test${structName}_Resize(t *testing.T) {
	es := generateTest${structName}()
	emptyVal := new${elementName}(&${originName}{})
	// Test Resize less elements.
	const resizeSmallLen = 4
	expectedEs := make(map[*${originName}]bool, resizeSmallLen)
	for i := 0; i < resizeSmallLen; i++ {
		expectedEs[es.At(i).orig] = true
	}
	assert.Equal(t, resizeSmallLen, len(expectedEs))
	es.Resize(resizeSmallLen)
	assert.Equal(t, resizeSmallLen, es.Len())
	foundEs := make(map[*${originName}]bool, resizeSmallLen)
	for i := 0; i < es.Len(); i++ {
		foundEs[es.At(i).orig] = true
	}
	assert.EqualValues(t, expectedEs, foundEs)

	// Test Resize more elements.
	const resizeLargeLen = 7
	oldLen := es.Len()
	expectedEs = make(map[*${originName}]bool, oldLen)
	for i := 0; i < oldLen; i++ {
		expectedEs[es.At(i).orig] = true
	}
	assert.Equal(t, oldLen, len(expectedEs))
	es.Resize(resizeLargeLen)
	assert.Equal(t, resizeLargeLen, es.Len())
	foundEs = make(map[*${originName}]bool, oldLen)
	for i := 0; i < oldLen; i++ {
		foundEs[es.At(i).orig] = true
	}
	assert.EqualValues(t, expectedEs, foundEs)
	for i := oldLen; i < resizeLargeLen; i++ {
		assert.EqualValues(t, emptyVal, es.At(i))
	}

	// Test Resize 0 elements.
	es.Resize(0)
	assert.Equal(t, 0, es.Len())
}

func Test${structName}_Append(t *testing.T) {
	es := generateTest${structName}()

	es.AppendEmpty()
	assert.EqualValues(t, &${originName}{}, es.At(7).orig)

	value := generateTest${elementName}()
	es.Append(value)
	assert.EqualValues(t, value.orig, es.At(8).orig)

	assert.Equal(t, 9, es.Len())
}`

const sliceValueTemplate = `// ${structName} logically represents a slice of ${elementName}.
//
// This is a reference type. If passed by value and callee modifies it, the
// caller will see the modification.
//
// Must use New${structName} function to create new instances.
// Important: zero-initialized instance is not valid for use.
type ${structName} struct {
	// orig points to the slice ${originName} field contained somewhere else.
	// We use pointer-to-slice to be able to modify it in functions like Resize.
	orig *[]${originName}
}

func new${structName}(orig *[]${originName}) ${structName} {
	return ${structName}{orig}
}

// New${structName} creates a ${structName} with 0 elements.
// Can use "Resize" to initialize with a given length.
func New${structName}() ${structName} {
	orig := []${originName}(nil)
	return ${structName}{&orig}
}

// Len returns the number of elements in the slice.
//
// Returns "0" for a newly instance created with "New${structName}()".
func (es ${structName}) Len() int {
	return len(*es.orig)
}

// At returns the element at the given index.
//
// This function is used mostly for iterating over all the values in the slice:
//   for i := 0; i < es.Len(); i++ {
//       e := es.At(i)
//       ... // Do something with the element
//   }
func (es ${structName}) At(ix int) ${elementName} {
	return new${elementName}(&(*es.orig)[ix])
}

// CopyTo copies all elements from the current slice to the dest.
func (es ${structName}) CopyTo(dest ${structName}) {
	srcLen := es.Len()
	destCap := cap(*dest.orig)
	if srcLen <= destCap {
		(*dest.orig) = (*dest.orig)[:srcLen:destCap]
	} else {
		(*dest.orig) = make([]${originName}, srcLen)
	}

	for i := range *es.orig {
		new${elementName}(&(*es.orig)[i]).CopyTo(new${elementName}(&(*dest.orig)[i]))
	}
}

// Resize is an operation that resizes the slice:
// 1. If the newLen <= len then equivalent with slice[0:newLen:cap].
// 2. If the newLen > len then (newLen - cap) empty elements will be appended to the slice.
//
// Here is how a new ${structName} can be initialized:
//   es := New${structName}()
//   es.Resize(4)
//   for i := 0; i < es.Len(); i++ {
//       e := es.At(i)
//       // Here should set all the values for e.
//   }
func (es ${structName}) Resize(newLen int) {
	oldLen := len(*es.orig)
	oldCap := cap(*es.orig)
	if newLen <= oldLen {
		*es.orig = (*es.orig)[:newLen:oldCap]
		return
	}

	if newLen > oldCap {
		newOrig := make([]${originName}, oldLen, newLen)
		copy(newOrig, *es.orig)
		*es.orig = newOrig
	}

	// Add extra empty elements to the array.
	empty := ${originName}{}
	for i := oldLen; i < newLen; i++ {
		*es.orig = append(*es.orig, empty)
	}
}

// Append will increase the length of the ${structName} by one and set the
// given ${elementName} at that new position.  The original ${elementName}
// could still be referenced so do not reuse it after passing it to this
// method.
// Deprecated: Use AppendEmpty.
func (es ${structName}) Append(e ${elementName}) {
	*es.orig = append(*es.orig, *e.orig)
}

// AppendEmpty will append to the end of the slice an empty ${elementName}.
// It returns the newly added ${elementName}.
func (es ${structName}) AppendEmpty() ${elementName} {
	*es.orig = append(*es.orig, ${originName}{})
	return es.At(es.Len() - 1)
}`

const sliceValueTestTemplate = `func Test${structName}(t *testing.T) {
	es := New${structName}()
	assert.EqualValues(t, 0, es.Len())
	es = new${structName}(&[]${originName}{})
	assert.EqualValues(t, 0, es.Len())

	es.Resize(7)
	emptyVal := new${elementName}(&${originName}{})
	testVal := generateTest${elementName}()
	assert.EqualValues(t, 7, es.Len())
	for i := 0; i < es.Len(); i++ {
		assert.EqualValues(t, emptyVal, es.At(i))
		fillTest${elementName}(es.At(i))
		assert.EqualValues(t, testVal, es.At(i))
	}
}

func Test${structName}_CopyTo(t *testing.T) {
	dest := New${structName}()
	// Test CopyTo to empty
	New${structName}().CopyTo(dest)
	assert.EqualValues(t, New${structName}(), dest)

	// Test CopyTo larger slice
	generateTest${structName}().CopyTo(dest)
	assert.EqualValues(t, generateTest${structName}(), dest)

	// Test CopyTo same size slice
	generateTest${structName}().CopyTo(dest)
	assert.EqualValues(t, generateTest${structName}(), dest)
}

func Test${structName}_Resize(t *testing.T) {
	es := generateTest${structName}()
	emptyVal := new${elementName}(&${originName}{})
	// Test Resize less elements.
	const resizeSmallLen = 4
	expectedEs := make(map[*${originName}]bool, resizeSmallLen)
	for i := 0; i < resizeSmallLen; i++ {
		expectedEs[es.At(i).orig] = true
	}
	assert.Equal(t, resizeSmallLen, len(expectedEs))
	es.Resize(resizeSmallLen)
	assert.Equal(t, resizeSmallLen, es.Len())
	foundEs := make(map[*${originName}]bool, resizeSmallLen)
	for i := 0; i < es.Len(); i++ {
		foundEs[es.At(i).orig] = true
	}
	assert.EqualValues(t, expectedEs, foundEs)

	// Test Resize more elements.
	const resizeLargeLen = 7
	oldLen := es.Len()
	expectedEs = make(map[*${originName}]bool, oldLen)
	for i := 0; i < oldLen; i++ {
		expectedEs[es.At(i).orig] = true
	}
	assert.Equal(t, oldLen, len(expectedEs))
	es.Resize(resizeLargeLen)
	assert.Equal(t, resizeLargeLen, es.Len())
	foundEs = make(map[*${originName}]bool, oldLen)
	for i := 0; i < oldLen; i++ {
		foundEs[es.At(i).orig] = true
	}
	assert.EqualValues(t, expectedEs, foundEs)
	for i := oldLen; i < resizeLargeLen; i++ {
		assert.EqualValues(t, emptyVal, es.At(i))
	}

	// Test Resize 0 elements.
	es.Resize(0)
	assert.Equal(t, 0, es.Len())
}

func Test${structName}_Append(t *testing.T) {
	es := generateTest${structName}()

	es.AppendEmpty()
	assert.EqualValues(t, new${elementName}(&${originName}{}), es.At(7))

	value := generateTest${elementName}()
	es.Append(value)
	assert.EqualValues(t, value, es.At(8))

	assert.Equal(t, 9, es.Len())
}`

type baseSlice interface {
	getName() string
}

// Will generate code only for a slice of pointer fields.
type sliceOfPtrs struct {
	structName string
	element    *messageValueStruct
}

func (ss *sliceOfPtrs) getName() string {
	return ss.structName
}

func (ss *sliceOfPtrs) generateStruct(sb *strings.Builder) {
	sb.WriteString(os.Expand(slicePtrTemplate, ss.templateFields()))
	sb.WriteString(os.Expand(commonSliceTemplate, ss.templateFields()))
}

func (ss *sliceOfPtrs) generateTests(sb *strings.Builder) {
	sb.WriteString(os.Expand(slicePtrTestTemplate, ss.templateFields()))
	sb.WriteString(os.Expand(commonSliceTestTemplate, ss.templateFields()))
}

func (ss *sliceOfPtrs) generateTestValueHelpers(sb *strings.Builder) {
	sb.WriteString(os.Expand(commonSliceGenerateTest, ss.templateFields()))
}

func (ss *sliceOfPtrs) templateFields() func(name string) string {
	return func(name string) string {
		switch name {
		case "structName":
			return ss.structName
		case "elementName":
			return ss.element.structName
		case "originName":
			return ss.element.originFullName
		default:
			panic(name)
		}
	}
}

var _ baseStruct = (*sliceOfPtrs)(nil)

// Will generate code only for a slice of value fields.
type sliceOfValues struct {
	structName string
	element    *messageValueStruct
}

func (ss *sliceOfValues) getName() string {
	return ss.structName
}

func (ss *sliceOfValues) generateStruct(sb *strings.Builder) {
	sb.WriteString(os.Expand(sliceValueTemplate, ss.templateFields()))
	sb.WriteString(os.Expand(commonSliceTemplate, ss.templateFields()))
}

func (ss *sliceOfValues) generateTests(sb *strings.Builder) {
	sb.WriteString(os.Expand(sliceValueTestTemplate, ss.templateFields()))
	sb.WriteString(os.Expand(commonSliceTestTemplate, ss.templateFields()))
}

func (ss *sliceOfValues) generateTestValueHelpers(sb *strings.Builder) {
	sb.WriteString(os.Expand(commonSliceGenerateTest, ss.templateFields()))
}

func (ss *sliceOfValues) templateFields() func(name string) string {
	return func(name string) string {
		switch name {
		case "structName":
			return ss.structName
		case "elementName":
			return ss.element.structName
		case "originName":
			return ss.element.originFullName
		default:
			panic(name)
		}
	}
}

var _ baseStruct = (*sliceOfValues)(nil)

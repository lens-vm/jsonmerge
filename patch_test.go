package jsonmerge_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	merge "github.com/lens-vm/jsonmerge"
)

func TestDecodePatchSingleMove(t *testing.T) {
	patchBuf := []byte(`[
	{ "op": "move", "from": "/biscuits", "path": "/cookies" }
]`)
	patch, err := merge.DecodePatch(patchBuf)
	require.NoError(t, err)

	require.Len(t, patch, 1)

	patchOp := patch[0]
	kind := patchOp.Kind()
	require.Equal(t, kind, "move")

	path, err := patchOp.Path()
	require.NoError(t, err)
	require.Equal(t, path, "/cookies")

	from, err := patchOp.From()
	require.NoError(t, err)
	require.Equal(t, from, "/biscuits")
}

func TestDecodePatchSingleAdd(t *testing.T) {
	patchBuf := []byte(`[
		{ "op": "add", "path": "/biscuits/1", "value": { "name": "Ginger Nut" } }
]`)
	patch, err := merge.DecodePatch(patchBuf)
	require.NoError(t, err)

	require.Len(t, patch, 1)

	patchOp := patch[0]
	kind := patchOp.Kind()
	require.Equal(t, kind, "add")

	path, err := patchOp.Path()
	require.NoError(t, err)
	require.Equal(t, path, "/biscuits/1")

	_, err = patchOp.From()
	require.Error(t, err)
}

type applyTestCase struct {
	desc       string
	doc        string
	patch      string
	result     string
	failed     bool
	failedPath string
}

func TestApplyPatch(t *testing.T) {
	testCases := []applyTestCase{
		{
			desc: "Adding an Object Member",
			doc:  `{ "foo": "bar"}`,
			patch: `[
				{ "op": "add", "path": "/baz", "value": "qux" }
			]`,
			result: `{
				"baz": "qux",
				"foo": "bar"
			}`,
		},
		{
			desc: "Adding an Array Element",
			doc:  `{ "foo": [ "bar", "baz" ] }`,
			patch: `[
				{ "op": "add", "path": "/foo/1", "value": "qux" }
			]`,
			result: `{ "foo": [ "bar", "qux", "baz" ] }`,
		},
		{
			desc: "Adding a nested member object",
			doc:  `{ "foo": "bar" }`,
			patch: `[
				{ "op": "add", "path": "/child", "value": { "grandchild": { } } }
			]`,
			result: `{
				"foo": "bar",
				"child": {
				  "grandchild": {
				  }
				}
			}`,
		},
	}

	for _, testcase := range testCases {
		runApplyPatchTest(t, testcase)
	}
}

func runApplyPatchTest(t *testing.T, testcase applyTestCase) {
	patch, err := merge.DecodePatch([]byte(testcase.patch))
	require.NoError(t, err, testcase.desc)

	// sanity check on correct parsing
	patchJSONBytes, err := patch.Marshal()
	require.NoError(t, err, testcase.desc)
	requireEqualJSON(t, []byte(testcase.patch), patchJSONBytes)

	result, err := patch.Apply([]byte(testcase.doc))
	require.NoError(t, err, testcase.desc)

	requireEqualJSON(t, []byte(testcase.result), result)
}

// requireEqualJSON ensures equality between two raw json objects
// provided as byte arrays.
// It marshals them into native Go types (encoding/json)
// and does a reflect.DeepEqual (basically)
func requireEqualJSON(t *testing.T, expected, actual []byte, msgAndArgs ...interface{}) {
	actualJSON, err := toGoJSON(actual)
	require.NoError(t, err, msgAndArgs...)
	expectedJSON, err := toGoJSON([]byte(expected))
	require.NoError(t, err, msgAndArgs...)
	require.Equal(t, expectedJSON, actualJSON)
}

func toGoJSON(buf []byte) (any, error) {
	var result any
	err := json.Unmarshal(buf, &result)
	return result, err
}

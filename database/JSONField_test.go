package database

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// JSON-01
func TestJSONField(t *testing.T) {
	t.Run("Scan nil sets the field as null", func(t *testing.T) {
		var field JSONField

		require.NoError(t, field.Scan(nil))

		assert.Nil(t, field)
		assert.True(t, field.IsNull())
	})

	t.Run("Scan []byte copies the payload", func(t *testing.T) {
		var field JSONField

		require.NoError(t, field.Scan([]byte(`{"a":1}`)))

		assert.True(t, field.Equals(JSONField(`{"a":1}`)))
	})

	t.Run("Scan should not alias the source buffer", func(t *testing.T) {
		var field JSONField
		source := []byte(`{"a":1}`)

		require.NoError(t, field.Scan(source))

		source[0] = 'X'

		assert.True(t, field.Equals(JSONField(`{"a":1}`)),
			"field should keep its own copy of the scanned bytes")
	})

	t.Run("Scan reuses the field without leftovers from previous scans", func(t *testing.T) {
		var field JSONField

		require.NoError(t, field.Scan([]byte(`{"aaaaaaaaaaaaaaa":1}`)))
		require.NoError(t, field.Scan([]byte(`{"b":2}`)))

		assert.True(t, field.Equals(JSONField(`{"b":2}`)))
		assert.Equal(t, len(`{"b":2}`), len(field))
	})

	t.Run("Scan accepts a string source", func(t *testing.T) {
		var field JSONField

		err := field.Scan(`{"a":1}`)

		require.NoError(t, err, "drivers may deliver string values and Scan should accept them")
		assert.True(t, field.Equals(JSONField(`{"a":1}`)))
	})

	t.Run("Scan rejects a numeric source", func(t *testing.T) {
		var field JSONField

		err := field.Scan(float64(3.14))

		assert.Error(t, err)
		assert.Nil(t, field, "field should remain unchanged when the source is invalid")
	})

	t.Run("Value marshals data as string and null as nil", func(t *testing.T) {
		field := JSONField(`{"a":1}`)

		value, err := field.Value()
		require.NoError(t, err)
		assert.Equal(t, `{"a":1}`, value)

		var nullField JSONField
		value, err = nullField.Value()
		require.NoError(t, err)
		assert.Nil(t, value)

		emptyField := JSONField{}
		value, err = emptyField.Value()
		require.NoError(t, err)
		assert.Nil(t, value)

		jsonNullField := JSONField("null")
		value, err = jsonNullField.Value()
		require.NoError(t, err)
		assert.Nil(t, value)
	})

	t.Run("IsNull semantics", func(t *testing.T) {
		assert.True(t, JSONField(nil).IsNull())
		assert.True(t, JSONField{}.IsNull())
		assert.True(t, JSONField("null").IsNull())
		assert.False(t, JSONField(`{}`).IsNull())
		assert.False(t, JSONField(`{"a":1}`).IsNull())
	})

	t.Run("Equals semantics", func(t *testing.T) {
		assert.True(t, JSONField(`{"a":1}`).Equals(JSONField(`{"a":1}`)))
		assert.False(t, JSONField(`{"a":1}`).Equals(JSONField(`{"a":2}`)))
		assert.True(t, JSONField(nil).Equals(JSONField{}))
		assert.False(t, JSONField(`{"a":1}`).Equals(JSONField(nil)))
	})

	t.Run("MarshalJSON handles null and raw payload", func(t *testing.T) {
		data, err := JSONField(nil).MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, "null", string(data))

		data, err = JSONField(`{"a":1}`).MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, `{"a":1}`, string(data))
	})

	t.Run("UnmarshalJSON fills the field from raw json", func(t *testing.T) {
		var field JSONField

		require.NoError(t, field.UnmarshalJSON([]byte(`[1,2]`)))

		assert.True(t, field.Equals(JSONField(`[1,2]`)))
	})

	t.Run("json roundtrip on a struct keeps the field content", func(t *testing.T) {
		type record struct {
			Meta JSONField `json:"meta"`
		}

		original := record{Meta: JSONField(`{"a":1}`)}

		data, err := json.Marshal(original)
		require.NoError(t, err)
		assert.JSONEq(t, `{"meta":{"a":1}}`, string(data))

		var decoded record
		require.NoError(t, json.Unmarshal(data, &decoded))

		assert.True(t, decoded.Meta.Equals(original.Meta))
	})

	t.Run("json roundtrip keeps null semantics", func(t *testing.T) {
		type record struct {
			Meta JSONField `json:"meta"`
		}

		data, err := json.Marshal(record{})
		require.NoError(t, err)
		assert.Equal(t, `{"meta":null}`, string(data))

		var decoded record
		require.NoError(t, json.Unmarshal([]byte(`{"meta":null}`), &decoded))

		assert.True(t, decoded.Meta.IsNull())
	})
}

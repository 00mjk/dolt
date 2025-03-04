// Copyright 2020 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package typeinfo

import (
	"context"
	"fmt"
	"math"

	"github.com/dolthub/go-mysql-server/sql"
	gmstypes "github.com/dolthub/go-mysql-server/sql/types"
	"github.com/dolthub/vitess/go/sqltypes"

	"github.com/dolthub/dolt/go/store/types"
)

type Identifier string

const (
	UnknownTypeIdentifier            Identifier = "unknown"
	BitTypeIdentifier                Identifier = "bit"
	BlobStringTypeIdentifier         Identifier = "blobstring"
	BoolTypeIdentifier               Identifier = "bool"
	DatetimeTypeIdentifier           Identifier = "datetime"
	DecimalTypeIdentifier            Identifier = "decimal"
	EnumTypeIdentifier               Identifier = "enum"
	FloatTypeIdentifier              Identifier = "float"
	JSONTypeIdentifier               Identifier = "json"
	InlineBlobTypeIdentifier         Identifier = "inlineblob"
	IntTypeIdentifier                Identifier = "int"
	SetTypeIdentifier                Identifier = "set"
	TimeTypeIdentifier               Identifier = "time"
	TupleTypeIdentifier              Identifier = "tuple"
	UintTypeIdentifier               Identifier = "uint"
	UuidTypeIdentifier               Identifier = "uuid"
	VarBinaryTypeIdentifier          Identifier = "varbinary"
	VarStringTypeIdentifier          Identifier = "varstring"
	YearTypeIdentifier               Identifier = "year"
	GeometryTypeIdentifier           Identifier = "geometry"
	PointTypeIdentifier              Identifier = "point"
	LineStringTypeIdentifier         Identifier = "linestring"
	PolygonTypeIdentifier            Identifier = "polygon"
	MultiPointTypeIdentifier         Identifier = "multipoint"
	MultiLineStringTypeIdentifier    Identifier = "multilinestring"
	MultiPolygonTypeIdentifier       Identifier = "multipolygon"
	GeometryCollectionTypeIdentifier Identifier = "geometrycollection"
)

var Identifiers = map[Identifier]struct{}{
	UnknownTypeIdentifier:            {},
	BitTypeIdentifier:                {},
	BlobStringTypeIdentifier:         {},
	BoolTypeIdentifier:               {},
	DatetimeTypeIdentifier:           {},
	DecimalTypeIdentifier:            {},
	EnumTypeIdentifier:               {},
	FloatTypeIdentifier:              {},
	JSONTypeIdentifier:               {},
	InlineBlobTypeIdentifier:         {},
	IntTypeIdentifier:                {},
	SetTypeIdentifier:                {},
	TimeTypeIdentifier:               {},
	TupleTypeIdentifier:              {},
	UintTypeIdentifier:               {},
	UuidTypeIdentifier:               {},
	VarBinaryTypeIdentifier:          {},
	VarStringTypeIdentifier:          {},
	YearTypeIdentifier:               {},
	GeometryTypeIdentifier:           {},
	PointTypeIdentifier:              {},
	LineStringTypeIdentifier:         {},
	PolygonTypeIdentifier:            {},
	MultiPointTypeIdentifier:         {},
	MultiLineStringTypeIdentifier:    {},
	MultiPolygonTypeIdentifier:       {},
	GeometryCollectionTypeIdentifier: {},
}

// TypeInfo is an interface used for encoding type information.
type TypeInfo interface {
	// ConvertNomsValueToValue converts a Noms value to a go value. The expected NomsKind of the given
	// parameter is equivalent to the NomsKind returned by this type info. This is intended for retrieval
	// from storage, thus we do no validation as we assume the stored value is already validated against
	// the given type.
	ConvertNomsValueToValue(v types.Value) (interface{}, error)

	// ReadFrom reads a go value from a noms types.CodecReader directly
	ReadFrom(nbf *types.NomsBinFormat, reader types.CodecReader) (interface{}, error)

	// ConvertValueToNomsValue converts a go value or Noms value to a Noms value. The type of the Noms
	// value will be equivalent to the NomsKind returned from NomsKind.
	ConvertValueToNomsValue(ctx context.Context, vrw types.ValueReadWriter, v interface{}) (types.Value, error)

	// Equals returns whether the given TypeInfo is equivalent to this TypeInfo.
	Equals(other TypeInfo) bool

	// FormatValue returns the stringified version of the value.
	FormatValue(v types.Value) (*string, error)

	// GetTypeIdentifier returns an identifier for this type used for serialization.
	GetTypeIdentifier() Identifier

	// GetTypeParams returns a map[string]string containing the type parameters.  This is used for
	// serialization and deserialization of type information.
	GetTypeParams() map[string]string

	// IsValid takes in a types.Value and returns whether it is valid for this type.
	IsValid(v types.Value) bool

	// NomsKind returns the NomsKind that best matches this TypeInfo.
	NomsKind() types.NomsKind

	// Promote will promote the current TypeInfo to the largest representing TypeInfo of the same kind, such as Int8 to Int64.
	Promote() TypeInfo

	// ToSqlType returns the TypeInfo as a sql.Type. If an exact match is able to be made then that is
	// the one returned, otherwise the sql.Type is the closest match possible.
	ToSqlType() sql.Type

	// Stringer results are used to inform users of the constraint's properties.
	fmt.Stringer
}

// FromSqlType takes in a sql.Type and returns the most relevant TypeInfo.
func FromSqlType(sqlType sql.Type) (TypeInfo, error) {
	switch sqlType.Type() {
	case sqltypes.Null:
		return UnknownType, nil
	case sqltypes.Int8:
		return Int8Type, nil
	case sqltypes.Int16:
		return Int16Type, nil
	case sqltypes.Int24:
		return Int24Type, nil
	case sqltypes.Int32:
		return Int32Type, nil
	case sqltypes.Int64:
		return Int64Type, nil
	case sqltypes.Uint8:
		return Uint8Type, nil
	case sqltypes.Uint16:
		return Uint16Type, nil
	case sqltypes.Uint24:
		return Uint24Type, nil
	case sqltypes.Uint32:
		return Uint32Type, nil
	case sqltypes.Uint64:
		return Uint64Type, nil
	case sqltypes.Float32:
		return Float32Type, nil
	case sqltypes.Float64:
		return Float64Type, nil
	case sqltypes.Timestamp:
		return TimestampType, nil
	case sqltypes.Date:
		return DateType, nil
	case sqltypes.Time:
		return TimeType, nil
	case sqltypes.Datetime:
		return DatetimeType, nil
	case sqltypes.Year:
		return YearType, nil
	case sqltypes.Geometry:
		switch sqlType.String() {
		case gmstypes.PointType{}.String():
			return &pointType{sqlType.(gmstypes.PointType)}, nil
		case gmstypes.LineStringType{}.String():
			return &linestringType{sqlType.(gmstypes.LineStringType)}, nil
		case gmstypes.PolygonType{}.String():
			return &polygonType{sqlType.(gmstypes.PolygonType)}, nil
		case gmstypes.MultiPointType{}.String():
			return &multipointType{sqlType.(gmstypes.MultiPointType)}, nil
		case gmstypes.MultiLineStringType{}.String():
			return &multilinestringType{sqlType.(gmstypes.MultiLineStringType)}, nil
		case gmstypes.MultiPolygonType{}.String():
			return &multipolygonType{sqlType.(gmstypes.MultiPolygonType)}, nil
		case gmstypes.GeomCollType{}.String():
			return &geomcollType{sqlType.(gmstypes.GeomCollType)}, nil
		case gmstypes.GeometryType{}.String():
			return &geometryType{sqlGeometryType: sqlType.(gmstypes.GeometryType)}, nil
		default:
			return nil, fmt.Errorf(`expected "PointTypeIdentifier" from SQL basetype "Geometry"`)
		}
	case sqltypes.Decimal:
		decimalSQLType, ok := sqlType.(sql.DecimalType)
		if !ok {
			return nil, fmt.Errorf(`expected "DecimalTypeIdentifier" from SQL basetype "Decimal"`)
		}
		return &decimalType{decimalSQLType}, nil
	case sqltypes.Text:
		stringType, ok := sqlType.(sql.StringType)
		if !ok {
			return nil, fmt.Errorf(`expected "StringType" from SQL basetype "Text"`)
		}
		return &blobStringType{stringType}, nil
	case sqltypes.Blob:
		stringType, ok := sqlType.(sql.StringType)
		if !ok {
			return nil, fmt.Errorf(`expected "StringType" from SQL basetype "Blob"`)
		}
		return &varBinaryType{stringType}, nil
	case sqltypes.VarChar:
		stringType, ok := sqlType.(sql.StringType)
		if !ok {
			return nil, fmt.Errorf(`expected "StringType" from SQL basetype "VarChar"`)
		}
		return &varStringType{stringType}, nil
	case sqltypes.VarBinary:
		stringType, ok := sqlType.(sql.StringType)
		if !ok {
			return nil, fmt.Errorf(`expected "StringType" from SQL basetype "VarBinary"`)
		}
		return &inlineBlobType{stringType}, nil
	case sqltypes.Char:
		stringType, ok := sqlType.(sql.StringType)
		if !ok {
			return nil, fmt.Errorf(`expected "StringType" from SQL basetype "Char"`)
		}
		return &varStringType{stringType}, nil
	case sqltypes.Binary:
		stringType, ok := sqlType.(sql.StringType)
		if !ok {
			return nil, fmt.Errorf(`expected "StringType" from SQL basetype "Binary"`)
		}
		return &inlineBlobType{stringType}, nil
	case sqltypes.Bit:
		bitSQLType, ok := sqlType.(gmstypes.BitType)
		if !ok {
			return nil, fmt.Errorf(`expected "BitTypeIdentifier" from SQL basetype "Bit"`)
		}
		return &bitType{bitSQLType}, nil
	case sqltypes.TypeJSON:
		js, ok := sqlType.(gmstypes.JsonType)
		if !ok {
			return nil, fmt.Errorf(`expected "JsonType" from SQL basetype "TypeJSON"`)
		}
		return &jsonType{js}, nil
	case sqltypes.Enum:
		enumSQLType, ok := sqlType.(sql.EnumType)
		if !ok {
			return nil, fmt.Errorf(`expected "EnumTypeIdentifier" from SQL basetype "Enum"`)
		}
		return &enumType{enumSQLType}, nil
	case sqltypes.Set:
		setSQLType, ok := sqlType.(sql.SetType)
		if !ok {
			return nil, fmt.Errorf(`expected "SetTypeIdentifier" from SQL basetype "Set"`)
		}
		return &setType{setSQLType}, nil
	default:
		return nil, fmt.Errorf(`no type info can be created from SQL base type "%v"`, sqlType.String())
	}
}

// FromTypeParams constructs a TypeInfo from the given identifier and parameters.
func FromTypeParams(id Identifier, params map[string]string) (TypeInfo, error) {
	switch id {
	case BitTypeIdentifier:
		return CreateBitTypeFromParams(params)
	case BlobStringTypeIdentifier:
		return CreateBlobStringTypeFromParams(params)
	case BoolTypeIdentifier:
		return BoolType, nil
	case DatetimeTypeIdentifier:
		return CreateDatetimeTypeFromParams(params)
	case DecimalTypeIdentifier:
		return CreateDecimalTypeFromParams(params)
	case EnumTypeIdentifier:
		return CreateEnumTypeFromParams(params)
	case FloatTypeIdentifier:
		return CreateFloatTypeFromParams(params)
	case InlineBlobTypeIdentifier:
		return CreateInlineBlobTypeFromParams(params)
	case IntTypeIdentifier:
		return CreateIntTypeFromParams(params)
	case JSONTypeIdentifier:
		return JSONType, nil
	case GeometryTypeIdentifier:
		return CreateGeometryTypeFromParams(params)
	case PointTypeIdentifier:
		return CreatePointTypeFromParams(params)
	case LineStringTypeIdentifier:
		return CreateLineStringTypeFromParams(params)
	case PolygonTypeIdentifier:
		return CreatePolygonTypeFromParams(params)
	case MultiPointTypeIdentifier:
		return CreateMultiPointTypeFromParams(params)
	case MultiLineStringTypeIdentifier:
		return CreateMultiLineStringTypeFromParams(params)
	case MultiPolygonTypeIdentifier:
		return CreateMultiPolygonTypeFromParams(params)
	case GeometryCollectionTypeIdentifier:
		return CreateGeomCollTypeFromParams(params)
	case SetTypeIdentifier:
		return CreateSetTypeFromParams(params)
	case TimeTypeIdentifier:
		return TimeType, nil
	case TupleTypeIdentifier:
		return TupleType, nil
	case UintTypeIdentifier:
		return CreateUintTypeFromParams(params)
	case UuidTypeIdentifier:
		return UuidType, nil
	case VarBinaryTypeIdentifier:
		return CreateVarBinaryTypeFromParams(params)
	case VarStringTypeIdentifier:
		return CreateVarStringTypeFromParams(params)
	case YearTypeIdentifier:
		return YearType, nil
	default:
		return nil, fmt.Errorf(`"%v" cannot be made from an identifier and params`, id)
	}
}

// FromKind returns the default TypeInfo for a given types.Value.
func FromKind(kind types.NomsKind) TypeInfo {
	switch kind {
	case types.BlobKind:
		return &varBinaryType{gmstypes.LongBlob}
	case types.BoolKind:
		return BoolType
	case types.FloatKind:
		return Float64Type
	case types.InlineBlobKind:
		return &inlineBlobType{gmstypes.MustCreateBinary(sqltypes.VarBinary, math.MaxUint16)}
	case types.IntKind:
		return Int64Type
	case types.JSONKind:
		return JSONType
	case types.LineStringKind:
		return LineStringType
	case types.NullKind:
		return UnknownType
	case types.GeometryKind:
		return GeometryType
	case types.PointKind:
		return PointType
	case types.PolygonKind:
		return PolygonType
	case types.StringKind:
		return StringDefaultType
	case types.TimestampKind:
		return DatetimeType
	case types.TupleKind:
		return TupleType
	case types.UintKind:
		return Uint64Type
	case types.UUIDKind:
		return UuidType
	case types.DecimalKind:
		return &decimalType{gmstypes.MustCreateDecimalType(65, 30)}
	default:
		panic(fmt.Errorf(`no default type info for NomsKind "%v"`, kind.String()))
	}
}

// IsStringType returns whether the given TypeInfo represents a CHAR, VARCHAR, or TEXT-derivative.
func IsStringType(ti TypeInfo) bool {
	_, ok := ti.(*varStringType)
	return ok
}

// ParseIdentifier takes in an Identifier in string form and returns the matching Identifier.
// Returns UnknownTypeIdentifier when the string match is not found.
func ParseIdentifier(name string) Identifier {
	id := Identifier(name)
	_, ok := Identifiers[id]
	if ok {
		return id
	}
	return UnknownTypeIdentifier
}

// String returns a string representation of the identifier. This may later be used in parsing to
// retrieve the original identifier.
func (i Identifier) String() string {
	return string(i)
}

package mongocursorpagination

import (
	"encoding/base64"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type item struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	CreatedAt time.Time          `bson:"createdAt"`
}

func TestFind(t *testing.T) {
	var cases = []struct {
		name               string
		findParams         FindParams
		results            interface{}
		executeCountQuery  func(db *mongo.Database, collectionName string, queries []bson.M) (int64, error)
		executeCursorQuery func(db *mongo.Database, collectionName string, query []bson.M, sort []string, limit int64, collation *options.Collation, results interface{}) error
		expectedCursor     Cursor
		expectedErr        error
	}{
		{
			name:               "errors when results is nil",
			findParams:         FindParams{},
			results:            nil,
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("results can't be nil"),
		},
		{
			name:               "errors when DB is nil",
			findParams:         FindParams{},
			results:            &[]item{},
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("DB can't be nil"),
		},
		{
			name: "errors when limit is less than 1",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Limit:          0,
			},
			results:            &[]item{},
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("a limit of at least 1 is required"),
		},
		{
			name: "errors when next cursor is bad",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Limit:          2,
				Next:           "XXXXXaGVsbG8=",
			},
			results:            &[]item{},
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("next cursor parse failed: illegal base64 data at input byte 12"),
		},
		{
			name: "errors when previous cursor is bad",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Limit:          2,
				Previous:       "XXXXXaGVsbG8=",
			},
			results:            &[]item{},
			executeCountQuery:  nil,
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("previous cursor parse failed: illegal base64 data at input byte 12"),
		},
		{
			name: "errors when executeCountQuery errors",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}},
				SortAscending:  false,
				PaginatedField: "name",
				Limit:          2,
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db *mongo.Database, collectionName string, queries []bson.M) (int64, error) {
				return 0, errors.New("error")
			},
			executeCursorQuery: nil,
			expectedCursor:     Cursor{},
			expectedErr:        errors.New("error"),
		},
		{
			name: "errors when executeCursorQuery errors",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}},
				SortAscending:  false,
				PaginatedField: "name",
				Limit:          2,
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db *mongo.Database, collectionName string, queries []bson.M) (int64, error) {
				return 2, nil
			},
			executeCursorQuery: func(db *mongo.Database, collectionName string, query []bson.M, sort []string, limit int64, collation *options.Collation, results interface{}) error {
				return errors.New("error")
			},
			expectedCursor: Cursor{},
			expectedErr:    errors.New("error"),
		},
		{
			name: "return cursor with next and count also populates results when next and prev not specified",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}},
				SortAscending:  false,
				PaginatedField: "name",
				Limit:          2,
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db *mongo.Database, collectionName string, queries []bson.M) (int64, error) {
				return 2, nil
			},
			executeCursorQuery: func(db *mongo.Database, collectionName string, query []bson.M, sort []string, limit int64, collation *options.Collation, results interface{}) error {
				resultv := reflect.ValueOf(results)
				resultv.Elem().Set(reflect.ValueOf([]item{
					{ID: makeObjectId("111"), Name: "test item 1", CreatedAt: time.Now()},
					{ID: makeObjectId("222"), Name: "test item 2", CreatedAt: time.Now()},
					{ID: makeObjectId("333"), Name: "test item 3", CreatedAt: time.Now()},
				}))
				return nil
			},
			expectedCursor: Cursor{
				Previous:    "",
				Next:        "PwAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMgACX2lkABsAAAA8cHJpbWl0aXZlLk9iamVjdElEIFZhbHVlPgAA",
				HasPrevious: false,
				HasNext:     true,
				Count:       2,
			},
			expectedErr: nil,
		},
		{
			name: "return cursor with previous and count also populates results when next is specified",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}},
				SortAscending:  true,
				PaginatedField: "name",
				Limit:          2,
				Next:           "PwAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQACX2lkABsAAAA8cHJpbWl0aXZlLk9iamVjdElEIFZhbHVlPgAA",
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db *mongo.Database, collectionName string, queries []bson.M) (int64, error) {
				return 2, nil
			},
			executeCursorQuery: func(db *mongo.Database, collectionName string, query []bson.M, sort []string, limit int64, collation *options.Collation, results interface{}) error {
				resultv := reflect.ValueOf(results)
				resultv.Elem().Set(reflect.ValueOf([]item{
					{ID: makeObjectId("111"), Name: "test item 1", CreatedAt: time.Now()},
					{ID: makeObjectId("222"), Name: "test item 2", CreatedAt: time.Now()},
				}))
				return nil
			},
			expectedCursor: Cursor{
				Previous:    "PwAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQACX2lkABsAAAA8cHJpbWl0aXZlLk9iamVjdElEIFZhbHVlPgAA",
				Next:        "",
				HasPrevious: true,
				HasNext:     false,
				Count:       2,
			},
			expectedErr: nil,
		},
		{
			name: "return cursor with next and count also populates results when previous is specified",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Query:          bson.M{"name": primitive.Regex{Pattern: "test item.*", Options: "i"}},
				SortAscending:  true,
				PaginatedField: "name",
				Limit:          2,
				Previous:       "PwAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQACX2lkABsAAAA8cHJpbWl0aXZlLk9iamVjdElEIFZhbHVlPgAA",
				CountTotal:     true,
			},
			results: &[]item{},
			executeCountQuery: func(db *mongo.Database, collectionName string, queries []bson.M) (int64, error) {
				return 2, nil
			},
			executeCursorQuery: func(db *mongo.Database, collectionName string, query []bson.M, sort []string, limit int64, collation *options.Collation, results interface{}) error {
				resultv := reflect.ValueOf(results)
				resultv.Elem().Set(reflect.ValueOf([]item{
					{ID: makeObjectId("111"), Name: "test item 1", CreatedAt: time.Now()},
					{ID: makeObjectId("222"), Name: "test item 2", CreatedAt: time.Now()},
				}))
				return nil
			},
			expectedCursor: Cursor{
				Previous:    "",
				Next:        "PwAAAAJuYW1lAAwAAAB0ZXN0IGl0ZW0gMQACX2lkABsAAAA8cHJpbWl0aXZlLk9iamVjdElEIFZhbHVlPgAA",
				HasPrevious: false,
				HasNext:     true,
				Count:       2,
			},
			expectedErr: nil,
		},
		{
			name: "return cursor with next also populates results when no pagination field specified",
			findParams: FindParams{
				DB:             &mongo.Database{},
				CollectionName: "items",
				Query:          bson.M{},
				SortAscending:  true,
				Limit:          2,
				CountTotal:     false,
			},
			results:           &[]item{},
			executeCountQuery: nil,
			executeCursorQuery: func(db *mongo.Database, collectionName string, query []bson.M, sort []string, limit int64, collation *options.Collation, results interface{}) error {
				resultv := reflect.ValueOf(results)
				resultv.Elem().Set(reflect.ValueOf([]item{
					{ID: makeObjectId("1addf533e81549de7696cb04"), Name: "test item 1", CreatedAt: time.Now()},
					{ID: makeObjectId("2addf533e81549de7696cb04"), Name: "test item 2", CreatedAt: time.Now()},
					{ID: makeObjectId("3addf533e81549de7696cb04"), Name: "test item 3", CreatedAt: time.Now()},
				}))
				return nil
			},
			expectedCursor: Cursor{
				Previous:    "",
				Next:        "FgAAAAdfaWQAKt31M-gVSd52lssEAA",
				HasPrevious: false,
				HasNext:     true,
				Count:       0,
			},
			expectedErr: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executeCountQueryOri := executeCountQuery
			executeCountQuery = tc.executeCountQuery
			defer func() {
				executeCountQuery = executeCountQueryOri
			}()

			executeCursorQueryOri := executeCursorQuery
			executeCursorQuery = tc.executeCursorQuery
			defer func() {
				executeCursorQuery = executeCursorQueryOri
			}()

			cursor, err := Find(tc.findParams, tc.results)

			if tc.results != nil && tc.expectedErr == nil {
				resultsSlice := tc.results.(*[]item)
				require.Equal(t, tc.findParams.Limit, int64(len(*resultsSlice)))
				if tc.findParams.CountTotal == true {
					require.Equal(t, cursor.Count, int64(len(*resultsSlice)))
				}
			}
			require.Equal(t, tc.expectedCursor, cursor)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}

func makeObjectId(s string) primitive.ObjectID {
	objectID, _ := primitive.ObjectIDFromHex(s)
	return objectID
}

func TestParseCursor(t *testing.T) {
	var cases = []struct {
		name                      string
		cursor                    string
		shouldSecondarySortOnID   bool
		expectedCursorFieldValues []interface{}
		expectedErr               error
	}{
		{
			"return appropriate cursor field values when shouldSecondarySortOnID is true",
			"LwAAAAJuYW1lAAoAAAB0ZXN0IGl0ZW0AAl9pZAANAAAAWt31M-gVSd52lssEAAA",
			true,
			[]interface{}{"test item", "Z\xdd\xf53\xe8\x15I\xdev\x96\xcb\x04"},
			nil,
		},
		{
			"return appropriate cursor field values when shouldSecondarySortOnID is false",
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			false,
			[]interface{}{makeObjectId("5addf533e81549de7696cb04")},
			nil,
		},
		{
			"errors when decode fails",
			"XXXXXaGVsbG8=",
			true,
			nil,
			base64.CorruptInputError(12),
		},
		{
			"errors when expecting cursor with 2 elements and only 1 present",
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			true,
			nil,
			errors.New("expecting a cursor with two elements"),
		},
		{
			"errors when expecting cursor with 1 elements and only 2 present",
			"LwAAAAJuYW1lAAoAAAB0ZXN0IGl0ZW0AAl9pZAANAAAAWt31M-gVSd52lssEAAA",
			false,
			nil,
			errors.New("expecting a cursor with a single element"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursorFieldValues, err := parseCursor(tc.cursor, tc.shouldSecondarySortOnID)
			require.Equal(t, tc.expectedCursorFieldValues, cursorFieldValues)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestDecodeCursor(t *testing.T) {
	var cases = []struct {
		name               string
		cursor             string
		expectedCursorData bson.D
		expectedErr        error
	}{
		{
			"errors when base 64 decode fails",
			"XXXXXaGVsbG8=",
			nil,
			base64.CorruptInputError(12),
		},
		{
			"decodes cursor data when valid input cursor provided",
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			primitive.D{primitive.E{Key: "_id", Value: makeObjectId("5addf533e81549de7696cb04")}},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursorData, err := decodeCursor(tc.cursor)
			require.Equal(t, tc.expectedCursorData, cursorData)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGenerateCursorQuery(t *testing.T) {
	var cases = []struct {
		name                    string
		shouldSecondarySortOnID bool
		paginatedField          string
		comparisonOp            string
		cursorFieldValues       []interface{}
		expectedQuery           bson.M
		expectedErr             error
	}{
		{
			"error when wrong number of cursor field values specified and shouldSecondarySortOnID is true",
			true,
			"name",
			"$gt",
			[]interface{}{"abc"},
			nil,
			errors.New("wrong number of cursor field values specified"),
		},
		{
			"error when wrong number of cursor field values specified and shouldSecondarySortOnID is false",
			false,
			"_id",
			"$lt",
			[]interface{}{},
			nil,
			errors.New("wrong number of cursor field values specified"),
		},
		{
			"return appropriate cursor query when shouldSecondarySortOnID is true",
			true,
			"name",
			"$gt",
			[]interface{}{"test item", "123"},
			bson.M{"$or": []bson.M{
				{"name": bson.M{"$gt": "test item"}},
				{"$and": []bson.M{
					{"name": bson.M{"$eq": "test item"}},
					{"_id": bson.M{"$gt": "123"}}},
				},
			}},
			nil,
		},
		{
			"return appropriate cursor query when shouldSecondarySortOnID is false",
			false,
			"_id",
			"$lt",
			[]interface{}{"123"},
			bson.M{"_id": bson.M{"$lt": "123"}},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := generateCursorQuery(tc.shouldSecondarySortOnID, tc.paginatedField, tc.comparisonOp, tc.cursorFieldValues)
			require.Equal(t, tc.expectedQuery, query)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestGenerateCursor(t *testing.T) {
	var cases = []struct {
		name                    string
		result                  interface{}
		paginatedField          string
		shouldSecondarySortOnID bool
		expectedCursor          string
		expectedErr             error
	}{
		{
			"return the generated cursor for an item search paginated by _id",
			item{ID: makeObjectId("5addf533e81549de7696cb04"), Name: "test item", CreatedAt: time.Now()},
			"_id",
			false,
			"FgAAAAdfaWQAWt31M-gVSd52lssEAA",
			nil,
		},
		{
			"return the generated cursor for an item search paginated by name",
			item{ID: makeObjectId("5addf533e81549de7696cb04"), Name: "test item", CreatedAt: time.Now()},
			"name",
			true,
			"PQAAAAJuYW1lAAoAAAB0ZXN0IGl0ZW0AAl9pZAAbAAAAPHByaW1pdGl2ZS5PYmplY3RJRCBWYWx1ZT4AAA",
			nil,
		},
		//{
		//	"errors when invalid result _id is set",
		//	item{ID: makeObjectId("123"), Name: "test item", CreatedAt: time.Now()},
		//	"_id",
		//	false,
		//	"",
		//	errors.New("failed to encode cursor using [{_id ObjectIdHex(\"313233\")}]: ObjectIDs must be exactly 12 bytes long (got 3)"),
		//},
		{
			"errors when result is nil",
			nil,
			"_id",
			false,
			"",
			errors.New("the specified result must be a non nil value"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursor, err := generateCursor(tc.result, tc.paginatedField, tc.shouldSecondarySortOnID)
			require.Equal(t, tc.expectedCursor, cursor)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestFindStructFieldNameByBsonTag(t *testing.T) {
	var cases = []struct {
		name                    string
		structType              reflect.Type
		tag                     string
		expectedStructFieldName string
	}{
		{
			"return struct field name when matching bson tag specified",
			reflect.TypeOf(item{}),
			"name",
			"Name",
		},
		{
			"return empty struct field name when a non matching bson tag specified",
			reflect.TypeOf(item{}),
			"notastructfield",
			"",
		},
		{
			"return empty struct field name when tag is empty",
			reflect.TypeOf(item{}),
			"",
			"",
		},
		{
			"return empty struct field name when structType is nil",
			nil,
			"name",
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			structFieldName := findStructFieldNameByBsonTag(tc.structType, tc.tag)
			require.Equal(t, tc.expectedStructFieldName, structFieldName)
		})
	}
}

func TestEncodeCursorCursor(t *testing.T) {
	var cases = []struct {
		name           string
		cursorData     bson.D
		expectedCursor string
		expectedErr    error
	}{
		{
			"encode cursor when valid input cursor data provided",
			bson.D(primitive.D{primitive.E{Key: "_id", Value: "5addf533e81549de7696cb04"}}),
			"JwAAAAJfaWQAGQAAADVhZGRmNTMzZTgxNTQ5ZGU3Njk2Y2IwNAAA",
			nil,
		},
		// 	Can't find an easy way to make bson marshaling error so skipping that test case
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursor, err := encodeCursor(tc.cursorData)
			require.Equal(t, tc.expectedCursor, cursor)
			require.Equal(t, tc.expectedErr, err)
		})
	}
}
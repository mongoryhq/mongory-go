package mongory_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/mongoryhq/mongory-go"
)

func decodeJSONValue(raw []byte) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	return v, true
}

func fuzzFloatVariant(base float64, mode uint8) float64 {
	switch mode % 8 {
	case 1:
		return math.NaN()
	case 2:
		return math.Inf(1)
	case 3:
		return math.Inf(-1)
	case 4:
		return math.Copysign(0, -1)
	case 5:
		return float64(9007199254740992) // 2^53
	case 6:
		return 1e308
	default:
		return base
	}
}

func assertMatchAndTraceAgree(t *testing.T, cond any, m *mongory.Matcher, rec any) {
	t.Helper()
	gotMatch := m.Match(rec)
	gotTrace, traceStr := m.Trace(rec)
	if gotMatch != gotTrace {
		t.Fatalf("Match vs Trace mismatch\ncond: %#v\nrec:  %#v\nMatch=%v Trace=%v\n%s",
			cond, rec, gotMatch, gotTrace, traceStr)
	}
}

func FuzzCompile(f *testing.F) {
	f.Add([]byte(`{"age":{"$gte":18},"name":{"$regex":"^Al","$options":"i"}}`), "^Al", "name", 18.0, uint8(0))
	f.Add([]byte(`{"a.b.c":{"$in":[1,2,3]},"x\\.y":null}`), "[a-", "a.b", 0.0, uint8(1))
	f.Add([]byte(`{"$or":[{"tags":"go"},{"scores":{"$elemMatch":{"$gt":5,"$lt":9}}}]}`), "(?x)foo", "$unknown", -1.5, uint8(2))
	f.Add([]byte(`{"$and":[{"flag":{"$exists":true}},{"bio":{"$present":false}}]}`), `.*`, "bio", math.NaN(), uint8(4))
	f.Add([]byte(`{"tags":{"$size":{"$gte":1,"$lte":3}}}`), "測試|🧪", `a\.b.$in`, 9007199254740993.0, uint8(5))
	f.Add([]byte(`{}`), "", "", 0.0, uint8(7))

	f.Fuzz(func(t *testing.T, raw []byte, pattern, key string, num float64, mode uint8) {
		val := fuzzFloatVariant(num, mode)
		candidates := []any{
			M{key: M{"$regex": pattern}},
			M{key: M{"$regex": pattern, "$options": key}},
			M{key: M{"$in": A{val, pattern, nil, true}}},
			M{key: M{"$nin": A{val, pattern}}},
			M{key: M{"$gt": val, "$lte": num}},
			M{key: M{"$not": M{"$eq": val}}},
			M{"$or": A{M{key: val}, M{key: M{"$exists": mode%2 == 0}}}},
		}
		if decoded, ok := decodeJSONValue(raw); ok {
			candidates = append(candidates, decoded)
		}

		if mode%16 == 0 {
			// Exercise depth/node guard on bounded deep conditions without OOM.
			depth := int(mode%100) + 1
			candidates = append(candidates, makeNestedCondition(depth))
		}

		sampleRec := M{key: val, "tags": A{pattern, val}, "next": M{"leaf": 1}}
		for _, cond := range candidates {
			m, err := mongory.Compile(cond)
			if err != nil {
				if !errors.Is(err, mongory.ErrInvalidCondition) {
					t.Fatalf("Compile error %v does not wrap ErrInvalidCondition (cond=%#v)", err, cond)
				}
				continue
			}
			if m.Explain() == "" || m.String() != m.Explain() {
				t.Fatalf("invalid Explain/String for compiled cond %#v", cond)
			}
			assertMatchAndTraceAgree(t, cond, m, nil)
			assertMatchAndTraceAgree(t, cond, m, sampleRec)
		}
	})
}

func FuzzMatch(f *testing.F) {
	f.Add([]byte(`{"a":1,"b":{"$gt":0.5}}`), []byte(`{"a":1,"b":1.0}`), 1.0, uint8(0))
	f.Add([]byte(`{"tags":{"$elemMatch":{"$regex":"^go","$options":"i"}}}`), []byte(`{"tags":["Rust","Golang"]}`), 2.5, uint8(1))
	f.Add([]byte(`{"x":{"$in":[1,2.0,null,"台灣"]}}`), []byte(`{"x":"台灣"}`), 0.0, uint8(2))
	f.Add([]byte(`{"nested.0.k":{"$ne":false}}`), []byte(`{"nested":[{"k":true}]}`), -0.0, uint8(3))
	f.Add([]byte(`{"$or":[{"v":{"$nin":[1,2]}},{"v":{"$present":true}}]}`), []byte(`{}`), 1e20, uint8(4))

	f.Fuzz(func(t *testing.T, condBytes, recBytes []byte, rawFloat float64, mode uint8) {
		rec, ok := decodeJSONValue(recBytes)
		if !ok {
			rec = M{"a": fuzzFloatVariant(rawFloat, mode), "b": string(recBytes), "tags": A{string(condBytes), nil}}
		}

		var conds []any
		if c, ok := decodeJSONValue(condBytes); ok {
			conds = append(conds, c)
		}

		v := fuzzFloatVariant(rawFloat, mode)
		s := string(condBytes)
		conds = append(conds,
			M{"a": v},
			M{"a": M{"$eq": v}},
			M{"a": M{"$gt": v, "$lte": rawFloat}},
			M{"$not": M{"a": nil}},
			M{"b": M{"$in": A{s, v, nil, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}}},
			M{"tags": M{"$elemMatch": M{"$present": mode%2 == 0}}},
			M{"tags": M{"$every": M{"$ne": s}}},
			M{"tags": M{"$size": M{"$gte": int(mode % 4)}}},
		)

		for _, cond := range conds {
			m, err := mongory.Compile(cond)
			if err != nil {
				if !errors.Is(err, mongory.ErrInvalidCondition) {
					t.Fatalf("unexpected error without ErrInvalidCondition: %v", err)
				}
				continue
			}
			first := m.Match(rec)
			second := m.Match(rec)
			if first != second {
				t.Fatalf("non-deterministic Match on cond=%#v rec=%#v", cond, rec)
			}
			assertMatchAndTraceAgree(t, cond, m, rec)
		}
	})
}

type fuzzInline struct {
	Code string `mongory:"code" bson:"ignored_code"`
}

type fuzzRecord struct {
	*fuzzInline `bson:",inline"`
	Name        string         `bson:"name"`
	Age         *int64         `json:"age"`
	Score       float64        `bson:"score"`
	Tags        []string       `bson:"tags"`
	Flag        bool           `bson:"flag"`
	Count       json.Number    `bson:"count"`
	OID         objectID       `bson:"oid"`
	At          dateTime       `bson:"at"`
	Exact       string         `bson:"-" json:"exact_name"`
	Plain       string         // exact field name
	Extra       map[string]any `bson:",inline"`
}

func FuzzStructMatch(f *testing.F) {
	f.Add("Alice", int64(30), 9.5, "go", "台灣", uint8(0x0F), []byte(`{"name":"Alice","age":{"$gte":18},"tags":"go"}`))
	f.Add("Bob🧪", int64(0), 0.0, "", "", uint8(0x00), []byte(`{"code":"C1","tags":null,"age":{"$exists":true}}`))
	f.Add("", int64(-5), math.NaN(), "a", "b", uint8(0x35), []byte(`{"score":{"$gt":0},"flag":{"$present":false},"Plain":{"$ne":""}}`))
	f.Add("Cid", int64(9007199254740993), 9007199254740992.0, "ruby", "c", uint8(0x7F), []byte(`{"count":42,"oid":{"$exists":true}}`))

	f.Fuzz(func(t *testing.T, name string, age int64, score float64, tag1, tag2 string, flags uint8, condBytes []byte) {
		fvScore := fuzzFloatVariant(score, flags>>4)

		var agePtr *int64
		if flags&0x01 != 0 {
			v := age
			agePtr = &v
		}

		var tags []string
		switch (flags >> 1) & 0x03 {
		case 1:
			tags = []string{}
		case 2:
			tags = []string{tag1}
		case 3:
			tags = []string{tag1, tag2}
		}

		var inline *fuzzInline
		if flags&0x08 != 0 {
			inline = &fuzzInline{Code: tag1}
		}

		numStr := json.Number(strconv.FormatInt(age, 10))
		if flags&0x20 != 0 {
			numStr = json.Number(tag2)
		}

		var oid objectID
		copy(oid[:], name)

		extra := M{"dyn": tag2}
		if flags&0x40 != 0 {
			extra["code"] = "fallback-" + tag2
		}

		sRec := fuzzRecord{
			fuzzInline: inline,
			Name:       name,
			Age:        agePtr,
			Score:      fvScore,
			Tags:       tags,
			Flag:       flags&0x80 != 0,
			Count:      numStr,
			OID:        oid,
			At:         dateTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).UnixMilli() + age%10000),
			Exact:      tag1,
			Plain:      tag2,
			Extra:      extra,
		}

		mRec := M{
			"name":       sRec.Name,
			"score":      sRec.Score,
			"flag":       sRec.Flag,
			"count":      sRec.Count,
			"oid":        sRec.OID,
			"at":         sRec.At,
			"exact_name": sRec.Exact,
			"Plain":      sRec.Plain,
		}
		if sRec.Age != nil {
			mRec["age"] = *sRec.Age
		} else {
			mRec["age"] = nil
		}
		if sRec.Tags != nil {
			tagList := make(A, len(sRec.Tags))
			for i, tg := range sRec.Tags {
				tagList[i] = tg
			}
			mRec["tags"] = tagList
		} else {
			mRec["tags"] = nil
		}
		if sRec.fuzzInline != nil {
			mRec["code"] = sRec.fuzzInline.Code
		}
		for k, v := range sRec.Extra {
			if _, exists := mRec[k]; !exists {
				mRec[k] = v
			}
		}

		var conds []any
		if c, ok := decodeJSONValue(condBytes); ok {
			conds = append(conds, c)
		}
		conds = append(conds,
			M{"name": name, "age": M{"$ne": nil}},
			M{"score": M{"$gt": 0}},
			M{"score": fvScore},
			M{"tags": tag1},
			M{"tags": M{"$size": M{"$gte": 0}}},
			M{"tags": M{"$present": true}},
			M{"tags": M{"$every": M{"$ne": ""}}},
			M{"tags": M{"$not": tag2}},
			M{"code": M{"$exists": true}},
			M{"count": M{"$in": A{age, fvScore, tag2, nil, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}}},
			M{"$or": A{M{"flag": true}, M{"Plain": tag2}, M{"oid": oid}}},
		)

		sliceRecs := []fuzzRecord{sRec}
		for _, cond := range conds {
			m, err := mongory.Compile(cond)
			if err != nil {
				if !errors.Is(err, mongory.ErrInvalidCondition) {
					t.Fatalf("unexpected compile error: %v", err)
				}
				continue
			}

			want := m.Match(mRec)
			gotVal := m.Match(sRec)
			gotPtr := m.Match(&sRec)
			if gotVal != want || gotPtr != want {
				t.Fatalf("struct vs map mismatch\ncond=%#v\nmap=%#v (want=%v)\nstructVal=%v structPtr=%v\nstruct=%+v",
					cond, mRec, want, gotVal, gotPtr, sRec)
			}
			assertMatchAndTraceAgree(t, cond, m, mRec)

			gotTracePtr, _ := m.Trace(&sRec)
			if gotTracePtr != want {
				t.Fatalf("Trace(&struct) = %v, want %v (cond=%#v)", gotTracePtr, want, cond)
			}

			filtered := mongory.Filter(sliceRecs, m)
			if (len(filtered) == 1) != want {
				t.Fatalf("Filter len=%d, want match=%v", len(filtered), want)
			}
			if found, ok := mongory.Find(sliceRecs, m); ok != want || (ok && found.Name != sRec.Name) {
				t.Fatalf("Find = (%v, %v), want %v", found, ok, want)
			}
			if mongory.Any(sliceRecs, m) != want || mongory.Count(sliceRecs, m) != map[bool]int{true: 1, false: 0}[want] {
				t.Fatalf("Any/Count mismatch for want=%v", want)
			}
		}
	})
}

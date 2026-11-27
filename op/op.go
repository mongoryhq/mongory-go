// Package op defines string constants for all built-in Mongory query operators.
//
// Using these constants avoids typos in map keys (e.g. mongory.M) while adding
// zero runtime overhead:
//
//	m := mongory.MustCompile(mongory.M{
//		"age": mongory.M{op.Gte: 18, op.Lt: 65},
//		op.Or: mongory.A{
//			mongory.M{"role": mongory.M{op.In: mongory.A{"admin", "editor"}}},
//			mongory.M{"name": mongory.M{op.Regex: "^A", op.Options: "i"}},
//		},
//	})
package op

const (
	// Comparison operators.
	Eq  = "$eq"
	Ne  = "$ne"
	Gt  = "$gt"
	Gte = "$gte"
	Lt  = "$lt"
	Lte = "$lte"

	// Inclusion operators.
	In  = "$in"
	Nin = "$nin"

	// Existence operators.
	Exists  = "$exists"
	Present = "$present"

	// Logical operators.
	And = "$and"
	Or  = "$or"
	Nor = "$nor"
	Not = "$not"

	// Array operators.
	ElemMatch = "$elemMatch"
	Every     = "$every"
	Size      = "$size"

	// Regex operator and its modifier key.
	Regex   = "$regex"
	Options = "$options"
)

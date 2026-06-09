package resetexample

// generate:reset
type ResetableSimpleStruct struct {
	Counter int
	Name    string
}

// generate:reset
type ResetableStruct struct {
	// primitives
	IntField    int
	Int8Field   int8
	Int64Field  int64
	UintField   uint
	FloatField  float64
	StringField string
	BoolField   bool

	// slices and maps
	SliceField []int
	MapField   map[string]string

	// pointer to primitives
	IntPtr  *int
	StrPtr  *string
	BoolPtr *bool

	// nested struct (has Reset method generated)
	Child  *ResetableStruct
	Child2 ResetableSimpleStruct
}

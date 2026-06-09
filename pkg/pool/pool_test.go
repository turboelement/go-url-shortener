package pool

import (
	"sync"
	"testing"
)

// testObject - test object, implements Resetter interface.
type testObject struct {
	Value      int
	Name       string
	resetCount int
}

func (o *testObject) Reset() {
	if o == nil {
		return
	}
	o.Value = 0
	o.Name = ""
	o.resetCount++
}

func TestPoolNew(t *testing.T) {
	p := New[*testObject]()
	if p == nil {
		t.Fatal("New() should return non-nil pool")
	}
}

func TestPoolGet(t *testing.T) {
	p := New[*testObject]()
	obj := p.Get()
	// For pointer type T, empty sync.Pool returns nil via New().
	if obj != nil {
		t.Error("Get() from empty pool with pointer type should return nil")
	}
}

func TestPoolPut(t *testing.T) {
	p := New[*testObject]()

	obj := &testObject{Value: 12345, Name: "test"}
	p.Put(obj)

	// Reset() must have been called on the object.
	if obj.Value != 0 {
		t.Errorf("Value should be 0 after Reset, got %d", obj.Value)
	}
	if obj.Name != "" {
		t.Errorf("Name should be empty after Reset, got %q", obj.Name)
	}
	if obj.resetCount != 1 {
		t.Errorf("Reset should be called exactly once, got %d calls", obj.resetCount)
	}
}

func TestPoolPutThenGet(t *testing.T) {
	p := New[*testObject]()

	obj := &testObject{Value: 12345, Name: "test"}
	p.Put(obj)

	got := p.Get()
	if got == nil {
		// sync.Pool may have discarded the object; that's allowed.
		return
	}

	if got.Value != 0 {
		t.Errorf("Value should be 0 after Reset, got %d", got.Value)
	}
	if got.Name != "" {
		t.Errorf("Name should be empty after Reset, got %q", got.Name)
	}
}

func TestPoolMultiplePut(t *testing.T) {
	p := New[*testObject]()

	obj := &testObject{}
	p.Put(obj)
	p.Put(obj)

	got := p.Get()
	if got == nil {
		return
	}
	// resetCount increments each time Reset called.
	if got.resetCount < 2 {
		t.Errorf("Reset should be called on each Put, got resetCount=%d", got.resetCount)
	}
}

func TestPoolConcurrentPutAndGet(t *testing.T) {
	p := New[*testObject]()

	var wg sync.WaitGroup
	const goroutines = 20
	const iterations = 50

	// Pre-populate pool with some objects.
	for i := 0; i < 10; i++ {
		p.Put(&testObject{})
	}

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				obj := p.Get()
				if obj == nil {
					obj = &testObject{Value: 123, Name: "test"}
				}
				p.Put(obj)
			}
		}()
	}

	wg.Wait()
	// No panics or races — success.
}

func BenchmarkPoolPutGet(b *testing.B) {
	p := New[*testObject]()
	obj := &testObject{}

	b.ResetTimer()
	for range b.N {
		p.Put(obj)
		obj = p.Get()
		if obj == nil {
			obj = &testObject{}
		}
	}
}

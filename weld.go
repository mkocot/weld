package weld

import (
	"errors"
	"reflect"
	"strings"
)

var (
	// ErrUnweldable returned when two objects are unmergable
	ErrUnweldable = errors.New("Unweldable")
)

const (
	defaultZeroAsEmpty bool = false
	lookupTag               = "weld"
)

func mergeTag(a interface{}, b interface{}) {
	mirrora := reflect.TypeOf(a)
	mirrorb := reflect.TypeOf(b)
	if mirrora != mirrorb {
		return
	}
	for i := 0; i < mirrora.NumField(); i++ {
		f := mirrora.Field(i)
		t, ok := f.Tag.Lookup(lookupTag)
		if !ok {
			continue
		}
		print(t)
	}
}

// Weld together given arguments
// Treating 0 as literal value! To change this, mark fields as "zero-as-empty"
// Arguments are read-only
// tag: weld
// options:
// * zero-as-empty - treat empty (default) values as unset, and keep previous.
func Weld(a, b interface{}) (interface{}, error) {
	v, err := merge(reflect.ValueOf(a), reflect.ValueOf(b), defaultZeroAsEmpty)
	if err != nil {
		return nil, err
	}
	return v.Interface(), nil
}

func merge(atype, btype reflect.Value, zeroAsEmpty bool) (out reflect.Value, err error) {
	// NOTE: We are creating new object of given type each time and not
	// using inplace replace. This is by design.

	//if atype.Kind() != reflect.Ptr {
	//	return reflect.Value{}, ErrUnweldable
	//}

	if atype.IsValid() && btype.IsValid() && atype.Type() != btype.Type() {
		return reflect.Value{}, ErrUnweldable
	}
	//if atype.Kind() == reflect.Ptr && atype.Elem().Kind() == reflect.Map {
	//	return reflect.Value{}, ErrUnweldable
	//}
	if atype.Kind() == reflect.Ptr {
		// If we do unwrap then pack again
		out, err = merge(atype.Elem(), btype.Elem(), zeroAsEmpty)
		if err == nil && out.IsValid() && out.Kind() != reflect.Map {
			out = out.Addr()
		}
		return
	} else if atype.Kind() == reflect.Struct {
		// make type - this ensures we use modifable value later
		ctypePtr := reflect.New(atype.Type())
		ctype := ctypePtr.Elem()

		for i := 0; i < atype.NumField(); i++ {
			f := atype.Type().Field(i)
			tag, ok := f.Tag.Lookup(lookupTag)
			if ok {
				// reset zeroAsEmpty to default value
				zeroAsEmpty = defaultZeroAsEmpty
				for _, arg := range strings.Split(tag, ",") {
					switch arg {
					case "zero-as-empty":
						zeroAsEmpty = true
					}
				}
			}
			// great!
			afield := atype.Field(i)
			cfield := ctype.Field(i)

			if !btype.IsValid() {
				cfield.Set(afield)
			} else {
				bfield := btype.Field(i)
				merged, _ := merge(afield, bfield, zeroAsEmpty)
				if merged.IsValid() {
					cfield.Set(merged)
				}
			}
		}
		return ctype, nil
	} else if atype.Kind() == reflect.Slice {
		alen := atype.Len()
		blen := btype.Len()
		carray := reflect.MakeSlice(reflect.SliceOf(atype.Type().Elem()), alen, alen+blen)
		reflect.Copy(carray, atype)
		return reflect.AppendSlice(carray, btype), nil
	} else if atype.Kind() == reflect.Map {
		amap := atype.MapKeys()
		bmap := btype.MapKeys()
		cmap := reflect.MakeMap(reflect.MapOf(atype.Type().Key(), atype.Type().Elem()))
		for _, akey := range amap {
			cmap.SetMapIndex(akey, atype.MapIndex(akey))
		}
		for _, bkey := range bmap {
			bval := btype.MapIndex(bkey)
			i := atype.MapIndex(bkey)
			has := i.IsValid()
			if has {
				ix, err := merge(i, bval, zeroAsEmpty)
				if err != nil {
					return reflect.Value{}, err
				}
				i = ix
			} else {
				i = bval
			}
			cmap.SetMapIndex(bkey, i)
		}
		return cmap, nil
	} else {
		err = nil
		if zeroAsEmpty {
			switch btype.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if btype.Int() == 0 {
					out = atype
					return
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if btype.Uint() == 0 {
					out = atype
					return
				}
			case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
				if btype.IsNil() {
					out = atype
					return
				}
			case reflect.String:
				if btype.Len() == 0 {
					out = atype
					return
				}
				//add more cases for Float, Bool, String, etc (and anything else listed http://golang.org/pkg/reflect/#Kind )
			}
		}
		out = btype
		return
	}
}

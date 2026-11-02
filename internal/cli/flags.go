package cli

import (
	"fmt"
	"strconv"
	"strings"
)

type flagSet struct {
	bools  map[string]*bool
	strs   map[string]*string
	ints   map[string]*int
	floats map[string]*float64
}

func newFlags() *flagSet {
	return &flagSet{
		bools:  map[string]*bool{},
		strs:   map[string]*string{},
		ints:   map[string]*int{},
		floats: map[string]*float64{},
	}
}

func (f *flagSet) Bool(name string, def bool) *bool {
	v := def
	f.bools[name] = &v
	return &v
}

func (f *flagSet) String(name, def string) *string {
	v := def
	f.strs[name] = &v
	return &v
}

func (f *flagSet) Int(name string, def int) *int {
	v := def
	f.ints[name] = &v
	return &v
}

func (f *flagSet) Float(name string, def float64) *float64 {
	v := def
	f.floats[name] = &v
	return &v
}

func (f *flagSet) parse(args []string) ([]string, error) {
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if a == "-h" || a == "--help" {
			return nil, errHelp
		}
		if !strings.HasPrefix(a, "--") {
			pos = append(pos, a)
			continue
		}
		name := strings.TrimPrefix(a, "--")
		raw := ""
		hasVal := false
		if j := strings.IndexByte(name, '='); j >= 0 {
			raw = name[j+1:]
			name = name[:j]
			hasVal = true
		}
		if _, ok := f.bools[name]; ok {
			if !hasVal {
				raw = "true"
			}
			b, err := strconv.ParseBool(raw)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: %w", name, err)
			}
			*f.bools[name] = b
			continue
		}
		if !hasVal {
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("flag --%s needs a value", name)
			}
			raw = args[i]
		}
		switch {
		case f.strs[name] != nil:
			*f.strs[name] = raw
		case f.ints[name] != nil:
			n, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: %w", name, err)
			}
			*f.ints[name] = n
		case f.floats[name] != nil:
			n, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: %w", name, err)
			}
			*f.floats[name] = n
		default:
			return nil, fmt.Errorf("unknown flag --%s", name)
		}
	}
	return pos, nil
}

type helpError struct{}

func (helpError) Error() string { return "help" }

var errHelp = helpError{}

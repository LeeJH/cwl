package expr

import (
	"regexp"
	//"github.com/robertkrimen/otto"
	//"github.com/buchanae/cwl"
)

var rx = regexp.MustCompile(`\$\((.*?)\)`)

type Part struct {
	Raw        string
	Expr       string
	Start, End int
}

func Parse(e string) []*Part {
	if len(e) == 0 {
		return nil
	}

	var parts []*Part

	last := 0
	matches := rx.FindAllStringSubmatchIndex(e, -1)
	for _, match := range matches {
		start := match[0]
		end := match[1]
		gstart := match[2]
		gend := match[3]

		if start > last {
			parts = append(parts, &Part{
				Raw:   e[last:start],
				Start: last,
				End:   start,
			})
		}

		parts = append(parts, &Part{
			Raw:   string(e[start:end]),
			Expr:  string(e[gstart:gend]),
			Start: start,
			End:   end,
		})
		last = end
	}

	if last < len(e)-1 {
		parts = append(parts, &Part{
			Raw:   string(e[last:]),
			Start: last,
			End:   len(e),
		})
	}
	return parts
}

//func Eval(e cwl.Expression,

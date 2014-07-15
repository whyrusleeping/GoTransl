package main

import (
	"os"
	"fmt"
	"bufio"
	"strings"
)

//Type equivalencies from C to Go
var FTypes = map[string]string {
	"int" : "int",
	"void" : "",
	"static int" : "int",
	"static SDL_bool" : "bool",
	"static SDL_BlitFunc" : "BlitFunc",
	"static uint32" : "uint32",
	"while" : "for",
	"SDL_BlitFunc" : "BlitFunc",
	"SDL_BlitInfo" : "BlitInfo",
	"SDL_BlitMap" : "BlitMap",
	"SDL_Surface" : "Surface",
	"SDL_Rect" : "Rect",
	"uint64" : "uint64",
	"size_t" : "uint",
	"char" : "char",
	"const char" : "string",
}

// Strings in C that can be directly translated into another string in Go
var OneToOne = map[string]string {
	"SDL_TRUE" : "true",
	"SDL_FALSE" : "false",
	"u_int64_t" : "uint64",
	"Uint32" : "uint32",
	"NULL" : "nil",
	"->" : ".",
	"~" : "^",
	"    " : "\t",
	"#if" : "//#if",
	"#else" : "//#else",
	"#endif" : "//#endif",
	"const" : "",
	"map" : "Map",
	"if(" : "if (",
}

// For the state machine
const (
	None = iota
	FnType
	FnDecl
	FnOpen
	Comment
	VarDecl
)

type Func struct {
	Type string
	Header string
}

/* Matches the first line of functions following this pattern:
int
my_func() {
*/
func MatchFnType(s string) bool {
	for k,_ := range FTypes {
		if k == s {
			return true
		}
	}
	return false
}

func CorrectTypeName(s string) string {
	v,ok := FTypes[s]
	if ok {
		return v
	}
	return s
}

func MatchLineComment(s string) bool {
	t := strings.TrimLeft(s, "\t")
	return len(t) > 1 && t[:2] == "//"
}

func MatchScopeOpen(s string) bool {
	t := strings.TrimRight(s, " \t")
	return len(t) > 0 && t[len(t) - 1] == '{'
}

//Switch spaces to tabs and remove trailing semicolons
func BasicClean(s string) string {
	if len(s) > 0 && s[len(s) - 1] == ';' {
		s = s[:len(s) - 1]
	}
	for k,v := range OneToOne {
		s = strings.Replace(s, k, v, -1)
	}
	return s
}

func SplitSpaceNoEmpty(s string) []string {
	arr := strings.Split(s, " ")
	out := []string{}
	for _,v := range arr {
		v = strings.Trim(v, " \t")
		if len(v) > 0 {
			out = append(out, v)
		}
	}
	return out
}

// Turns "int *a" into "a *int"
func SwapTypeAndName(s string) string {
	p := SplitSpaceNoEmpty(s)
	if len(p) == 0 {
		return ""
	}

	if len(p) == 2 {
		if p[1][0] == '*' {
			p = []string{p[0], "*", p[1][1:]}
		}

		if p[0][len(p[0])-1] == '*' {
			p = []string{p[0][:len(p[0])-1], "*", p[1]}
		}
	}

	p[0] = CorrectTypeName(p[0])

	switch len(p) {
	case 2:
		return p[1] + " " + p[0]
	case 3:
		return p[2] + " *" + p[0]
	default:
		panic("could not parse: '" + s + "'")
	}
}

// turns "int myfunc(int a, int b) {"
// into
// "func myfunc(a int, b int) int {"
func FixFuncParams(s string) string {
	in := strings.Split(s, "(")
	sec := strings.Split(in[1], ")")
	params := strings.Split(sec[0], ",")
	out := in[0] + "("
	for _,v := range params {
		out += SwapTypeAndName(v) + ","
	}
	if out[len(out)-1] == ',' {
		out = out[:len(out)-1]
	}
	out += ")" + sec[1]
	return out
}

// Matches C style variable declarations
func MatchVarDecl(s string) bool {
	/*
	//TODO: handle assignments later
	if strings.Contains(s, "=") {
		return false
	}
	*/

	t := strings.TrimLeft(s, "\t")
	spl := strings.Split(t, " ")
	for k,_ := range FTypes {
		if k == spl[0] {
			return true
		}
	}
	return false
}

func MatchBadIf(s string) bool {
	t := strings.Trim(s, " \t")
	return len(t) > 2 && t[:2] == "if" && t[len(t)-1] != '{'
}

func FixVarDecl(s string) string {
	assign := strings.Contains(s, "=")
	s = strings.Replace(s, ",", " ", -1)
	s = strings.TrimLeft(s, "\t ")
	spl := strings.Split(s, " ")
	if assign {
		if spl[1][0] == '*' {
			spl[1] = spl[1][1:]
		}
		return spl[1] + " := " + spl[3]
	}
	out := "var "
	for _,v := range spl[1:] {
		if len(v) > 0 {
			out += v + ","
		}
	}
	return out[:len(out)-1] + " " + CorrectTypeName(spl[0])
}

func main() {
	fi,err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}

	out := []string{"package gdl"}
	scan := bufio.NewScanner(fi)
	state := None
	var cur *Func
	for scan.Scan() {
		s := BasicClean(scan.Text())

		switch state {
		case None:
			if MatchFnType(s) {
				state = FnType
				cur = &Func{Type: FTypes[s]}
				continue
			}
		case FnType:
			if MatchScopeOpen(s) {
				state = None
				if len(s) > 1 {
					cur.Header += s[:len(s) - 1]
				}
				post := "func "
				post += cur.Header
				if cur.Type != "void" {
					post += " " + cur.Type
				}
				post += " {"
				out = append(out, FixFuncParams(post))
				continue
			} else {
				cur.Header += s
			}
			continue
		}

		//Remove #includes
		if len(s) > 8 && s[:8] == "#include" {
			continue
		}

		//Fix C style variable declarations
		if MatchVarDecl(s) {
			s = FixVarDecl(s)
		}

		if MatchBadIf(s) {
			fmt.Printf("Potentially bad 'if' on line %d\n", len(out) + 1)
		}

		out = append(out, s)
	}
	fi.Close()
	nfi,err := os.Create(os.Args[1] + ".go")
	if err != nil {
		panic(err)
	}
	for _,v := range out {
		fmt.Fprintln(nfi, v)
	}
	nfi.Close()
}

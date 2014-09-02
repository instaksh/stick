package stick

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenType int

const (
	tokenText tokenType = iota
	tokenName
	tokenNumber
	tokenTagOpen
	tokenTagName
	tokenTagClose
	tokenPrintOpen
	tokenPrintClose
	tokenParensOpen
	tokenParensClose
	tokenArrayOpen
	tokenArrayClose
	tokenHashOpen
	tokenHashClose
	tokenStringOpen
	tokenStringClose
	tokenPunctuation
	tokenOperator
	tokenError
	tokenEof
)

var names = map[tokenType]string{
	tokenText:        "TEXT",
	tokenName:        "NAME",
	tokenNumber:      "NUMBER",
	tokenTagOpen:     "TAG_OPEN",
	tokenTagName:     "TAG_NAME",
	tokenTagClose:    "TAG_CLOSE",
	tokenPrintOpen:   "PRINT_OPEN",
	tokenPrintClose:  "PRINT_CLOSE",
	tokenParensOpen:  "PARENS_OPEN",
	tokenParensClose: "PARENS_CLOSE",
	tokenArrayOpen:   "ARRAY_OPEN",
	tokenArrayClose:  "ARRAY_CLOSE",
	tokenHashOpen:    "HASH_OPEN",
	tokenHashClose:   "HASH_CLOSE",
	tokenStringOpen:  "STRING_OPEN",
	tokenStringClose: "STRING_CLOSE",
	tokenPunctuation: "PUNCTUATION",
	tokenOperator:    "OPERATOR",
	tokenError:       "ERROR",
	tokenEof:         "EOF",
}

const (
	delimEof          = ""
	delimOpenTag      = "{%"
	delimCloseTag     = "%}"
	delimOpenPrint    = "{{"
	delimClosePrint   = "}}"
	delimOpenComment  = "{#"
	delimCloseComment = "#}"
)

type lexerState int

const (
	stateData lexerState = iota
	stateBlock
	stateVar
	stateString
	stateInterpolation
)

type token struct {
	value     string
	pos       int
	tokenType tokenType
}

func (tok token) String() string {
	return fmt.Sprintf("{%s '%s' %d}", names[tok.tokenType], tok.value, tok.pos)
}

type tokenStream []token

type stateFn func(*lexer) stateFn

type lexer struct {
	pos    int // The position of the last emission
	cursor int // The position of the cursor
	parens int // Number of still-open parenthesis in the current expression
	input  string
	tokens tokenStream
	state  stateFn
}

func (lex *lexer) tokenize(code string) tokenStream {
	lex.pos = 0
	lex.cursor = 0
	lex.input = code
	lex.tokens = tokenStream{}

	for lex.state = lexData; lex.state != nil; {
		lex.state = lex.state(lex)
	}

	return lex.tokens
}

func (lex *lexer) next() string {
	if lex.cursor >= len(lex.input) {
		return delimEof
	}
	lex.cursor += 1
	if lex.cursor == len(lex.input) {
		return delimEof
	}
	return lex.current()
}

func (lex *lexer) backup() {
	if lex.cursor <= lex.pos {
		return
	}
	lex.cursor -= 1
}

func (lex *lexer) peek() string {
	return lex.input[lex.cursor+1 : lex.cursor+2]
}

func (lex *lexer) current() string {
	return lex.input[lex.cursor : lex.cursor+1]
}

func (lex *lexer) ignore() {
	lex.pos = lex.cursor
}

func (lex *lexer) emit(t tokenType) {
	val := lex.input[lex.pos:lex.cursor]
	tok := token{val, lex.pos, t}
	lex.tokens = append(lex.tokens, tok)
	lex.pos = lex.cursor
	if lex.pos < len(lex.input) {
		lex.consumeWhitespace()
	}
}

func (lex *lexer) errorf(format string, args ...interface{}) stateFn {
	tok := token{fmt.Sprintf(format, args...), lex.pos, tokenError}
	lex.tokens = append(lex.tokens, tok)

	return nil
}

func (lex *lexer) consumeWhitespace() {
	if lex.pos != lex.cursor {
		panic("Whitespace may only be consumed directly after emission")
	}
	for {
		if isSpace(lex.current()) {
			lex.next()
		} else {
			break
		}
	}

	lex.ignore()
}

func lexData(lex *lexer) stateFn {
	for {
		switch {
		case strings.HasPrefix(lex.input[lex.cursor:], delimOpenTag):
			if lex.cursor > lex.pos {
				lex.emit(tokenText)
			}
			return lexTagOpen

		case strings.HasPrefix(lex.input[lex.cursor:], delimOpenPrint):
			if lex.cursor > lex.pos {
				lex.emit(tokenText)
			}
			return lexPrintOpen
		}

		if lex.next() == delimEof {
			break
		}
	}

	if lex.cursor > lex.pos {
		lex.emit(tokenText)
	}

	lex.emit(tokenEof)

	return nil
}

func lexExpression(lex *lexer) stateFn {
	switch str := lex.current(); {
	case strings.HasPrefix(lex.input[lex.cursor:], delimCloseTag):
		if lex.cursor > lex.pos {
			panic("Incomplete token?")
		}
		return lexTagClose

	case strings.HasPrefix(lex.input[lex.cursor:], delimClosePrint):
		if lex.cursor > lex.pos {
			panic("Incomplete token?")
		}
		return lexPrintClose

	case strings.ContainsAny(str, "+-/*%~"):
		return lexOperator

	case strings.ContainsAny(str, ",?:|"):
		return lexPunctuation

	case strings.ContainsAny(str, "([{"):
		return lexOpenParens

	case strings.ContainsAny(str, "}])"):
		return lexCloseParens

	case str == "\"":
		return lexString

	case isNumeric(str):
		return lexNumber

	case isName(str):
		return lexName

	default:
		panic("Unknown expression")
	}
}

func lexNumber(lex *lexer) stateFn {
	for {
		str := lex.next()
		if !isNumeric(str) {
			break
		}
	}

	lex.emit(tokenNumber)

	return lexExpression
}

func lexOperator(lex *lexer) stateFn {
	lex.next()
	lex.emit(tokenOperator)

	return lexExpression
}

func lexPunctuation(lex *lexer) stateFn {
	lex.next()
	lex.emit(tokenPunctuation)

	return lexExpression
}

func lexString(lex *lexer) stateFn {
	lex.next()
	lex.emit(tokenStringOpen)
	closePos := strings.Index(lex.input[lex.cursor:], "\"")
	if closePos < 0 {
		return lex.errorf("unclosed string")
	}

	lex.cursor += closePos
	lex.emit(tokenText)

	lex.next()
	lex.emit(tokenStringClose)

	return lexExpression
}

func lexOpenParens(lex *lexer) stateFn {
	switch str := lex.current(); {
	case str == "(":
		lex.next()
		lex.emit(tokenParensOpen)

	case str == "[":
		lex.next()
		lex.emit(tokenArrayOpen)

	case str == "{":
		lex.next()
		lex.emit(tokenHashOpen)

	default:
		panic("Unknown parens")
	}

	lex.parens += 1

	return lexExpression
}

func lexCloseParens(lex *lexer) stateFn {
	switch str := lex.current(); {
	case str == ")":
		lex.next()
		lex.emit(tokenParensClose)

	case str == "]":
		lex.next()
		lex.emit(tokenArrayClose)

	case str == "}":
		lex.next()
		lex.emit(tokenHashClose)

	default:
		panic("Unknown parens")
	}

	lex.parens -= 1

	return lexExpression
}

func lexName(lex *lexer) stateFn {
	for {
		str := lex.current()
		if isAlphaNumeric(str) {
			lex.next()
		} else {
			break
		}
	}

	lex.emit(tokenName)

	return lexExpression
}

func lexTagOpen(lex *lexer) stateFn {
	lex.cursor += len(delimOpenTag)
	lex.emit(tokenTagOpen)

	return lexTagName
}

func lexTagName(lex *lexer) stateFn {
	for {
		str := lex.next()
		if !isAlphaNumeric(str) {
			break
		}
	}

	lex.emit(tokenTagName)

	return lexExpression
}

func lexTagClose(lex *lexer) stateFn {
	if lex.parens > 0 {
		return lex.errorf("unclosed parenthesis")
	}
	lex.cursor += len(delimCloseTag)
	lex.emit(tokenTagClose)

	return lexData
}

func lexPrintOpen(lex *lexer) stateFn {
	lex.cursor += len(delimOpenPrint)
	lex.emit(tokenPrintOpen)

	return lexExpression
}

func lexPrintClose(lex *lexer) stateFn {
	if lex.parens > 0 {
		return lex.errorf("unclosed parenthesis")
	}
	lex.cursor += len(delimClosePrint)
	lex.emit(tokenPrintClose)

	return lexData
}

func isSpace(str string) bool {
	return str == " " || str == "\t"
}

func isName(str string) bool {
	for _, s := range str {
		if string(s) != "_" && !unicode.IsLetter(s) && !unicode.IsDigit(s) {
			return false
		}
	}

	return true
}

func isAlphaNumeric(str string) bool {
	for _, s := range str {
		if !unicode.IsLetter(s) && !unicode.IsDigit(s) {
			return false
		}
	}

	return true
}

func isNumeric(str string) bool {
	for _, s := range str {
		if !unicode.IsDigit(s) {
			return false
		}
	}

	return true
}

func lex(input string) tokenStream {
	lex := lexer{}

	return lex.tokenize(input)
}

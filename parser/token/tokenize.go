package token

import (
	//	"fmt"
	"io"
	"strings"
	"unicode"
)

type context int

const (
	DefaultContext = context(iota)
	WhiteSpaceContext
	StringContext
	LineCommentContext
	BlockCommentContext
)

func addToken(cur []Token, val string) []Token {
	switch val {
	case "func", "mutable", "let", "while", "if", "else", "return", "type",
		"enum", "match", "case", "cast", "as", "affects", "assert":
		return append(cur, Keyword(val))
	case "(", ")", "{", "}", `"`, `,`, ":", ".":
		return append(cur, Char(val))
	case "//", "/*", "*/":
		return append(cur, CommentDelimiter(val))
	case "+", "-", "*", "/", "%",
		"<=", "<", "==", ">", ">=", "=", "!=",
		"|",
		"->":
		return append(cur, Operator(val))
	case "int", "bool", "string",
		"uint8", "uint16", "uint32", "uint64",
		"int8", "int16", "int32", "int64":
		return append(cur, Type(val))
	}
	if strings.TrimSpace(val) == "" {
		return append(cur, Whitespace(val))
	}
	return append(cur, Unknown(val))
}

func Tokenize(r io.RuneScanner) ([]Token, error) {
	var currentToken string
	var tokens []Token
	var currentContext context = DefaultContext
	for c, _, err := r.ReadRune(); err == nil; c, _, err = r.ReadRune() {
		// First handle whitespace
		if unicode.IsSpace(c) {
			switch currentContext {
			case LineCommentContext:
				if c == '\n' {
					tokens = append(tokens, LineComment(currentToken))
					currentContext = DefaultContext
					currentToken = ""
					continue
				}
				fallthrough
			case WhiteSpaceContext, StringContext, BlockCommentContext:
				currentToken += string(c)
			default:
				if currentToken != "" {
					tokens = addToken(tokens, currentToken)
				}
				currentToken = string(c)
				currentContext = WhiteSpaceContext
			}
			continue
		}

		switch currentContext {
		case WhiteSpaceContext:
			// It's not a space and we were processing spaces, so add
			// the whitespace token to the list of tokens.
			tokens = append(tokens, Whitespace(currentToken))
			currentContext = DefaultContext
			currentToken = ""
			fallthrough
		case DefaultContext:
			switch c {
			case '<', '=', '>', '|', '!',
				'+', '-', '*', '/', '%':
				peekedToken, _, err := r.ReadRune()
				if err != nil {
					if err != io.EOF {
						panic(err)
					}
				} else {
					if err := r.UnreadRune(); err != nil {
						panic(err)
					}
				}
				if Operator(currentToken+string(c)+string(peekedToken)).IsValid() ||
					CommentDelimiter(currentToken+string(c)+string(peekedToken)).IsValid() {
					currentToken += string(c)
					continue
				} else if Operator(currentToken + string(c)).IsValid() {
					tokens = addToken(tokens, currentToken+string(c))
					currentToken = ""
					currentContext = DefaultContext
					continue
				} else if cd := CommentDelimiter(currentToken + string(c)); cd.IsValid() {
					tokens = addToken(tokens, currentToken+string(c))
					currentToken = ""
					if cd == "//" {
						currentContext = LineCommentContext
					} else if cd == "/*" {
						currentContext = BlockCommentContext
					} else {
						panic("Unhandled comment delimiter type")
					}
					continue
				}
				if currentToken != "" {
					tokens = addToken(tokens, currentToken)
				}
				tokens = append(tokens, Operator(string(c)))
				currentToken = ""
				currentContext = DefaultContext
				continue
			case '(', ')', '{', '}', '"', ',', ':', '[', ']', '.':
				if currentToken != "" {
					tokens = addToken(tokens, currentToken)
				}
				tokens = append(tokens, Char(string(c)))
				currentToken = ""
				if c == '"' {
					currentContext = StringContext
				} else {
					currentContext = DefaultContext
				}
				continue
			}
		case StringContext:
			if c == '"' {
				if l := len(currentToken); l >= 1 && currentToken[l-1] != '\\' {
					tokens = append(tokens, String(currentToken))
					tokens = append(tokens, Char(`"`))
					currentToken = ""
					currentContext = DefaultContext
					continue
				}
			}
		case BlockCommentContext:
			peekedToken, _, err := r.ReadRune()
			if err != nil {
				panic(err)
			}
			if c == '*' && peekedToken == '/' {
				tokens = append(tokens, BlockComment(currentToken), CommentDelimiter("*/"))
				currentContext = DefaultContext
				currentToken = ""
				continue
			}
			if err := r.UnreadRune(); err != nil {
				panic(err)
			}
		}
		currentToken += string(c)
	}
	if currentToken != "" {
		tokens = addToken(tokens, currentToken)
	}
	return tokens, nil
}

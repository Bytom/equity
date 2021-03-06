package compiler

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"unicode"
)

// We have some function naming conventions.
//
// For terminals:
//   scanX     takes buf and position, returns new position (and maybe a value)
//   peekX     takes *parser, returns bool or string
//   consumeX  takes *parser and maybe a required literal, maybe returns value
//             also updates the parser position
//
// For nonterminals:
//   parseX    takes *parser, returns AST node, updates parser position

type parser struct {
	buf []byte
	pos int
}

func (p *parser) errorf(format string, args ...interface{}) {
	panic(parserErr{buf: p.buf, offset: p.pos, format: format, args: args})
}

// parse is the main entry point to the parser
func parse(buf []byte) (contracts []*Contract, err error) {
	defer func() {
		if val := recover(); val != nil {
			if e, ok := val.(parserErr); ok {
				err = e
			} else {
				panic(val)
			}
		}
	}()
	p := &parser{buf: buf}
	contracts = parseContracts(p)
	return
}

// parse contracts
func parseContracts(p *parser) []*Contract {
	var result []*Contract
	contracts := parseImportDirectives(p)
	for _, c := range contracts {
		result = append(result, c)
	}

	if pos := scanKeyword(p.buf, p.pos, "contract"); pos < 0 {
		p.errorf("expected contract")
	}
	for peekKeyword(p) == "contract" {
		contract := parseContract(p)
		result = append(result, contract)
	}
	return result
}

// contract name(p1, p2: t1, p3: t2) locks value { ... }
func parseContract(p *parser) *Contract {
	consumeKeyword(p, "contract")
	name := consumeIdentifier(p)
	params := parseParams(p)
	// locks amount of asset
	consumeKeyword(p, "locks")
	value := ValueInfo{}
	value.Amount = consumeIdentifier(p)
	consumeKeyword(p, "of")
	value.Asset = consumeIdentifier(p)
	consumeTok(p, "{")
	clauses := parseClauses(p)
	consumeTok(p, "}")
	return &Contract{Name: name, Params: params, Clauses: clauses, Value: value}
}

// (p1, p2: t1, p3: t2)
func parseParams(p *parser) []*Param {
	var params []*Param
	consumeTok(p, "(")
	first := true
	for !peekTok(p, ")") {
		if first {
			first = false
		} else {
			consumeTok(p, ",")
		}
		pt := parseParamsType(p)
		params = append(params, pt...)
	}
	consumeTok(p, ")")
	return params
}

func parseClauses(p *parser) []*Clause {
	var clauses []*Clause
	for !peekTok(p, "}") {
		c := parseClause(p)
		clauses = append(clauses, c)
	}
	return clauses
}

func parseParamsType(p *parser) []*Param {
	firstName := consumeIdentifier(p)
	params := []*Param{&Param{Name: firstName}}
	for peekTok(p, ",") {
		consumeTok(p, ",")
		name := consumeIdentifier(p)
		params = append(params, &Param{Name: name})
	}
	consumeTok(p, ":")
	typ := consumeIdentifier(p)
	for _, parm := range params {
		if tdesc, ok := types[typ]; ok {
			parm.Type = tdesc
		} else {
			p.errorf("unknown type %s", typ)
		}
	}
	return params
}

func parseClause(p *parser) *Clause {
	var c Clause
	consumeKeyword(p, "clause")
	c.Name = consumeIdentifier(p)
	c.Params = parseParams(p)
	consumeTok(p, "{")
	c.statements = parseStatements(p)
	consumeTok(p, "}")
	return &c
}

func parseStatements(p *parser) []statement {
	var statements []statement
	for !peekTok(p, "}") {
		s := parseStatement(p)
		statements = append(statements, s)
	}
	return statements
}

func parseStatement(p *parser) statement {
	switch peekKeyword(p) {
	case "if":
		return parseIfStmt(p)
	case "define":
		return parseDefineStmt(p)
	case "assign":
		return parseAssignStmt(p)
	case "verify":
		return parseVerifyStmt(p)
	case "lock":
		return parseLockStmt(p)
	case "unlock":
		return parseUnlockStmt(p)
	}
	panic(parseErr(p.buf, p.pos, "unknown keyword \"%s\"", peekKeyword(p)))
}

func parseIfStmt(p *parser) *ifStatement {
	consumeKeyword(p, "if")
	condition := parseExpr(p)
	body := &IfStatmentBody{}
	consumeTok(p, "{")
	body.trueBody = parseStatements(p)
	consumeTok(p, "}")
	if peekKeyword(p) == "else" {
		consumeKeyword(p, "else")
		consumeTok(p, "{")
		body.falseBody = parseStatements(p)
		consumeTok(p, "}")
	}
	return &ifStatement{condition: condition, body: body}
}

func parseDefineStmt(p *parser) *defineStatement {
	defineStat := &defineStatement{}
	consumeKeyword(p, "define")
	param := &Param{}
	param.Name = consumeIdentifier(p)
	consumeTok(p, ":")
	variableType := consumeIdentifier(p)
	if tdesc, ok := types[variableType]; ok {
		param.Type = tdesc
	} else {
		p.errorf("unknown type %s", variableType)
	}
	defineStat.variable = param
	if peekTok(p, "=") {
		consumeTok(p, "=")
		defineStat.expr = parseExpr(p)
	}
	return defineStat
}

func parseAssignStmt(p *parser) *assignStatement {
	consumeKeyword(p, "assign")
	varName := consumeIdentifier(p)
	consumeTok(p, "=")
	expr := parseExpr(p)
	return &assignStatement{variable: &Param{Name: varName}, expr: expr}
}

func parseVerifyStmt(p *parser) *verifyStatement {
	consumeKeyword(p, "verify")
	expr := parseExpr(p)
	return &verifyStatement{expr: expr}
}

func parseLockStmt(p *parser) *lockStatement {
	consumeKeyword(p, "lock")
	lockedAmount := parseExpr(p)
	consumeKeyword(p, "of")
	lockedAsset := parseExpr(p)
	consumeKeyword(p, "with")
	program := parseExpr(p)
	return &lockStatement{lockedAmount: lockedAmount, lockedAsset: lockedAsset, program: program}
}

func parseUnlockStmt(p *parser) *unlockStatement {
	consumeKeyword(p, "unlock")
	unlockedAmount := parseExpr(p)
	consumeKeyword(p, "of")
	unlockedAsset := parseExpr(p)
	return &unlockStatement{unlockedAmount: unlockedAmount, unlockedAsset: unlockedAsset}
}

func parseExpr(p *parser) expression {
	// Uses the precedence-climbing algorithm
	// <https://en.wikipedia.org/wiki/Operator-precedence_parser#Precedence_climbing_method>
	expr := parseUnaryExpr(p)
	expr2, pos := parseExprCont(p, expr, 0)
	if pos < 0 {
		p.errorf("expected expression")
	}
	p.pos = pos
	return expr2
}

func parseUnaryExpr(p *parser) expression {
	op, pos := scanUnaryOp(p.buf, p.pos)
	if pos < 0 {
		return parseExpr2(p)
	}
	p.pos = pos
	expr := parseUnaryExpr(p)
	return &unaryExpr{op: op, expr: expr}
}

func parseExprCont(p *parser, lhs expression, minPrecedence int) (expression, int) {
	for {
		op, pos := scanBinaryOp(p.buf, p.pos)
		if pos < 0 || op.precedence < minPrecedence {
			break
		}
		p.pos = pos

		rhs := parseUnaryExpr(p)

		for {
			op2, pos2 := scanBinaryOp(p.buf, p.pos)
			if pos2 < 0 || op2.precedence <= op.precedence {
				break
			}
			rhs, p.pos = parseExprCont(p, rhs, op2.precedence)
			if p.pos < 0 {
				return nil, -1 // or is this an error?
			}
		}
		lhs = &binaryExpr{left: lhs, right: rhs, op: op}
	}
	return lhs, p.pos
}

func parseExpr2(p *parser) expression {
	if expr, pos := scanLiteralExpr(p.buf, p.pos); pos >= 0 {
		p.pos = pos
		return expr
	}
	return parseExpr3(p)
}

func parseExpr3(p *parser) expression {
	e := parseExpr4(p)
	if peekTok(p, "(") {
		args := parseArgs(p)
		return &callExpr{fn: e, args: args}
	}
	return e
}

func parseExpr4(p *parser) expression {
	if peekTok(p, "(") {
		consumeTok(p, "(")
		e := parseExpr(p)
		consumeTok(p, ")")
		return e
	}
	if peekTok(p, "[") {
		var elts []expression
		consumeTok(p, "[")
		first := true
		for !peekTok(p, "]") {
			if first {
				first = false
			} else {
				consumeTok(p, ",")
			}
			e := parseExpr(p)
			elts = append(elts, e)
		}
		consumeTok(p, "]")
		return listExpr(elts)
	}
	name := consumeIdentifier(p)
	return varRef(name)
}

func parseArgs(p *parser) []expression {
	var exprs []expression
	consumeTok(p, "(")
	first := true
	for !peekTok(p, ")") {
		if first {
			first = false
		} else {
			consumeTok(p, ",")
		}
		e := parseExpr(p)
		exprs = append(exprs, e)
	}
	consumeTok(p, ")")
	return exprs
}

// peek functions

func peekKeyword(p *parser) string {
	name, _ := scanIdentifier(p.buf, p.pos)
	return name
}

func peekTok(p *parser, token string) bool {
	pos := scanTok(p.buf, p.pos, token)
	return pos >= 0
}

// consume functions

var keywords = []string{
	"contract", "clause", "verify", "locks", "of",
	"lock", "with", "unlock", "if", "else",
	"define", "assign", "true", "false",
}

func consumeKeyword(p *parser, keyword string) {
	pos := scanKeyword(p.buf, p.pos, keyword)
	if pos < 0 {
		p.errorf("expected keyword %s", keyword)
	}
	p.pos = pos
}

func consumeIdentifier(p *parser) string {
	name, pos := scanIdentifier(p.buf, p.pos)
	if pos < 0 {
		p.errorf("expected identifier")
	}
	p.pos = pos
	return name
}

func consumeTok(p *parser, token string) {
	pos := scanTok(p.buf, p.pos, token)
	if pos < 0 {
		p.errorf("expected %s token", token)
	}
	p.pos = pos
}

// scan functions

func scanUnaryOp(buf []byte, offset int) (*unaryOp, int) {
	// Maximum munch. Make sure "-3" scans as ("-3"), not ("-", "3").
	if _, pos := scanIntLiteral(buf, offset); pos >= 0 {
		return nil, -1
	}
	for _, op := range unaryOps {
		newOffset := scanTok(buf, offset, op.op)
		if newOffset >= 0 {
			return &op, newOffset
		}
	}
	return nil, -1
}

func scanBinaryOp(buf []byte, offset int) (*binaryOp, int) {
	offset = skipWsAndComments(buf, offset)
	var (
		found     *binaryOp
		newOffset = -1
	)
	for i, op := range binaryOps {
		offset2 := scanTok(buf, offset, op.op)
		if offset2 >= 0 {
			if found == nil || len(op.op) > len(found.op) {
				found = &binaryOps[i]
				newOffset = offset2
			}
		}
	}
	return found, newOffset
}

// TODO(bobg): boolean literals?
func scanLiteralExpr(buf []byte, offset int) (expression, int) {
	offset = skipWsAndComments(buf, offset)
	intliteral, newOffset := scanIntLiteral(buf, offset)
	if newOffset >= 0 {
		return intliteral, newOffset
	}
	strliteral, newOffset := scanStrLiteral(buf, offset)
	if newOffset >= 0 {
		return strliteral, newOffset
	}
	bytesliteral, newOffset := scanBytesLiteral(buf, offset) // 0x6c249a...
	if newOffset >= 0 {
		return bytesliteral, newOffset
	}
	booleanLiteral, newOffset := scanBoolLiteral(buf, offset) // true or false
	if newOffset >= 0 {
		return booleanLiteral, newOffset
	}
	return nil, -1
}

func scanIdentifier(buf []byte, offset int) (string, int) {
	offset = skipWsAndComments(buf, offset)
	i := offset
	for ; i < len(buf) && isIDChar(buf[i], i == offset); i++ {
	}
	if i == offset {
		return "", -1
	}
	return string(buf[offset:i]), i
}

func scanTok(buf []byte, offset int, s string) int {
	offset = skipWsAndComments(buf, offset)
	prefix := []byte(s)
	if bytes.HasPrefix(buf[offset:], prefix) {
		return offset + len(prefix)
	}
	return -1
}

func scanKeyword(buf []byte, offset int, keyword string) int {
	id, newOffset := scanIdentifier(buf, offset)
	if newOffset < 0 {
		return -1
	}
	if id != keyword {
		return -1
	}
	return newOffset
}

func scanIntLiteral(buf []byte, offset int) (integerLiteral, int) {
	offset = skipWsAndComments(buf, offset)
	start := offset
	if offset < len(buf) && buf[offset] == '-' {
		offset++
	}
	i := offset
	for ; i < len(buf) && unicode.IsDigit(rune(buf[i])); i++ {
		// the literal is BytesLiteral when it starts with 0x/0X
		if buf[i] == '0' && i < len(buf)-1 && (buf[i+1] == 'x' || buf[i+1] == 'X') {
			return 0, -1
		}
	}
	if i > offset {
		n, err := strconv.ParseInt(string(buf[start:i]), 10, 64)
		if err != nil {
			return 0, -1
		}
		return integerLiteral(n), i
	}
	return 0, -1
}

func scanStrLiteral(buf []byte, offset int) (bytesLiteral, int) {
	offset = skipWsAndComments(buf, offset)
	if offset >= len(buf) || !(buf[offset] == '\'' || buf[offset] == '"') {
		return bytesLiteral{}, -1
	}
	var byteBuf bytesLiteral
	for i := offset + 1; i < len(buf); i++ {
		if (buf[offset] == '\'' && buf[i] == '\'') || (buf[offset] == '"' && buf[i] == '"') {
			return byteBuf, i + 1
		}
		if buf[i] == '\\' && i < len(buf)-1 {
			if c, ok := scanEscape(buf[i+1]); ok {
				byteBuf = append(byteBuf, c)
				i++
				continue
			}
		}
		byteBuf = append(byteBuf, buf[i])
	}
	panic(parseErr(buf, offset, "unterminated string literal"))
}

func scanBytesLiteral(buf []byte, offset int) (bytesLiteral, int) {
	offset = skipWsAndComments(buf, offset)
	if offset+4 >= len(buf) {
		return nil, -1
	}
	if buf[offset] != '0' || (buf[offset+1] != 'x' && buf[offset+1] != 'X') {
		return nil, -1
	}
	if !isHexDigit(buf[offset+2]) || !isHexDigit(buf[offset+3]) {
		return nil, -1
	}
	i := offset + 4
	for ; i < len(buf); i += 2 {
		if i == len(buf)-1 {
			panic(parseErr(buf, offset, "odd number of digits in hex literal"))
		}
		if !isHexDigit(buf[i]) {
			break
		}
		if !isHexDigit(buf[i+1]) {
			panic(parseErr(buf, offset, "odd number of digits in hex literal"))
		}
	}
	decoded := make([]byte, hex.DecodedLen(i-(offset+2)))
	_, err := hex.Decode(decoded, buf[offset+2:i])
	if err != nil {
		return bytesLiteral{}, -1
	}
	return bytesLiteral(decoded), i
}

func scanBoolLiteral(buf []byte, offset int) (booleanLiteral, int) {
	offset = skipWsAndComments(buf, offset)
	if offset >= len(buf) {
		return false, -1
	}

	newOffset := scanKeyword(buf, offset, "true")
	if newOffset < 0 {
		if newOffset = scanKeyword(buf, offset, "false"); newOffset < 0 {
			return false, -1
		}
		return false, newOffset
	}
	return true, newOffset
}

func skipWsAndComments(buf []byte, offset int) int {
	var inComment bool
	for ; offset < len(buf); offset++ {
		c := buf[offset]
		if inComment {
			if c == '\n' {
				inComment = false
			}
		} else {
			if c == '/' && offset < len(buf)-1 && buf[offset+1] == '/' {
				inComment = true
				offset++ // skip two chars instead of one
			} else if !unicode.IsSpace(rune(c)) {
				break
			}
		}
	}
	return offset
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isIDChar(c byte, initial bool) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= 'A' && c <= 'Z' {
		return true
	}
	if c == '_' {
		return true
	}
	if initial {
		return false
	}
	return unicode.IsDigit(rune(c))
}

type parserErr struct {
	buf    []byte
	offset int
	format string
	args   []interface{}
}

func parseErr(buf []byte, offset int, format string, args ...interface{}) error {
	return parserErr{buf: buf, offset: offset, format: format, args: args}
}

func (p parserErr) Error() string {
	// Lines start at 1, columns start at 0, like nature intended.
	line := 1
	col := 0
	for i := 0; i < p.offset; i++ {
		if p.buf[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	args := []interface{}{line, col}
	args = append(args, p.args...)
	return fmt.Sprintf("line %d, col %d: "+p.format, args...)
}

func scanEscape(c byte) (byte, bool) {
	escapeFlag := true
	switch c {
	case '\'', '"', '\\':
	case 'b':
		c = '\b'
	case 'f':
		c = '\f'
	case 'n':
		c = '\n'
	case 'r':
		c = '\r'
	case 't':
		c = '\t'
	case 'v':
		c = '\v'
	default:
		escapeFlag = false
	}
	return c, escapeFlag
}

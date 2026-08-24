package i18n

type node interface {
	eval(n int) int
}

type literal struct {
	value int
}

func (l literal) eval(int) int { return l.value }

type variable struct{}

func (variable) eval(n int) int { return n }

type unary struct {
	operator string
	operand  node
}

func (u unary) eval(n int) int {
	value := u.operand.eval(n)
	if u.operator == "!" {
		return boolean(value == 0)
	}
	return value
}

type binary struct {
	operator    string
	left, right node
}

func (b binary) eval(n int) int {
	left := b.left.eval(n)

	switch b.operator {
	case "&&":
		if left == 0 {
			return 0
		}
		return boolean(b.right.eval(n) != 0)
	case "||":
		if left != 0 {
			return 1
		}
		return boolean(b.right.eval(n) != 0)
	}

	right := b.right.eval(n)
	switch b.operator {
	case "%":
		if right == 0 {
			return 0
		}
		return left % right
	case "*":
		return left * right
	case "/":
		if right == 0 {
			return 0
		}
		return left / right
	case "+":
		return left + right
	case "-":
		return left - right
	case "==":
		return boolean(left == right)
	case "!=":
		return boolean(left != right)
	case "<":
		return boolean(left < right)
	case "<=":
		return boolean(left <= right)
	case ">":
		return boolean(left > right)
	case ">=":
		return boolean(left >= right)
	}
	return 0
}

type ternary struct {
	condition, yes, no node
}

func (t ternary) eval(n int) int {
	if t.condition.eval(n) != 0 {
		return t.yes.eval(n)
	}
	return t.no.eval(n)
}

func boolean(value bool) int {
	if value {
		return 1
	}
	return 0
}

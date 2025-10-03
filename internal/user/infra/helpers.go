package infra

func Ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

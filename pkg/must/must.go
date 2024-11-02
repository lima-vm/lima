package must

func Must[T any](obj T, err error) T {
	if err != nil {
		panic(err)
	}
	return obj
}

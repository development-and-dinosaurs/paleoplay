package internal

func GroupByProperty[T any, K comparable](items []T, getProperty func(T) K) map[K][]T {
	grouped := make(map[K][]T)
	for _, item := range items {
		key := getProperty(item)
		grouped[key] = append(grouped[key], item)
	}
	return grouped
}

func Map[T any, O any](items []T, getProperty func(T) O) []O {
	output := make([]O, len(items))
	for i, item := range items {
		o := getProperty(item)
		output[i] = o
	}
	return output
}

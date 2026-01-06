package pagination

const (
	defaultPage    = 1
	defaultPerPage = 10
	maxPerPage     = 100
)

func Normalize(page, perPage int) (int, int) {
	if page <= 0 {
		page = defaultPage
	}
	if perPage <= 0 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return page, perPage
}

func TotalPages(total int64, perPage int) int64 {
	pages := total / int64(perPage)
	if total%int64(perPage) != 0 {
		pages++
	}
	return pages
}

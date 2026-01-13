package pagination

import "github.com/1tsndre/mini-go-project/store-service/internal/constant"

func Normalize(page, perPage int) (int, int) {
	if page <= 0 {
		page = constant.DefaultPage
	}
	if perPage <= 0 {
		perPage = constant.DefaultPerPage
	}
	return min(page, constant.MaxPage), min(perPage, constant.MaxPerPage)
}

func TotalPages(total int64, perPage int) int64 {
	return (total + int64(perPage) - 1) / int64(perPage)
}

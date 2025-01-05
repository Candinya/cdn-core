package handlers

func (a *App) parsePagination(page *uint, limit *uint) (int, int) {
	// 映射前：第几页，每页限制多少个
	// 映射后：页减一，限制不变
	var parsedPage, parsedLimit uint

	if page == nil || *page < 0 {
		parsedPage = 0
	} else {
		parsedPage = *page - 1
	}

	if limit == nil || *limit <= 0 {
		parsedLimit = 100
	} else {
		parsedLimit = *limit
	}

	return int(parsedPage), int(parsedLimit)
}

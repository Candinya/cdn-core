package handlers

func (a *App) parsePagination(page *uint, limit *uint) (bool, int, int) {
	if page != nil && *page == 0 && limit != nil && *limit == 0 {
		// 特殊参数：展示全部
		return true, -1, -1
	}
	// 映射前：第几页，每页限制多少个
	// 映射后：页减一，限制不变
	var parsedPage, parsedLimit uint

	if page == nil || *page < 1 {
		parsedPage = 0
	} else {
		parsedPage = *page - 1
	}

	if limit == nil || *limit <= 0 {
		parsedLimit = 100
	} else {
		parsedLimit = *limit
	}

	return false, int(parsedPage), int(parsedLimit)
}

func (a *App) calcMaxPage(count int64, showAll bool, limit int) int64 {
	if showAll {
		return 1
	} else {
		pageMax := count / int64(limit)
		if (count % int64(limit)) != 0 {
			pageMax++
		}
		return pageMax
	}
}

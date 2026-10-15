package handlers

import (
	"dormitory-management-system/internal/models"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetPagination parses page & limit query parameters and calculates offset and PageInfo
func GetPagination(c *gin.Context, totalItems int, defaultLimit int) (page int, limit int, offset int, pageInfo models.PageInfo) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", strconv.Itoa(defaultLimit))

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err = strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = defaultLimit
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset = (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	var pages []int
	start := page - 2
	if start < 1 {
		start = 1
	}
	end := start + 4
	if end > totalPages {
		end = totalPages
		start = end - 4
		if start < 1 {
			start = 1
		}
	}
	for i := start; i <= end; i++ {
		pages = append(pages, i)
	}

	pageInfo = models.PageInfo{
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalItems:  totalItems,
		Limit:       limit,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		Pages:       pages,
	}

	return page, limit, offset, pageInfo
}

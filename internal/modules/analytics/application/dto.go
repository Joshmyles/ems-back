package application

// SummaryQuery captures the optional analytics filters from the query string.
type SummaryQuery struct {
	DateFrom   string `form:"date_from"`
	DateTo     string `form:"date_to"`
	DistrictID string `form:"district_id"`
}

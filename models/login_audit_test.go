package models

import "testing"

func TestLoginAuditFilterDefaults(t *testing.T) {
	filter := LoginAuditFilter{Page: 0, Limit: 500}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Page != 1 || filter.Limit != 50 {
		t.Fatalf("unexpected defaults: page=%d limit=%d", filter.Page, filter.Limit)
	}
}

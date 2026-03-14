package handlers

import "testing"

func TestReProductPage(t *testing.T) {
	tests := []struct {
		url     string
		wantID  string
		wantHit bool
	}{
		// Standard numeric ID
		{"https://tienda.mercadona.es/product/60722/chocolate-negro", "60722", true},
		// ID with variant suffix (.1)
		{"https://tienda.mercadona.es/product/82830.1/barra-pan-campesina-masa-madre", "82830.1", true},
		// Plural "products"
		{"https://tienda.mercadona.es/products/12345/algo", "12345", true},
		// Plural with variant suffix
		{"https://tienda.mercadona.es/products/99999.2/nombre", "99999.2", true},
		// HTTP (not HTTPS)
		{"http://tienda.mercadona.es/product/11111/algo", "11111", true},
		// Direct image URL — must NOT match
		{"https://prod-mercadona.imgix.net/images/foo.jpg", "", false},
		// Unrelated URL
		{"https://example.com/product/123", "", false},
	}

	for _, tc := range tests {
		m := reProductPage.FindStringSubmatch(tc.url)
		hit := len(m) == 2
		if hit != tc.wantHit {
			t.Errorf("url=%q: wantHit=%v got hit=%v", tc.url, tc.wantHit, hit)
			continue
		}
		if hit && m[1] != tc.wantID {
			t.Errorf("url=%q: wantID=%q got %q", tc.url, tc.wantID, m[1])
		}
	}
}

package utils

import (
	"reflect"
	"testing"
)

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic normalization",
			input:    "São Paulo",
			expected: "sao paulo",
		},
		{
			name:     "Multiple accents",
			input:    "Coração de Jesus",
			expected: "coracao de jesus",
		},
		{
			name:     "Mixed case with accents",
			input:    "BAIRRO Açaí",
			expected: "bairro acai",
		},
		{
			name:     "Special characters",
			input:    "Centro-Sul (Região)",
			expected: "centrosul regiao",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Numbers and spaces",
			input:    "Rua 123 - Apartamento",
			expected: "rua 123 apartamento",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeText(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCreateSearchTerms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple query",
			input:    "casa praia",
			expected: []string{"casa", "praia"},
		},
		{
			name:     "Query with accents",
			input:    "apartamento coração",
			expected: []string{"apartamento", "coracao"},
		},
		{
			name:     "Single word",
			input:    "centro",
			expected: []string{"centro"},
		},
		{
			name:     "Empty query",
			input:    "",
			expected: []string{},
		},
		{
			name:     "Query with extra spaces",
			input:    "  casa   grande  ",
			expected: []string{"casa", "grande"},
		},
		{
			name:     "Query with special characters",
			input:    "2-quartos, piscina!",
			expected: []string{"2quartos", "piscina"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateSearchTerms(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("CreateSearchTerms(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildSearchRegex(t *testing.T) {
	tests := []struct {
		name     string
		terms    []string
		expected string
	}{
		{
			name:     "Single term",
			terms:    []string{"casa"},
			expected: "casa",
		},
		{
			name:     "Multiple terms",
			terms:    []string{"casa", "praia"},
			expected: "casa|praia",
		},
		{
			name:     "Empty terms",
			terms:    []string{},
			expected: "",
		},
		{
			name:     "Terms with special regex characters",
			terms:    []string{"2+quartos", "R$1000"},
			expected: "2\\+quartos|R\\$1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildSearchRegex(tt.terms)
			if result != tt.expected {
				t.Errorf("BuildSearchRegex(%v) = %q, want %q", tt.terms, result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkNormalizeText(b *testing.B) {
	text := "São Paulo - Coração de Jesus (Região Central)"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NormalizeText(text)
	}
}

func BenchmarkCreateSearchTerms(b *testing.B) {
	query := "apartamento 2 quartos piscina garagem"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CreateSearchTerms(query)
	}
}

func BenchmarkBuildSearchRegex(b *testing.B) {
	terms := []string{"casa", "praia", "piscina", "garagem", "quartos"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildSearchRegex(terms)
	}
}

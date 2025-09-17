package main

import (
	"fmt"
	"regexp"
	"strings"
)

func main() {
	pattern := "/imovel/espaco-de-uma-casa-seguranca-de-um-apartamento/{id}"
	path := "/imovel/espaco-de-uma-casa-seguranca-de-um-apartamento/25"

	fmt.Printf("Padrão original: %s\n", pattern)
	fmt.Printf("Path de teste: %s\n", path)

	// Simula o código atual
	if strings.Contains(pattern, "{id}") {
		// Escapa caracteres especiais da regex ANTES de substituir {id}
		escapedPattern := regexp.QuoteMeta(pattern)
		fmt.Printf("Padrão escapado: %s\n", escapedPattern)

		// Substitui o {id} escapado por \d+
		regex := strings.ReplaceAll(escapedPattern, `\{id\}`, `\d+`)
		fmt.Printf("Regex final: %s\n", regex)

		if matched, _ := regexp.MatchString("^"+regex+"$", path); matched {
			fmt.Println("✅ MATCH!")
		} else {
			fmt.Println("❌ NO MATCH")
		}
	}
}



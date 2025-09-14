# 🔍 Funcionalidade de Busca Inteligente

## 🎉 **Nova Funcionalidade Implementada!**

Implementamos um sistema de busca inteligente que permite encontrar propriedades usando palavras-chave e filtros avançados que ignoram maiúsculas/minúsculas e acentos.

## 🚀 **Filtro 'q' - Busca Inteligente**

### **Como Usar**
```bash
# Busca básica por palavras-chave
curl "http://localhost:8080/properties/search?q=casa+piscina"

# Busca com múltiplas palavras
curl "http://localhost:8080/properties/search?q=apartamento+2+quartos+garagem"

# Busca combinada com outros filtros
curl "http://localhost:8080/properties/search?q=casa+moderna&valor_min=200000&cidade=muzambinho"
```

### **Campos Pesquisados (por ordem de relevância)**
1. **Descrição** (peso 3.0) - Campo principal da busca
2. **Endereço** (peso 2.0) - Localização específica
3. **Cidade** (peso 1.5) - Localização geral
4. **Bairro** (peso 1.5) - Localização específica
5. **Tipo de Imóvel** (peso 1.0) - Categoria
6. **Características** (peso 1.0) - Amenidades

### **Funcionalidades Inteligentes**

#### ✅ **Normalização de Texto**
- **Remove acentos**: "São Paulo" = "sao paulo"
- **Ignora maiúsculas**: "CASA" = "casa" = "Casa"
- **Remove caracteres especiais**: "R$ 500.000,00" = "r 500000 00"

#### ✅ **Busca Flexível**
- **Palavras parciais**: "apto" encontra "apartamento"
- **Múltiplas palavras**: "casa piscina" encontra propriedades com ambas
- **Stopwords removidas**: ignora "de", "da", "em", "com", etc.

#### ✅ **Ordenação por Relevância**
- Propriedades com mais matches aparecem primeiro
- Peso maior para matches na descrição
- Combina score de relevância com outros critérios

## 📋 **Filtros Melhorados (Case-Insensitive)**

Todos os filtros de texto agora ignoram maiúsculas/minúsculas e acentos:

### **Exemplos de Uso**

```bash
# Cidade (ignora acentos e case)
curl "http://localhost:8080/properties/search?cidade=sao+paulo"
curl "http://localhost:8080/properties/search?cidade=SÃO+PAULO"
curl "http://localhost:8080/properties/search?cidade=São+Paulo"
# Todos retornam os mesmos resultados!

# Bairro (normalizado)
curl "http://localhost:8080/properties/search?bairro=jardim+europa"
curl "http://localhost:8080/properties/search?bairro=JARDIM+EUROPA"

# Tipo de imóvel (flexível)
curl "http://localhost:8080/properties/search?tipo_imovel=apartamento"
curl "http://localhost:8080/properties/search?tipo_imovel=APTO"
```

## 🎯 **Exemplos Práticos de Busca**

### **1. Busca por Características**
```bash
# Casas com piscina
curl "http://localhost:8080/properties/search?q=casa+piscina"

# Apartamentos com garagem
curl "http://localhost:8080/properties/search?q=apartamento+garagem"

# Propriedades com churrasqueira
curl "http://localhost:8080/properties/search?q=churrasqueira"
```

### **2. Busca por Localização**
```bash
# Propriedades no centro
curl "http://localhost:8080/properties/search?q=centro"

# Casas em bairros específicos
curl "http://localhost:8080/properties/search?q=jardim+europa"

# Propriedades próximas a pontos de referência
curl "http://localhost:8080/properties/search?q=escola+hospital"
```

### **3. Busca por Tipo e Tamanho**
```bash
# Casas grandes
curl "http://localhost:8080/properties/search?q=casa+grande+ampla"

# Apartamentos pequenos
curl "http://localhost:8080/properties/search?q=apartamento+compacto"

# Propriedades com múltiplos quartos
curl "http://localhost:8080/properties/search?q=3+quartos+suite"
```

### **4. Busca Combinada**
```bash
# Busca completa com múltiplos filtros
curl "http://localhost:8080/properties/search?q=casa+moderna+piscina&cidade=muzambinho&valor_min=300000&valor_max=800000&quartos_min=3&page=1&page_size=20"
```

## 📊 **Formato da Resposta**

```json
{
  "properties": [
    {
      "id": "...",
      "endereco": "Rua das Flores, 123",
      "cidade": "Muzambinho",
      "bairro": "Centro",
      "descricao": "Casa moderna com piscina e churrasqueira...",
      "valor": 450000,
      "quartos": 3,
      "banheiros": 2,
      "area_total": 200,
      "tipo_imovel": "Casa",
      "caracteristicas": ["piscina", "churrasqueira", "garagem"]
    }
  ],
  "total_items": 25,
  "total_pages": 3,
  "current_page": 1,
  "page_size": 10
}
```

## 🔧 **Parâmetros Disponíveis**

### **Busca Inteligente**
- `q` - Palavras-chave (máx. 200 caracteres)

### **Filtros Específicos**
- `cidade` - Nome da cidade (case-insensitive)
- `bairro` - Nome do bairro (case-insensitive)  
- `tipo_imovel` - Tipo (casa, apartamento, etc.)

### **Filtros Numéricos**
- `valor_min` / `valor_max` - Faixa de preço
- `quartos_min` / `quartos_max` - Número de quartos
- `banheiros_min` / `banheiros_max` - Número de banheiros
- `area_min` / `area_max` - Área em m²

### **Paginação**
- `page` - Página (padrão: 1)
- `page_size` - Itens por página (padrão: 10, máx: 100)

## 🎨 **Casos de Uso Avançados**

### **1. Busca por Investimento**
```bash
# Propriedades baratas para investir
curl "http://localhost:8080/properties/search?q=oportunidade+investimento&valor_max=200000"

# Imóveis comerciais
curl "http://localhost:8080/properties/search?q=comercial+loja&tipo_imovel=comercial"
```

### **2. Busca por Família**
```bash
# Casas familiares
curl "http://localhost:8080/properties/search?q=familia+criancas+quintal&quartos_min=3"

# Apartamentos seguros
curl "http://localhost:8080/properties/search?q=seguranca+portaria+elevador"
```

### **3. Busca por Luxo**
```bash
# Propriedades de alto padrão
curl "http://localhost:8080/properties/search?q=luxo+fino+acabamento&valor_min=1000000"

# Casas com lazer completo
curl "http://localhost:8080/properties/search?q=piscina+churrasqueira+sauna+academia"
```

## 🚀 **Performance e Otimizações**

### **Características Técnicas**
- ✅ **Busca em múltiplos campos** simultaneamente
- ✅ **Ordenação por relevância** automática
- ✅ **Normalização de texto** para melhor matching
- ✅ **Paginação eficiente** para grandes resultados
- ✅ **Cache-friendly** com filtros consistentes

### **Limitações Atuais**
- Máximo 200 caracteres na query
- Busca por OR (qualquer termo), não AND obrigatório
- Sem busca por sinônimos (ainda)

## 🔍 **Exemplos de Teste**

### **Teste Básico**
```bash
# 1. Busca simples
curl "http://localhost:8080/properties/search?q=casa"

# 2. Busca com acentos
curl "http://localhost:8080/properties/search?q=São+Paulo"

# 3. Busca case-insensitive
curl "http://localhost:8080/properties/search?cidade=MUZAMBINHO"
```

### **Teste de Relevância**
```bash
# Propriedades com piscina (ordenadas por relevância)
curl "http://localhost:8080/properties/search?q=piscina" | jq '.properties[] | {endereco, descricao}'
```

### **Teste de Combinação**
```bash
# Busca complexa
curl "http://localhost:8080/properties/search?q=casa+moderna&cidade=muzambinho&valor_min=200000&quartos_min=2&page_size=5" | jq '.'
```

## 📈 **Melhorias Implementadas**

### **Antes**
```bash
# Busca exata, case-sensitive
curl "http://localhost:8080/properties/search?cidade=Muzambinho"  # ✅ Funciona
curl "http://localhost:8080/properties/search?cidade=muzambinho"  # ❌ Não encontra
curl "http://localhost:8080/properties/search?cidade=MUZAMBINHO"  # ❌ Não encontra
```

### **Depois**
```bash
# Busca flexível, case-insensitive
curl "http://localhost:8080/properties/search?cidade=Muzambinho"  # ✅ Funciona
curl "http://localhost:8080/properties/search?cidade=muzambinho"  # ✅ Funciona
curl "http://localhost:8080/properties/search?cidade=MUZAMBINHO"  # ✅ Funciona
curl "http://localhost:8080/properties/search?q=casa+piscina"     # ✅ Busca inteligente
```

## 🎯 **Próximas Melhorias Planejadas**

1. **Busca por sinônimos** (casa = residência)
2. **Busca geográfica** (proximidade por coordenadas)
3. **Filtros por faixa de preço** predefinidos
4. **Sugestões de busca** (autocomplete)
5. **Histórico de buscas** populares

A nova funcionalidade está **ativa e funcionando**! Teste agora mesmo! 🚀

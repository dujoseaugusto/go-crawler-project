# 📚 Documentação da API - Go Crawler

## 🚀 Acesso Rápido

- **📖 Documentação Interativa (Swagger UI):** [http://localhost:8081/docs](http://localhost:8081/docs)
- **🏠 Interface Web Principal:** [http://localhost:8081/](http://localhost:8081/)
- **💚 Health Check:** [http://localhost:8081/health](http://localhost:8081/health)

## 📋 Visão Geral

A **Go Crawler API** é um sistema inteligente de crawling de imóveis que utiliza aprendizado de máquina para classificar páginas automaticamente, distinguindo entre:

- **📄 Páginas de Catálogo**: Listagens com múltiplos imóveis
- **🏠 Páginas de Propriedade**: Detalhes de imóveis individuais

### 🎯 Precisão Atual: **80%**

Após treinamento com **42 URLs de propriedades reais**, o sistema alcançou 80% de precisão na classificação automática.

## 🔧 Principais Funcionalidades

### 1. 🤖 **Sistema de Classificação Inteligente**
- **Método Recomendado**: Análise baseada em conteúdo (`/content/*`)
- **Método Legado**: Análise baseada em URL (`/patterns/*`)
- **Aprendizado Contínuo**: Melhora com mais exemplos de treinamento

### 2. 🏙️ **Gerenciamento de Cidades**
- Descoberta automática de sites imobiliários
- Validação e estatísticas de sites
- Gerenciamento por região

### 3. 🔍 **Busca Avançada**
- Filtros múltiplos (cidade, tipo, valor, área, quartos)
- Busca textual em descrições
- Paginação eficiente

### 4. 📊 **Monitoramento e Estatísticas**
- Dashboard web completo
- Métricas de performance
- Logs detalhados de crawling

## 🛠️ Endpoints Principais

### 🏠 **Propriedades**
```
GET    /properties              # Listar propriedades (paginado)
GET    /properties/search       # Busca avançada com filtros
```

### 🤖 **Crawler Inteligente**
```
POST   /crawler/trigger         # Iniciar crawling com classificação automática
POST   /crawler/cleanup         # Limpar banco de dados
```

### 🧠 **Aprendizado de Conteúdo (RECOMENDADO)**
```
POST   /content/learn/catalog   # Treinar com páginas de catálogo
POST   /content/learn/property  # Treinar com páginas de propriedades
POST   /content/classify        # Classificar página por conteúdo
GET    /content/patterns        # Ver padrões aprendidos
```

### 🏙️ **Gerenciamento de Cidades**
```
POST   /cities/discover-sites   # Descobrir sites de uma cidade
GET    /cities                  # Listar todas as cidades
GET    /cities/{city}           # Informações de uma cidade
GET    /cities/{city}/sites     # Sites de uma cidade
```

## 📖 Como Usar a Documentação

### 1. **Acesse a Documentação Interativa**
```
http://localhost:8081/docs
```

### 2. **Explore os Endpoints**
- Cada endpoint tem exemplos de request/response
- Você pode testar diretamente na interface
- Schemas detalhados para todos os objetos

### 3. **Teste os Endpoints**
- Use o botão "Try it out" em cada endpoint
- Modifique os parâmetros conforme necessário
- Veja as respostas em tempo real

## 🎯 Fluxo Recomendado de Uso

### 1. **Descobrir Sites de uma Cidade**
```bash
curl -X POST "http://localhost:8081/cities/discover-sites" \
  -H "Content-Type: application/json" \
  -d '{"city": "Muzambinho", "limit": 10}'
```

### 2. **Treinar o Sistema (Opcional)**
```bash
# Treinar com páginas de propriedades
curl -X POST "http://localhost:8081/content/learn/property" \
  -H "Content-Type: application/json" \
  -d '{"urls": ["https://example.com/imovel/123"]}'
```

### 3. **Iniciar Crawling Inteligente**
```bash
curl -X POST "http://localhost:8081/crawler/trigger" \
  -H "Content-Type: application/json" \
  -d '{"cities": ["Muzambinho"], "max_pages": 15}'
```

### 4. **Buscar Propriedades Coletadas**
```bash
curl "http://localhost:8081/properties?cidade=Muzambinho&page=1&page_size=10"
```

## 🔍 Exemplos de Classificação

### ✅ **Página de Propriedade Individual**
```json
{
  "url": "https://www.imobr.com.br/detalhes/casa/venda/muzambinho/mg/bairro-centro/suites-0/banheiros-3/vagas-3/3462038/1",
  "type": "property",
  "confidence": 0.90
}
```

### 🚨 **Página de Catálogo**
```json
{
  "url": "https://www.mgfimoveis.com.br/venda/casa/mg-muzambinho",
  "type": "catalog",
  "confidence": 0.95
}
```

## 📊 Métricas de Performance

| Métrica | Valor | Descrição |
|---------|-------|-----------|
| **Precisão** | 80% | Classificação correta de páginas |
| **Rate Limit** | 100 req/h | Limite geral de requisições |
| **Crawler Limit** | 50 req/h | Limite para endpoints de crawling |
| **Padrões Aprendidos** | 9+ | Padrões de propriedades no sistema |

## 🚨 Notas Importantes

### ⚠️ **Rate Limiting**
- **Endpoints Gerais**: 100 requisições por hora
- **Endpoints de Crawler**: 50 requisições por hora
- Headers de rate limit incluídos nas respostas

### 🧠 **Sistema de Aprendizado**
- **Método Recomendado**: Use `/content/*` para melhor precisão
- **Treinamento**: Mais exemplos = maior precisão
- **Persistência**: Padrões são salvos automaticamente

### 🔄 **Crawling Incremental**
- URLs já visitadas são ignoradas por padrão
- Use `"force": true` para forçar recrawling
- Sistema detecta e evita páginas de catálogo automaticamente

## 🆘 Suporte e Troubleshooting

### 🔍 **Verificar Status**
```bash
curl http://localhost:8081/health
```

### 📋 **Logs Detalhados**
- Logs disponíveis no console da aplicação
- Classificações são logadas com nível DEBUG
- Erros incluem contexto detalhado

### 🐛 **Problemas Comuns**
1. **Baixa Precisão**: Adicione mais exemplos de treinamento
2. **Rate Limit**: Aguarde o reset ou use menos requisições
3. **Sites Inacessíveis**: Verifique conectividade e robots.txt

## 📝 Changelog

### v1.2.0 (Atual)
- ✅ Sistema de classificação baseado em conteúdo
- ✅ Documentação Swagger completa
- ✅ Precisão de 80% após treinamento
- ✅ Interface web melhorada
- ✅ Persistência de padrões aprendidos

### v1.1.0
- ✅ Gerenciamento de cidades e sites
- ✅ Sistema de aprendizado de padrões (URL)
- ✅ Rate limiting implementado

### v1.0.0
- ✅ Crawling básico de propriedades
- ✅ API REST fundamental
- ✅ Interface web inicial

---

**🏠 Go Crawler API** - Sistema Inteligente de Crawling de Imóveis  
📖 **Documentação sempre atualizada em:** [http://localhost:8081/docs](http://localhost:8081/docs)
